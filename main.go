// Command mtt_notify watches a myTinyTodo installation and sends
// notifications about ongoing and soon-due tasks.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"text/template"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/rooty0/myTinyTodo-notifier/internal/config"
	"github.com/rooty0/myTinyTodo-notifier/internal/mtt"
	"github.com/rooty0/myTinyTodo-notifier/internal/notify"
	"github.com/rooty0/myTinyTodo-notifier/internal/report"
)

// version is set at build time via -ldflags (see Makefile).
var version = "dev"

// passTimeout bounds a single notification pass (API calls + deliveries).
const passTimeout = 2 * time.Minute

func main() {
	if err := run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath    = flag.String("config", "", "custom configuration file path")
		debug         = flag.Bool("debug", false, "print debugging messages about progress")
		silent        = flag.Bool("silent", false, "output errors only")
		once          = flag.Bool("once", false, "run a single notification pass and exit (crontab-style)")
		disableNotify = flag.Bool("disable-notification", false, "do not send actual notifications, useful for debugging")
		showVersion   = flag.Bool("version", false, "print version and exit")
	)
	flag.StringVar(configPath, "c", "", "custom configuration file path (shorthand)")
	flag.BoolVar(debug, "d", false, "print debugging messages (shorthand)")
	flag.BoolVar(silent, "s", false, "output errors only (shorthand)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("myTinyTodo Notifier v%s\n", version)
		return nil
	}

	level := slog.LevelInfo
	switch {
	case *debug:
		level = slog.LevelDebug
	case *silent:
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	cfg, cfgPath, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	slog.Debug("configuration loaded", "path", cfgPath)

	tmpl, err := report.LoadTemplate(cfg.TemplatePath)
	if err != nil {
		return err
	}

	notifiers := buildNotifiers(cfg.Notify)
	if len(notifiers) == 0 && !*disableNotify {
		return errors.New("no delivery method configured: add a notify section (pushover, telegram or webhook) to the config")
	}

	app := &app{
		cfg:           cfg,
		tmpl:          tmpl,
		notifiers:     notifiers,
		disableNotify: *disableNotify,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *once {
		return app.pass(ctx)
	}
	return runDaemon(ctx, cfg.Schedule, cfg.Location, app)
}

// runDaemon executes a notification pass on the configured cron schedule,
// evaluated in loc, until the context is cancelled by a signal.
func runDaemon(ctx context.Context, schedule string, loc *time.Location, app *app) error {
	c := cron.New(cron.WithLocation(loc))
	_, err := c.AddFunc(schedule, func() {
		if err := app.pass(ctx); err != nil {
			slog.Error("notification pass failed", "error", err)
		}
	})
	if err != nil {
		return fmt.Errorf("invalid schedule %q: %w", schedule, err)
	}

	c.Start()
	if entries := c.Entries(); len(entries) > 0 {
		slog.Info("daemon started", "schedule", schedule, "timezone", loc, "next_run", entries[0].Next)
	}

	<-ctx.Done()
	slog.Info("shutting down")
	<-c.Stop().Done()
	return nil
}

func buildNotifiers(n config.Notify) []notify.Notifier {
	var notifiers []notify.Notifier
	if n.Pushover != nil {
		notifiers = append(notifiers, notify.NewPushover(n.Pushover.User, n.Pushover.Token))
	}
	if n.Telegram != nil {
		notifiers = append(notifiers, notify.NewTelegram(n.Telegram.BotToken, n.Telegram.ChatID))
	}
	if n.Webhook != nil {
		notifiers = append(notifiers, notify.NewWebhook(n.Webhook.URL, n.Webhook.Field))
	}
	return notifiers
}

type app struct {
	cfg           config.Config
	tmpl          *template.Template
	notifiers     []notify.Notifier
	disableNotify bool
}

// pass performs one full notification round: query the API, build the
// report, render it and fan it out to all configured delivery methods.
func (a *app) pass(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, passTimeout)
	defer cancel()

	client, err := mtt.NewClient(a.cfg.MyTinyTodo.URL, a.cfg.MyTinyTodo.Password)
	if err != nil {
		return err
	}
	if b := a.cfg.MyTinyTodo.BasicAuth; b != nil {
		client.SetBasicAuth(b.User, b.Password)
	}
	client.SetLocation(a.cfg.Location)
	if err := client.Login(ctx); err != nil {
		return err
	}

	var ongoing []mtt.Task
	if tag := a.cfg.Rules.OngoingTag; tag != "" {
		ongoing, err = client.Tasks(ctx, a.cfg.MyTinyTodo.ListID, tag)
		if err != nil {
			return fmt.Errorf("ongoing tasks: %w", err)
		}
		slog.Debug("fetched ongoing tasks", "tag", tag, "count", len(ongoing))
	}

	due, err := client.Tasks(ctx, a.cfg.MyTinyTodo.ListID, "")
	if err != nil {
		return fmt.Errorf("due tasks: %w", err)
	}
	slog.Debug("fetched tasks for due date scan", "count", len(due))

	data := report.Build(ongoing, due, time.Now().In(a.cfg.Location), a.cfg.Rules.DueThresholdDays)
	if data.Empty() {
		slog.Info("nothing to report")
		return nil
	}
	slog.Info("tasks to report", "ongoing", len(data.Ongoing), "due", len(data.Due))

	message, err := report.Render(a.tmpl, data)
	if err != nil {
		return err
	}
	slog.Debug("rendered notification", "message", message)

	if a.disableNotify {
		slog.Info("not notifying because instructed to do so", "message", message)
		return nil
	}

	var errs []error
	for _, n := range a.notifiers {
		if err := n.Send(ctx, message); err != nil {
			errs = append(errs, err)
			slog.Error("delivery failed", "notifier", n.Name(), "error", err)
			continue
		}
		slog.Info("notification sent", "notifier", n.Name())
	}
	return errors.Join(errs...)
}
