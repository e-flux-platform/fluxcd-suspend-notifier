package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/exp/maps"
	"google.golang.org/genproto/googleapis/cloud/audit"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/auditlog"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/datastore"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/fluxcd"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/k8s"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/notification"
)

type Watcher struct {
	googleCloudProjectID string
	gkeClusterName       string
	k8sClient            k8sClient
	store                store
	notifier             notifier
}

func NewWatcher(
	googleCloudProjectID string,
	gkeClusterName string,
	k8sClient k8sClient,
	store store,
	notifier notifier,
) *Watcher {
	return &Watcher{
		googleCloudProjectID: googleCloudProjectID,
		gkeClusterName:       gkeClusterName,
		k8sClient:            k8sClient,
		store:                store,
		notifier:             notifier,
	}
}

type k8sClient interface {
	GetRawResource(ctx context.Context, resource k8s.ResourceReference) ([]byte, error)
	GetRawResources(ctx context.Context, group k8s.ResourceType) ([]byte, error)
	GetCustomResourceDefinitions(ctx context.Context, listOptions metav1.ListOptions) (*v1.CustomResourceDefinitionList, error)
}

type store interface {
	GetEntry(k8s.ResourceReference) (datastore.Entry, error)
	SaveEntry(datastore.Entry) error
}

type notifier interface {
	Notify(context.Context, notification.Notification) error
}

func (w *Watcher) Watch(ctx context.Context) error {
	crds, err := w.k8sClient.GetCustomResourceDefinitions(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/part-of=flux",
	})
	if err != nil {
		return err
	}

	uniqueResourceGroups := make(map[string]k8s.ResourceType)
	for _, crd := range crds.Items {
		for _, version := range crd.Spec.Versions {
			if _, exists := version.Schema.OpenAPIV3Schema.Properties["spec"].Properties["suspend"]; !exists {
				continue
			}
			key := fmt.Sprintf("%s:%s", crd.Spec.Group, crd.Spec.Names.Plural)
			uniqueResourceGroups[key] = k8s.ResourceType{
				Group:   crd.Spec.Group,
				Version: version.Name,
				Kind:    crd.Spec.Names.Plural,
			}
		}
	}
	resourceGroups := maps.Values(uniqueResourceGroups)

	if err = w.init(ctx, resourceGroups); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
	return w.watch(ctx, resourceGroups)
}

func (w *Watcher) init(ctx context.Context, groups []k8s.ResourceType) error {
	slog.Info("initializing")
	for _, group := range groups {
		res, err := w.k8sClient.GetRawResources(ctx, group)
		if err != nil {
			return err
		}
		var resourceList fluxcd.ResourceList
		if err = json.Unmarshal(res, &resourceList); err != nil {
			return fmt.Errorf("failed to unmarshal resource: %w", err)
		}
		for _, resource := range resourceList.Items {
			resourceRef := k8s.ResourceReference{
				Type:      group,
				Namespace: resource.Metadata.Namespace,
				Name:      resource.Metadata.Name,
			}
			if err = w.processResource(ctx, resourceRef, resource, "<unknown>"); err != nil {
				return fmt.Errorf("failed to process resource: %w", err)
			}
		}
	}
	return nil
}

func (w *Watcher) watch(ctx context.Context, groups []k8s.ResourceType) error {
	slog.Info("watching for resource modifications")
	resourceKinds := make([]string, 0, len(groups))
	for _, group := range groups {
		resourceKinds = append(resourceKinds, group.Kind)
	}

	return auditlog.Tail(ctx, w.googleCloudProjectID, w.gkeClusterName, resourceKinds, func(logEntry *audit.AuditLog) error {
		if code := logEntry.GetStatus().GetCode(); code != 0 {
			slog.Warn("operation appeared to fail", slog.Int("code", int(code)))
			return nil
		}

		resourceName := logEntry.GetResourceName()
		email := logEntry.GetAuthenticationInfo().GetPrincipalEmail()

		resourceRef, err := k8s.ResourceReferenceFromPath(resourceName)
		if err != nil {
			return err
		}

		res, err := w.k8sClient.GetRawResource(ctx, resourceRef)
		if err != nil {
			return fmt.Errorf("failed to get raw resource: %w", err)
		}

		var resource fluxcd.Resource
		if err = json.Unmarshal(res, &resource); err != nil {
			return fmt.Errorf("failed to unmarshal resource: %w", err)
		}

		if err = w.processResource(ctx, resourceRef, resource, email); err != nil {
			return fmt.Errorf("failed to re-check suspension status: %w", err)
		}

		return nil
	})
}

func (w *Watcher) processResource(
	ctx context.Context,
	resourceRef k8s.ResourceReference,
	resource fluxcd.Resource,
	updatedBy string,
) error {
	entry, err := w.store.GetEntry(resourceRef)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			// First time seeing the resource, so we'll save the state, but not notify - as we don't know what has
			// changed
			slog.Info(
				"new resource discovered",
				slog.String("kind", resourceRef.Type.Kind),
				slog.String("resource", resourceRef.Name),
				slog.Bool("suspended", resource.Spec.Suspend),
			)
			if err = w.store.SaveEntry(datastore.Entry{
				Resource:  resourceRef,
				Suspended: resource.Spec.Suspend,
				UpdatedBy: updatedBy,
				UpdatedAt: time.Now().UTC(),
			}); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("failed to fetch entry: %w", err)
	}

	if resource.Spec.Suspend == entry.Suspended {
		return nil // Probably something else about the resource modified
	}

	slog.Info(
		"suspension status updated",
		slog.String("kind", resourceRef.Type.Kind),
		slog.String("resourceRef", resourceRef.Name),
		slog.String("user", updatedBy),
		slog.Bool("suspended", resource.Spec.Suspend),
	)

	entry.Resource = resourceRef
	entry.Suspended = resource.Spec.Suspend
	entry.UpdatedBy = updatedBy
	entry.UpdatedAt = time.Now().UTC()

	if err = w.store.SaveEntry(entry); err != nil {
		return err
	}

	return w.notifier.Notify(ctx, notification.Notification{
		Resource:             entry.Resource,
		Suspended:            entry.Suspended,
		Email:                entry.UpdatedBy,
		GoogleCloudProjectID: w.googleCloudProjectID,
	})
}
