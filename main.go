package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/config"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/datastore"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/k8s"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/notification"
	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/watch"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		return errors.New("config path environment variable not set")
	}

	conf, err := config.Parse(configPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	k8sClient, err := k8s.NewClient(conf.KubernetesConfigPath)
	if err != nil {
		return err
	}

	store, err := datastore.NewBadgerStore(conf.BadgerPath)
	if err != nil {
		return err
	}
	defer store.Close()

	notifiers := make([]notification.Notifier, 0, len(conf.Notification.Slack))
	for _, slack := range conf.Notification.Slack {
		var notifier notification.Notifier
		notifier, err = notification.NewSlackNotifier(slack.WebhookURL)
		if err != nil {
			return fmt.Errorf("failed to create slack notifier: %w", err)
		}
		if slack.Filter != "" {
			notifier, err = notification.NewFilteringNotifier(slack.Filter, notifier)
			if err != nil {
				return fmt.Errorf("failed to create filtering notifier: %w", err)
			}
		}
		notifiers = append(notifiers, notifier)
	}

	watcher := watch.NewWatcher(
		conf.GoogleCloudProjectID,
		conf.GKEClusterName,
		k8sClient,
		store,
		notification.NewAggregateNotifier(notifiers),
	)

	return watcher.Watch(ctx)
}
