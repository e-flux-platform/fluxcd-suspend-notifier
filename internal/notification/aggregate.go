package notification

import "context"

type AggregateNotifier struct {
	notifiers []Notifier
}

func NewAggregateNotifier(notifiers []Notifier) *AggregateNotifier {
	return &AggregateNotifier{
		notifiers: notifiers,
	}
}

func (an *AggregateNotifier) Notify(ctx context.Context, notif Notification) error {
	for _, n := range an.notifiers {
		if err := n.Notify(ctx, notif); err != nil {
			return err
		}
	}
	return nil
}
