package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mtt_notify.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
		check   func(t *testing.T, cfg Config)
	}{
		{
			name: "full config",
			yaml: `
mytinytodo:
  url: https://todo.example.com
  password: secret
  list_id: 3
rules:
  ongoing_tag: urgent
  due_threshold_days: 3
schedule: "0 9 * * *"
timezone: America/New_York
template_path: /etc/mtt_notify/message.tmpl
notify:
  pushover:
    user: u1
    token: t1
  telegram:
    bot_token: bt
    chat_id: "42"
  webhook:
    url: https://hooks.example.com/x
    field: text
`,
			check: func(t *testing.T, cfg Config) {
				if cfg.MyTinyTodo.URL != "https://todo.example.com" {
					t.Errorf("url = %q", cfg.MyTinyTodo.URL)
				}
				if cfg.MyTinyTodo.ListID != 3 {
					t.Errorf("list_id = %d, want 3", cfg.MyTinyTodo.ListID)
				}
				if cfg.Rules.OngoingTag != "urgent" || cfg.Rules.DueThresholdDays != 3 {
					t.Errorf("rules = %+v", cfg.Rules)
				}
				if cfg.Notify.Pushover == nil || cfg.Notify.Telegram == nil || cfg.Notify.Webhook == nil {
					t.Errorf("notify sections missing: %+v", cfg.Notify)
				}
				if cfg.Notify.Webhook.Field != "text" {
					t.Errorf("webhook field = %q", cfg.Notify.Webhook.Field)
				}
				if cfg.Location == nil || cfg.Location.String() != "America/New_York" {
					t.Errorf("location = %v, want America/New_York", cfg.Location)
				}
			},
		},
		{
			name: "defaults applied",
			yaml: `
mytinytodo:
  url: https://todo.example.com
notify:
  pushover:
    user: u1
    token: t1
`,
			check: func(t *testing.T, cfg Config) {
				if cfg.MyTinyTodo.ListID != -1 {
					t.Errorf("default list_id = %d, want -1", cfg.MyTinyTodo.ListID)
				}
				if cfg.Rules.OngoingTag != "ongoing" {
					t.Errorf("default ongoing_tag = %q", cfg.Rules.OngoingTag)
				}
				if cfg.Rules.DueThresholdDays != 7 {
					t.Errorf("default due_threshold_days = %d", cfg.Rules.DueThresholdDays)
				}
				if cfg.Schedule != "30 20 * * *" {
					t.Errorf("default schedule = %q", cfg.Schedule)
				}
				// An empty timezone must mean host local time, not UTC:
				// time.LoadLocation("") would return UTC and silently
				// shift the schedule of existing installations.
				if cfg.Location != time.Local {
					t.Errorf("default location = %v, want %v", cfg.Location, time.Local)
				}
			},
		},
		{
			name: "invalid timezone",
			yaml: `
mytinytodo:
  url: https://todo.example.com
timezone: Mars/Olympus_Mons
`,
			wantErr: `invalid timezone "Mars/Olympus_Mons"`,
		},
		{
			name:    "missing url",
			yaml:    "schedule: \"* * * * *\"\n",
			wantErr: "mytinytodo.url is required",
		},
		{
			name: "incomplete pushover",
			yaml: `
mytinytodo:
  url: https://todo.example.com
notify:
  pushover:
    user: u1
`,
			wantErr: "notify.pushover requires both user and token",
		},
		{
			name: "incomplete telegram",
			yaml: `
mytinytodo:
  url: https://todo.example.com
notify:
  telegram:
    chat_id: "42"
`,
			wantErr: "notify.telegram requires both bot_token and chat_id",
		},
		{
			name:    "invalid yaml",
			yaml:    "mytinytodo: [unclosed",
			wantErr: "parse config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.yaml)
			cfg, _, err := Load(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestLoadRejectsOpenPermissions(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
		ok   bool
	}{
		{name: "0600 accepted", mode: 0o600, ok: true},
		{name: "0400 accepted", mode: 0o400, ok: true},
		{name: "0640 group-readable rejected", mode: 0o640},
		{name: "0644 world-readable rejected", mode: 0o644},
		{name: "0602 world-writable rejected", mode: 0o602},
	}
	yaml := "mytinytodo:\n  url: https://todo.example.com\n"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, yaml)
			if err := os.Chmod(path, tt.mode); err != nil {
				t.Fatal(err)
			}
			_, _, err := Load(path)
			if tt.ok {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "too open") {
				t.Fatalf("error = %v, want permissions error", err)
			}
			if !strings.Contains(err.Error(), "chmod 600") {
				t.Errorf("error should suggest chmod 600, got: %v", err)
			}
		})
	}
}

func TestLoadIncompleteBasicAuth(t *testing.T) {
	path := writeConfig(t, `
mytinytodo:
  url: https://todo.example.com
  basic_auth:
    user: stan
`)
	_, _, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "basic_auth requires both user and password") {
		t.Fatalf("error = %v, want basic_auth validation error", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, _, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v, want not found", err)
	}
}
