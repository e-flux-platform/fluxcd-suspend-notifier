package watch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/genproto/googleapis/cloud/audit"

	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/auditlog"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/datastore"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/k8s"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/notification"
)

type Watcher struct {
	googleCloudProjectID string
	k8sClient            k8sClient
	store                store
	notifier             notifier
}

func NewWatcher(
	googleCloudProjectID string,
	k8sClient k8sClient,
	store store,
	notifier notifier,
) *Watcher {
	return &Watcher{
		googleCloudProjectID: googleCloudProjectID,
		k8sClient:            k8sClient,
		store:                store,
		notifier:             notifier,
	}
}

type k8sClient interface {
	GetRawResource(ctx context.Context, path string) (map[string]any, error)
}

type store interface {
	IsSuspended(resource k8s.Resource) (bool, error)
	SetSuspended(resource k8s.Resource, suspended bool) error
}

type notifier interface {
	Notify(context.Context, notification.Notification) error
}

func (w *Watcher) Watch(ctx context.Context) error {
	return auditlog.Tail(ctx, w.googleCloudProjectID, func(logEntry *audit.AuditLog) error {
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

		res, err := w.k8sClient.GetRawResource(ctx, resourceName)
		if err != nil {
			return fmt.Errorf("failed to get raw resource: %w", err)
		}

		spec, ok := res["spec"].(map[string]any)
		if !ok {
			return errors.New("unexpected response payload")
		}
		isSuspended, _ := spec["suspend"].(bool)

		var modified bool
		wasSuspended, err := w.store.IsSuspended(resource)
		if err != nil {
			if !errors.Is(err, datastore.ErrNotFound) {
				return err
			}
			modified = true
		} else {
			modified = isSuspended != wasSuspended
		}

		if !modified {
			return nil // Probably something else about the resource modified
		}

		if err = w.store.SetSuspended(resource, isSuspended); err != nil {
			return err
		}

		slog.Info(
			"suspension status modified",
			slog.String("resource", resourceName),
			slog.String("user", email),
			slog.Bool("suspended", isSuspended),
		)

		return w.notifier.Notify(ctx, notification.Notification{
			Resource:             resource,
			Suspended:            isSuspended,
			Email:                email,
			GoogleCloudProjectID: w.googleCloudProjectID,
		})
	})
}
