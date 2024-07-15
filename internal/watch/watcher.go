package watch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/genproto/googleapis/cloud/audit"

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
	GetRawResource(ctx context.Context, path string) (map[string]any, error)
}

type store interface {
	GetEntry(k8s.Resource) (datastore.Entry, error)
	SaveEntry(datastore.Entry) error
	AllEntries() ([]datastore.Entry, error)
}

type notifier interface {
	Notify(context.Context, notification.Notification) error
}

func (w *Watcher) Watch(ctx context.Context) error {
	// Re-check resource suspension states, in case any have been modified since the process was last running
	slog.Info("re-checking resource suspension states")
	entries, err := w.store.AllEntries()
	if err != nil {
		return fmt.Errorf("failed to fetch all entries: %w", err)
	}
	for _, entry := range entries {
		if err = w.checkSuspensionStatus(ctx, entry.Resource, "<unknown>"); err != nil {
			// We don't return an error here, as CRD versions are liable to change as fluxcd is upgraded
			slog.Warn("failed to re-check suspension status", slog.Any("error", err))
		}
	}

	// Watch for new modifications
	slog.Info("watching for resource modifications")
	return auditlog.Tail(ctx, w.googleCloudProjectID, w.gkeClusterName, func(logEntry *audit.AuditLog) error {
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

		if err = w.checkSuspensionStatus(ctx, resource, email); err != nil {
			return fmt.Errorf("failed to re-check suspension status: %w", err)
		}

		return nil
	})
}

func (w *Watcher) checkSuspensionStatus(ctx context.Context, resource k8s.Resource, updatedBy string) error {
	res, err := w.k8sClient.GetRawResource(ctx, resource.Path)
	if err != nil {
		return fmt.Errorf("failed to get raw resource: %w", err)
	}

	spec, ok := res["spec"].(map[string]any)
	if !ok {
		return errors.New("unexpected response payload")
	}
	suspended, _ := spec["suspend"].(bool)

	var updated bool
	entry, err := w.store.GetEntry(resource)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			// First time seeing the resource, so we'll save the state, but not notify - as we don't know what has
			// changed
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
	} else {
		updated = suspended != entry.Suspended
	}

	if !updated {
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
		slog.String("resource", resource.Path),
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
