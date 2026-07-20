package notify

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const defaultPushoverURL = "https://api.pushover.net/1/messages.json"

// Pushover sends messages through the Pushover message API.
type Pushover struct {
	User  string
	Token string
	// URL overrides the API endpoint, for tests. Empty means the real API.
	URL string

	client *http.Client
}

// NewPushover creates a Pushover notifier.
func NewPushover(user, token string) *Pushover {
	return &Pushover{User: user, Token: token, client: newHTTPClient()}
}

func (p *Pushover) Name() string { return "pushover" }

// Send posts the message to the Pushover API.
func (p *Pushover) Send(ctx context.Context, message string) error {
	endpoint := p.URL
	if endpoint == "" {
		endpoint = defaultPushoverURL
	}
	form := url.Values{
		"token":   {p.Token},
		"user":    {p.User},
		"message": {message},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build pushover request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send to pushover: %w", err)
	}
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("pushover: %w", err)
	}
	return nil
}
