package notification

import "context"

type MultiNotifier struct {
	notifiers []Notifier
}

func NewMultiNotifier(notifiers []Notifier) *MultiNotifier {
	return &MultiNotifier{
		notifiers: notifiers,
	}
}

func (an *MultiNotifier) Notify(ctx context.Context, notif Notification) error {
	for _, n := range an.notifiers {
		if err := n.Notify(ctx, notif); err != nil {
			return err
		}
	}
	return nil
}
