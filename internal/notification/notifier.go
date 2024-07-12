package notification

import (
	"context"

	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/k8s"
)

type Notification struct {
	Resource             k8s.Resource
	Suspended            bool
	Email                string
	GoogleCloudProjectID string
}

type Notifier interface {
	Notify(context.Context, Notification) error
}
