package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type SlackNotifier struct {
	client     *http.Client
	webhookURL string
}

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

type SlackWebhook struct {
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

type SlackAttachment struct {
	Color      string                 `json:"color"`
	AuthorName string                 `json:"author_name"`
	Text       string                 `json:"text"`
	MrkdwnIn   []string               `json:"mrkdwn_in"`
	Fields     []SlackAttachmentField `json:"fields,omitempty"`
}

type SlackAttachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func (sn *SlackNotifier) Notify(ctx context.Context, notif Notification) error {
	var (
		action string
		color  string
	)
	if notif.Suspended {
		action = "suspended"
		color = "danger"
	} else {
		action = "unsuspended"
		color = "good"
	}

	reqBody, err := json.Marshal(SlackWebhook{
		Attachments: []SlackAttachment{
			{
				Color:      color,
				AuthorName: fmt.Sprintf("%s/%s.%s", notif.Resource.Kind, notif.Resource.Name, notif.Resource.Namespace),
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
