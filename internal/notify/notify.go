// Package notify delivers rendered notification messages.
package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Notifier sends one notification message to a single delivery method.
type Notifier interface {
	Name() string
	Send(ctx context.Context, message string) error
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// checkResponse drains the body and converts non-2xx statuses into errors.
func checkResponse(resp *http.Response) error {
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
