package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type capture struct {
	path        string
	contentType string
	body        []byte
}

func captureServer(t *testing.T, status int, got *capture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		*got = capture{path: r.URL.Path, contentType: r.Header.Get("Content-Type"), body: body}
		w.WriteHeader(status)
	}))
}

func TestPushoverSend(t *testing.T) {
	var got capture
	srv := captureServer(t, http.StatusOK, &got)
	defer srv.Close()

	p := NewPushover("user1", "token1")
	p.URL = srv.URL + "/1/messages.json"
	if err := p.Send(context.Background(), "hello world"); err != nil {
		t.Fatal(err)
	}
	form, err := url.ParseQuery(string(got.body))
	if err != nil {
		t.Fatal(err)
	}
	if form.Get("token") != "token1" || form.Get("user") != "user1" || form.Get("message") != "hello world" {
		t.Errorf("form = %v", form)
	}
	if got.contentType != "application/x-www-form-urlencoded" {
		t.Errorf("content type = %q", got.contentType)
	}
}

func TestPushoverSendError(t *testing.T) {
	var got capture
	srv := captureServer(t, http.StatusBadRequest, &got)
	defer srv.Close()

	p := NewPushover("user1", "token1")
	p.URL = srv.URL
	if err := p.Send(context.Background(), "hello"); err == nil {
		t.Fatal("expected error on HTTP 400")
	}
}

func TestTelegramSend(t *testing.T) {
	var got capture
	srv := captureServer(t, http.StatusOK, &got)
	defer srv.Close()

	tg := NewTelegram("bot-token", "42")
	tg.API = srv.URL
	if err := tg.Send(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if got.path != "/botbot-token/sendMessage" {
		t.Errorf("path = %q", got.path)
	}
	var payload map[string]string
	if err := json.Unmarshal(got.body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["chat_id"] != "42" || payload["text"] != "hello" {
		t.Errorf("payload = %v", payload)
	}
}

func TestWebhookSend(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		wantField string
	}{
		{name: "default field", field: "", wantField: "message"},
		{name: "custom field", field: "text", wantField: "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got capture
			srv := captureServer(t, http.StatusOK, &got)
			defer srv.Close()

			w := NewWebhook(srv.URL, tt.field)
			if err := w.Send(context.Background(), "hello"); err != nil {
				t.Fatal(err)
			}
			var payload map[string]string
			if err := json.Unmarshal(got.body, &payload); err != nil {
				t.Fatal(err)
			}
			if payload[tt.wantField] != "hello" {
				t.Errorf("payload = %v, want %q in field %q", payload, "hello", tt.wantField)
			}
		})
	}
}

func TestWebhookSendError(t *testing.T) {
	var got capture
	srv := captureServer(t, http.StatusInternalServerError, &got)
	defer srv.Close()

	w := NewWebhook(srv.URL, "")
	if err := w.Send(context.Background(), "hello"); err == nil {
		t.Fatal("expected error on HTTP 500")
	}
}
