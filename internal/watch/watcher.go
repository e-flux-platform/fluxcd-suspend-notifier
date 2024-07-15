package watch

import (
	"context"
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
	GetRawResource(ctx context.Context, resource k8s.Resource) (map[string]any, error)
	GetRawResources(ctx context.Context, group k8s.ResourceType) (map[string]any, error)
	GetCustomResourceDefinitions(ctx context.Context, listOptions metav1.ListOptions) (*v1.CustomResourceDefinitionList, error)
}

type store interface {
	GetEntry(k8s.Resource) (datastore.Entry, error)
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

	if err = w.bootstrap(ctx, resourceGroups); err != nil {
		return fmt.Errorf("failed to bootstrap: %w", err)
	}
	return w.watch(ctx, resourceGroups)
}

func (w *Watcher) bootstrap(ctx context.Context, groups []k8s.ResourceType) error {
	slog.Info("bootstrapping")
	for _, group := range groups {
		resources, err := w.k8sClient.GetRawResources(ctx, group)
		if err != nil {
			return err
		}
		items, ok := resources["items"].([]any)
		if !ok {
			return errors.New("expected items to be set")
		}
		for _, i := range items {
			item, ok := i.(map[string]any)
			if !ok {
				return errors.New("invalid item")
			}
			spec, ok := item["spec"].(map[string]any)
			if !ok {
				return errors.New("invalid spec")
			}
			metadata, ok := item["metadata"].(map[string]any)
			if !ok {
				return errors.New("invalid metadata")
			}
			resource := k8s.Resource{
				Type:      group,
				Namespace: metadata["namespace"].(string),
				Name:      metadata["name"].(string),
			}
			if err = w.processResource(ctx, resource, spec, "<unknown>"); err != nil {
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

		resource, err := k8s.ResourceFromPath(resourceName)
		if err != nil {
			return err
		}

		res, err := w.k8sClient.GetRawResource(ctx, resource)
		if err != nil {
			return fmt.Errorf("failed to get raw resource: %w", err)
		}

		spec, ok := res["spec"].(map[string]any)
		if !ok {
			return errors.New("unexpected response payload")
		}

		if err = w.processResource(ctx, resource, spec, email); err != nil {
			return fmt.Errorf("failed to re-check suspension status: %w", err)
		}

		return nil
	})
}

func (w *Watcher) processResource(
	ctx context.Context,
	resource k8s.Resource,
	spec map[string]any,
	updatedBy string,
) error {
	suspended, _ := spec["suspend"].(bool)

	entry, err := w.store.GetEntry(resource)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			// First time seeing the resource, so we'll save the state, but not notify - as we don't know what has
			// changed
			slog.Info(
				"new resource discovered",
				slog.String("kind", resource.Type.Kind),
				slog.String("resource", resource.Name),
				slog.Bool("suspended", suspended),
			)
			if err = w.store.SaveEntry(datastore.Entry{
				Resource:  resource,
				Suspended: suspended,
				UpdatedBy: updatedBy,
				UpdatedAt: time.Now().UTC(),
			}); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("failed to fetch entry: %w", err)
	}

	if suspended == entry.Suspended {
		return nil // Probably something else about the resource modified
	}

	entry.Resource = resource
	entry.Suspended = suspended
	entry.UpdatedBy = updatedBy
	entry.UpdatedAt = time.Now().UTC()

	if err = w.store.SaveEntry(entry); err != nil {
		return err
	}

	slog.Info(
		"suspension status updated",
		slog.String("kind", resource.Type.Kind),
		slog.String("resource", resource.Name),
		slog.String("user", updatedBy),
		slog.Bool("suspended", suspended),
	)

	return w.notifier.Notify(ctx, notification.Notification{
		Resource:             resource,
		Suspended:            suspended,
		Email:                updatedBy,
		GoogleCloudProjectID: w.googleCloudProjectID,
	})
}
