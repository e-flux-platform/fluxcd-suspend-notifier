package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SlackNotifier sends notifications to Slack via a webhook
type SlackNotifier struct {
	client     *http.Client
	webhookURL string
}

// NewSlackNotifier instantiates and returns SlackNotifier
func NewSlackNotifier(webhookURL string) (*SlackNotifier, error) {
	if webhookURL == "" {
		return nil, errors.New("empty webhook url supplied")
	}
	return &SlackNotifier{
		client: &http.Client{
			Timeout: time.Second * 5,
		},
		webhookURL: webhookURL,
	}, nil
}

// SlackWebhook is a Slack webhook payload
type SlackWebhook struct {
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment forms part of a Slack webhook payload
type SlackAttachment struct {
	Color      string                 `json:"color"`
	AuthorName string                 `json:"author_name"`
	Text       string                 `json:"text"`
	MrkdwnIn   []string               `json:"mrkdwn_in"`
	Fields     []SlackAttachmentField `json:"fields,omitempty"`
}

// SlackAttachmentField forms part of a Slack webhook attachment value
type SlackAttachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Notify sends a notification via the underlying Slack webhook URL.
func (sn *SlackNotifier) Notify(ctx context.Context, notif Notification) error {
	var (
		action string
		color  string
	)
	if notif.Suspended {
		action = "suspended"
		color = "danger"
	} else {
		action = "resumed"
		color = "good"
	}

	kind := strings.TrimSuffix(notif.Resource.Type.Kind, "s")

	reqBody, err := json.Marshal(SlackWebhook{
		Attachments: []SlackAttachment{
			{
				Color:      color,
				AuthorName: fmt.Sprintf("%s/%s.%s", kind, notif.Resource.Name, notif.Resource.Namespace),
				Text:       fmt.Sprintf("%s by %s", action, notif.Email),
				MrkdwnIn:   []string{"text"},
				Fields: []SlackAttachmentField{
					{
						Title: "project",
						Value: notif.GoogleCloudProjectID,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sn.webhookURL, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sn.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
