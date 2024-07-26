package notification

import (
	"context"

	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/k8s"
)

// Notification carries information relevant for dispatching external notifications
type Notification struct {
	Resource             k8s.ResourceReference
	Suspended            bool
	Email                string
	GoogleCloudProjectID string
}

// Notifier is the interface that is expected to be implemented for notification mechanisms
type Notifier interface {
	Notify(context.Context, Notification) error
}
