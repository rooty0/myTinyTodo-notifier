package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Webhook posts the message as a small JSON object to an arbitrary URL.
// With Field set to "text" it is compatible with Slack/Mattermost incoming
// webhooks, with "content" with Discord ones.
type Webhook struct {
	URL   string
	Field string

	client *http.Client
}

// NewWebhook creates a webhook notifier. field defaults to "message".
func NewWebhook(url, field string) *Webhook {
	if field == "" {
		field = "message"
	}
	return &Webhook{URL: url, Field: field, client: newHTTPClient()}
}

func (w *Webhook) Name() string { return "webhook" }

// Send posts {"<field>": message} to the configured URL.
func (w *Webhook) Send(ctx context.Context, message string) error {
	payload, err := json.Marshal(map[string]string{w.Field: message})
	if err != nil {
		return fmt.Errorf("encode webhook payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send to webhook: %w", err)
	}
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("webhook: %w", err)
	}
	return nil
}
