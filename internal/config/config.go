// Package config loads and validates the notifier YAML configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// AppName is used to build the default configuration search paths.
const AppName = "mtt_notify"

// Config is the root of the YAML configuration file.
type Config struct {
	MyTinyTodo MyTinyTodo `yaml:"mytinytodo"`
	Rules      Rules      `yaml:"rules"`
	// Schedule is a standard 5-field cron expression evaluated in Location.
	// It is kept at the top level for now; if rules ever need independent
	// timing, each rule can grow its own schedule.
	Schedule string `yaml:"schedule"`
	// Timezone names the IANA zone (e.g. "America/New_York") used for both
	// the schedule and the due date comparisons. Empty means host local time.
	Timezone     string `yaml:"timezone"`
	TemplatePath string `yaml:"template_path"`
	Notify       Notify `yaml:"notify"`
	// Location is Timezone resolved at load time.
	Location *time.Location `yaml:"-"`
}

// MyTinyTodo describes how to reach the myTinyTodo API.
type MyTinyTodo struct {
	// URL is the base URL of the myTinyTodo installation,
	// e.g. "https://todo.example.com" (the directory containing api.php).
	URL      string `yaml:"url"`
	Password string `yaml:"password"`
	// BasicAuth adds HTTP Basic Auth credentials to every request, for
	// installations behind a web-server auth layer (htpasswd etc.).
	BasicAuth *BasicAuth `yaml:"basic_auth"`
	// ListID selects which list to query; -1 means all lists.
	ListID int `yaml:"list_id"`
}

// BasicAuth holds HTTP Basic Auth credentials.
type BasicAuth struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// Rules controls which tasks end up in the notification.
type Rules struct {
	// OngoingTag marks tasks that are always reported. Empty disables the section.
	OngoingTag string `yaml:"ongoing_tag"`
	// DueThresholdDays reports tasks due within this many days (overdue included).
	DueThresholdDays int `yaml:"due_threshold_days"`
}

// Notify lists delivery methods; a method is enabled when its section is present.
type Notify struct {
	Pushover *Pushover `yaml:"pushover"`
	Telegram *Telegram `yaml:"telegram"`
	Webhook  *Webhook  `yaml:"webhook"`
}

// Pushover holds credentials for the Pushover message API.
type Pushover struct {
	User  string `yaml:"user"`
	Token string `yaml:"token"`
}

// Telegram holds credentials for the Telegram Bot API.
type Telegram struct {
	BotToken string `yaml:"bot_token"`
	// ChatID is a numeric chat id or a "@channelname".
	ChatID string `yaml:"chat_id"`
}

// Webhook posts the rendered message as JSON to an arbitrary URL.
type Webhook struct {
	URL string `yaml:"url"`
	// Field is the JSON field name carrying the message, e.g. "text" for
	// Slack-compatible webhooks. Defaults to "message".
	Field string `yaml:"field"`
}

func defaults() Config {
	return Config{
		MyTinyTodo: MyTinyTodo{ListID: -1},
		Rules: Rules{
			OngoingTag:       "ongoing",
			DueThresholdDays: 7,
		},
		Schedule: "30 20 * * *",
	}
}

// searchPaths returns the locations probed when no explicit path is given.
func searchPaths() []string {
	cwd, err := os.Getwd()
	paths := []string{}
	if err == nil {
		paths = append(paths, fmt.Sprintf("%s/%s.yaml", cwd, AppName))
	}
	return append(paths,
		fmt.Sprintf("/etc/%s/%s.yaml", AppName, AppName),
		fmt.Sprintf("/etc/%s.yaml", AppName),
		fmt.Sprintf("/usr/local/etc/%s/%s.yaml", AppName, AppName),
		fmt.Sprintf("/usr/local/etc/%s.yaml", AppName),
	)
}

// Load reads the configuration from path, or from the first existing
// well-known location when path is empty.
func Load(path string) (Config, string, error) {
	paths := []string{path}
	if path == "" {
		paths = searchPaths()
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Config{}, p, fmt.Errorf("read config: %w", err)
		}
		if err := checkPermissions(p); err != nil {
			return Config{}, p, err
		}
		cfg, err := parse(data)
		if err != nil {
			return Config{}, p, err
		}
		return cfg, p, nil
	}
	return Config{}, "", fmt.Errorf("configuration file not found (looked in %v)", paths)
}

// checkPermissions rejects config files accessible by group or others,
// like ssh does for private keys: the file holds credentials, so at most
// 0600 (0700) is allowed.
func checkPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat config: %w", err)
	}
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		return fmt.Errorf(
			"permissions %04o for %q are too open: the file contains credentials and must not be accessible by group/others; fix with: chmod 600 %s",
			mode, path, path)
	}
	return nil
}

func parse(data []byte) (Config, error) {
	cfg := defaults()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	loc, err := resolveLocation(cfg.Timezone)
	if err != nil {
		return Config{}, err
	}
	cfg.Location = loc
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// resolveLocation turns a timezone name into a *time.Location. An empty
// name means host local time; note that time.LoadLocation("") returns UTC
// instead, which would silently shift existing installations.
func resolveLocation(name string) (*time.Location, error) {
	if name == "" {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", name, err)
	}
	return loc, nil
}

func (c Config) validate() error {
	if c.MyTinyTodo.URL == "" {
		return errors.New("mytinytodo.url is required")
	}
	if c.Schedule == "" {
		return errors.New("schedule is required")
	}
	if c.Rules.DueThresholdDays < 0 {
		return errors.New("rules.due_threshold_days must not be negative")
	}
	if b := c.MyTinyTodo.BasicAuth; b != nil && (b.User == "" || b.Password == "") {
		return errors.New("mytinytodo.basic_auth requires both user and password")
	}
	if p := c.Notify.Pushover; p != nil && (p.User == "" || p.Token == "") {
		return errors.New("notify.pushover requires both user and token")
	}
	if t := c.Notify.Telegram; t != nil && (t.BotToken == "" || t.ChatID == "") {
		return errors.New("notify.telegram requires both bot_token and chat_id")
	}
	if w := c.Notify.Webhook; w != nil && w.URL == "" {
		return errors.New("notify.webhook requires url")
	}
	return nil
}
