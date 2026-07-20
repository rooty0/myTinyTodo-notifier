package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const defaultTelegramAPI = "https://api.telegram.org"

// Telegram sends messages through the Telegram Bot API.
type Telegram struct {
	BotToken string
	ChatID   string
	// API overrides the API base URL, for tests. Empty means the real API.
	API string

	client *http.Client
}

// NewTelegram creates a Telegram notifier.
func NewTelegram(botToken, chatID string) *Telegram {
	return &Telegram{BotToken: botToken, ChatID: chatID, client: newHTTPClient()}
}

func (t *Telegram) Name() string { return "telegram" }

// Send posts the message via the bot's sendMessage method.
func (t *Telegram) Send(ctx context.Context, message string) error {
	api := t.API
	if api == "" {
		api = defaultTelegramAPI
	}
	payload, err := json.Marshal(map[string]string{
		"chat_id": t.ChatID,
		"text":    message,
	})
	if err != nil {
		return fmt.Errorf("encode telegram payload: %w", err)
	}
	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", api, t.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("send to telegram: %w", err)
	}
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	return nil
}
