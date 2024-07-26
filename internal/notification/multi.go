package notification

import "context"

// MultiNotifier is used to broadcast notifications
type MultiNotifier struct {
	notifiers []Notifier
}

// NewMultiNotifier instantiates and returns MultiNotifier
func NewMultiNotifier(notifiers []Notifier) *MultiNotifier {
	return &MultiNotifier{
		notifiers: notifiers,
	}
}

// Notify passes the notification to all underlying notifiers
func (an *MultiNotifier) Notify(ctx context.Context, notif Notification) error {
	for _, n := range an.notifiers {
		if err := n.Notify(ctx, notif); err != nil {
			return err
		}
	}
	return nil
}
