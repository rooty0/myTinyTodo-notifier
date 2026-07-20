# myTinyTodo-notifier
Notifier for myTinyTodo [(maxpozdeev/mytinytodo)](https://github.com/maxpozdeev/mytinytodo)

![Screen Shot](https://repository-images.githubusercontent.com/622862195/3bd4e263-c7fd-409d-a30d-f9bef9abf5a6)

Never miss your task deadline again!

This tool expands the capabilities of [myTinyTodo](https://www.mytinytodo.net/) by providing notifications based on the following:
- due date (tasks overdue, due today, or due within a configurable number of days)
- special tag (e.g. `ongoing` — tasks that should always be reported)

Supported delivery methods:
- [Pushover](https://pushover.net/)
- Telegram (Bot API)
- Generic webhook (Slack / Mattermost / Discord compatible)

Written in Go. Talks to your myTinyTodo installation over its HTTP API — no direct database access required, so it can run on a different machine than the todo app itself. The API endpoint is discovered automatically from the installation's index page, so both the default `api.php?_path=` routing and pretty-URL setups (`MTT_API_USE_PATH_INFO` with an `/api/` rewrite) work out of the box.

## Installation

```bash
make build                # embeds the version from git into the binary
sudo make install         # installs to /usr/local/bin (override with PREFIX=...)
```

The Makefile works with both GNU make and BSD make. If your Go toolchain is installed under a versioned name (e.g. FreeBSD's `lang/go125` port ships `/usr/local/bin/go125`), point the `GO` variable at it:

```bash
make build GO=go125
```

Alternatively, install the `lang/go` meta-port, which provides the unversioned `go` symlink.

## Configuration

Copy `mtt_notify.example.yaml` to one of the locations below and adjust it (the first existing file wins), or pass a custom location with `--config`:

- `./mtt_notify.yaml`
- `/etc/mtt_notify/mtt_notify.yaml`
- `/etc/mtt_notify.yaml`
- `/usr/local/etc/mtt_notify/mtt_notify.yaml`
- `/usr/local/etc/mtt_notify.yaml`

The config file contains credentials, so — like an SSH private key — it must not be accessible by group or others. The notifier refuses to start otherwise:

```bash
chmod 600 mtt_notify.yaml
```

```yaml
mytinytodo:
  url: https://todo.example.com   # base URL of the installation (directory containing api.php)
  password: xxxxxxxxxx            # leave empty if the installation has no password
  basic_auth:                     # optional: HTTP Basic Auth (htpasswd-protected installations)
    user: xxxxxxxxxx
    password: xxxxxxxxxx
  list_id: -1                     # -1 = all lists

rules:
  ongoing_tag: ongoing            # tasks with this tag are always reported; empty disables
  due_threshold_days: 7           # report tasks due within N days (overdue always included)

schedule: "30 20 * * *"           # when to send notifications (5-field cron expression)
timezone: America/New_York        # IANA zone for the schedule and due dates; empty = host local time

template_path: ""                 # optional custom message template; empty = built-in default

notify:                           # enable any subset
  pushover:
    user: xxxxxxxxxx
    token: xxxxxxxxxx
  telegram:
    bot_token: "123456:ABC-DEF..."
    chat_id: "123456789"          # numeric chat id or "@channelname"
  webhook:
    url: https://hooks.example.com/services/xxx
    field: text                   # JSON field name: "text" for Slack, "content" for Discord
```

### Timezone

`timezone` takes an [IANA zone name](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) (e.g. `Europe/Berlin`) and applies to both the schedule and the "overdue / due today" comparisons, so a task is due today according to *your* day, not the server's. Leaving it empty or omitting it keeps the host's local time, which is the previous behavior.

The zone is resolved from the system tzdata at startup, and an unknown name aborts with an error instead of being silently ignored. Systems without tzdata installed (a scratch container, for instance) would need the database compiled into the binary via a blank `import _ "time/tzdata"`.

The cron library also honors a `CRON_TZ=` prefix inside the schedule itself (`schedule: "CRON_TZ=Asia/Tokyo 30 20 * * *"`). It overrides `timezone` for the firing time only, leaving the due date comparisons on `timezone` — mixing the two is a good way to confuse yourself, so prefer `timezone` alone.

## Usage

Run as a daemon; notifications are sent on the configured cron schedule:

```bash
mtt_notify
```

Useful flags:

```text
-c, --config PATH        custom configuration file path
    --once               run a single notification pass and exit (crontab-style)
    --disable-notification  render the message but do not send it, useful for debugging
-d, --debug              print debugging messages about progress
-s, --silent             output errors only
    --version            print version and exit
```

Test your setup without sending anything:

```bash
mtt_notify --once --disable-notification --debug
```

### Running under systemd

A ready-to-use unit file ships with the repository: [`init/systemd/mtt_notify.service`](init/systemd/mtt_notify.service).

Install it, create the service user, and hand it the config (the 0600 check means the service user must own the file):

```bash
sudo cp init/systemd/mtt_notify.service /etc/systemd/system/
sudo useradd --system --shell /usr/sbin/nologin mtt_notify
sudo chown mtt_notify /etc/mtt_notify.yaml
sudo chmod 600 /etc/mtt_notify.yaml
sudo systemctl enable --now mtt_notify
```

### Running under FreeBSD (rc.d)

A ready-to-use rc script ships with the repository: [`init/freebsd/mtt_notify`](init/freebsd/mtt_notify). It runs the notifier via `daemon(8)` with logs going to syslog (`-S`). FreeBSD's `/usr/local/etc/mtt_notify.yaml` is already in the config search paths.

Install, enable and start it (the run-as user — `nobody` by default, override with `sysrc mtt_notify_user=...` — must own the config because of the 0600 check):

```bash
cp init/freebsd/mtt_notify /usr/local/etc/rc.d/
chmod +x /usr/local/etc/rc.d/mtt_notify
chown nobody /usr/local/etc/mtt_notify.yaml
chmod 600 /usr/local/etc/mtt_notify.yaml
sysrc mtt_notify_enable=YES
service mtt_notify start
```

The old crontab workflow still works if you prefer it — schedule `mtt_notify --once --silent` instead of running the daemon.

## Message templates

The notification text is rendered with Go's [text/template](https://pkg.go.dev/text/template). Point `template_path` at your own file to customize it; the built-in default lives at [`internal/report/templates/message.tmpl`](internal/report/templates/message.tmpl):

```gotemplate
Hi,

{{ if .Ongoing -}}
=== Ongoing tasks: ===
{{ range .Ongoing -}}
- {{ .Title }}
{{ end -}}
{{ end -}}
{{ if .Due -}}
=== Due date: ===
{{ range .Due -}}
- "{{ .Title }}" {{ if .Overdue }}EXPIRED{{ else if .DueToday }}DUE TODAY{{ else }}due{{ end }} on {{ .Due.Format "Monday, January 2" }} ({{ .Due.Format "01/02" }})
{{ end -}}
{{ end -}}
```

Template data:

- `.Ongoing` — tasks carrying the configured tag; each item has `.Title`
- `.Due` — tasks due soon; each item has `.Title`, `.Due` (a Go `time.Time`), `.Overdue` and `.DueToday` booleans

No notification is sent when both sections are empty.

## Migrating from the Python version

- The tool now reads tasks via the myTinyTodo HTTP API instead of the SQLite database: replace `database_path` with the `mytinytodo` section (`url` + `password`).
- The "ongoing" tag is matched by name (`rules.ongoing_tag`) instead of a hardcoded tag id.
- The due date window is configurable (`rules.due_threshold_days`) and only includes tasks that are overdue or due within the window.
- Pushover credentials moved from `pushover_user` / `pushover_token` to `notify.pushover.user` / `notify.pushover.token`.
- It runs as a daemon with a built-in cron schedule (`schedule`); use `--once` to keep the old crontab behavior.

## Development

Run `make help` to list all available targets.

## Future Improvements

- per-rule schedules (send different task groups at different times)
- more delivery methods

## Contribute

Feel free to create a PR
