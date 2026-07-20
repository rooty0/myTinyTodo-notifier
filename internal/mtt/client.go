// Package mtt is a minimal client for the myTinyTodo HTTP API.
package mtt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Client talks to a myTinyTodo installation via its HTTP API.
//
// Call Login before Tasks: the API protects every endpoint with an MTT-Token
// header tied to the PHP session, even when no password is configured.
type Client struct {
	base      *url.URL // installation directory, without trailing slash
	apiPrefix string   // API endpoint prefix, discovered from the index page
	password  string
	basicUser string
	basicPass string
	http      *http.Client
	token     string
	loc       *time.Location // zone the due dates are interpreted in
}

// Task is a single myTinyTodo task in the fields the notifier cares about.
type Task struct {
	ID    int
	Title string
	// Due is midnight (local time) of the task due date; zero when unset.
	Due time.Time
}

// HasDue reports whether the task has a due date.
func (t Task) HasDue() bool { return !t.Due.IsZero() }

// SetBasicAuth makes the client send HTTP Basic Auth credentials with
// every request, for installations behind a web-server auth layer.
func (c *Client) SetBasicAuth(user, password string) {
	c.basicUser = user
	c.basicPass = password
}

// SetLocation sets the time zone in which task due dates are interpreted.
// It must match the zone the report is built in, otherwise the due date
// comparisons are skewed by the offset between the two. Defaults to
// time.Local.
func (c *Client) SetLocation(loc *time.Location) {
	c.loc = loc
}

// authorize attaches the auth headers common to all requests.
func (c *Client) authorize(req *http.Request) {
	if c.basicUser != "" {
		req.SetBasicAuth(c.basicUser, c.basicPass)
	}
	if c.token != "" {
		req.Header.Set("MTT-Token", c.token)
	}
}

// NewClient creates a client for the myTinyTodo installation at rawURL
// (the directory that contains api.php). password may be empty when the
// installation has no password protection.
func NewClient(rawURL, password string) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(rawURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse mytinytodo url: %w", err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, fmt.Errorf("mytinytodo url %q must start with http:// or https://", rawURL)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	return &Client{
		base:      base,
		apiPrefix: base.String() + "/api.php?_path=/",
		password:  password,
		loc:       time.Local,
		http: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
	}, nil
}

type sessionResponse struct {
	Token    string `json:"token"`
	Disabled int    `json:"disabled"`
}

type loginResponse struct {
	Logged int    `json:"logged"`
	Token  string `json:"token"`
}

// Login establishes a session and obtains the MTT-Token required by all
// other endpoints. It is safe to call again to refresh an expired session.
func (c *Client) Login(ctx context.Context) error {
	if err := c.fetchIndex(ctx); err != nil {
		return err
	}

	var sess sessionResponse
	if err := c.do(ctx, http.MethodPost, "session", nil, nil, &sess); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	if sess.Disabled == 1 {
		// No password on the server: the token lives in the mtt-token
		// cookie, which the index page has just issued.
		return c.anonymousToken()
	}

	c.token = sess.Token
	var login loginResponse
	if err := c.do(ctx, http.MethodPost, "login", nil, map[string]string{"password": c.password}, &login); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if login.Logged != 1 {
		return fmt.Errorf("login rejected: check the configured password")
	}
	c.token = login.Token
	return nil
}

// apiURLPattern matches the "apiUrl" entry of the JSON options object the
// index page hands to the official web client.
var apiURLPattern = regexp.MustCompile(`"apiUrl"\s*:\s*("(?:[^"\\]|\\.)*")`)

// fetchIndex loads the installation's index page. The page publishes the
// API endpoint URL, which differs between installations: api.php?_path=/
// by default, or a rewritten path like /api/ when the server is set up
// with MTT_API_USE_PATH_INFO. On password-less installations the page also
// issues the mtt-token cookie consumed by anonymousToken.
func (c *Client) fetchIndex(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base.String()+"/", nil)
	if err != nil {
		return fmt.Errorf("build index request: %w", err)
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetch index page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read index page: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("index page returned HTTP %d", resp.StatusCode)
	}

	m := apiURLPattern.FindSubmatch(body)
	if m == nil {
		return nil // keep the default api.php?_path=/ style
	}
	var apiURL string
	if err := json.Unmarshal(m[1], &apiURL); err != nil {
		return fmt.Errorf("decode api url from index page: %w", err)
	}
	// The value may be absolute or relative to the installation directory.
	base, err := url.Parse(c.base.String() + "/")
	if err != nil {
		return fmt.Errorf("parse base url: %w", err)
	}
	resolved, err := base.Parse(apiURL)
	if err != nil {
		return fmt.Errorf("resolve api url %q: %w", apiURL, err)
	}
	c.apiPrefix = resolved.String()
	return nil
}

// anonymousToken uses the mtt-token cookie issued by the index page as the
// MTT-Token header value.
func (c *Client) anonymousToken() error {
	for _, cookie := range c.http.Jar.Cookies(c.base) {
		if cookie.Name == "mtt-token" {
			c.token = cookie.Value
			return nil
		}
	}
	return fmt.Errorf("server did not issue an mtt-token cookie; is the url pointing at a myTinyTodo installation")
}

type tasksResponse struct {
	Total  int        `json:"total"`
	Denied int        `json:"denied"`
	List   []taskJSON `json:"list"`
}

type taskJSON struct {
	ID    int    `json:"id"`
	Title string `json:"titleText"`
	// DueInt is the due date as YYYYMMDD; 33330000 means no due date.
	DueInt int `json:"dueInt"`
}

const noDueDate = 33330000

// Tasks returns incomplete tasks of the given list (-1 for all lists),
// optionally filtered by tag name.
func (c *Client) Tasks(ctx context.Context, listID int, tag string) ([]Task, error) {
	params := url.Values{
		"list":  {strconv.Itoa(listID)},
		"compl": {"0"},
	}
	if tag != "" {
		params.Set("t", tag)
	}
	var tr tasksResponse
	if err := c.do(ctx, http.MethodGet, "tasks", params, nil, &tr); err != nil {
		return nil, fmt.Errorf("get tasks: %w", err)
	}
	if tr.Denied == 1 {
		return nil, fmt.Errorf("access to tasks denied: check password and list visibility")
	}

	tasks := make([]Task, 0, len(tr.List))
	for _, t := range tr.List {
		tasks = append(tasks, Task{
			ID:    t.ID,
			Title: t.Title,
			Due:   dueIntToTime(t.DueInt, c.loc),
		})
	}
	return tasks, nil
}

// dueIntToTime converts a YYYYMMDD due date into midnight of that day in
// loc. The API reports a calendar date, so the zone has to be supplied by
// the caller rather than assumed.
func dueIntToTime(v int, loc *time.Location) time.Time {
	if v <= 0 || v == noDueDate {
		return time.Time{}
	}
	y, m, d := v/10000, (v/100)%100, v%100
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, loc)
}

// endpoint builds the URL of an API route the same way the official web
// client does: the route (no leading slash) is appended to the API prefix,
// and query parameters follow with '&' when the prefix already carries a
// query string (api.php?_path=/) or '?' otherwise (/api/).
func (c *Client) endpoint(route string, params url.Values) string {
	u := c.apiPrefix + route
	if len(params) == 0 {
		return u
	}
	sep := "?"
	if strings.Contains(c.apiPrefix, "?") {
		sep = "&"
	}
	return u + sep + params.Encode()
}

// do performs an API request against the given route (without leading
// slash), encoding body as JSON when non-nil and decoding the JSON
// response into out.
func (c *Client) do(ctx context.Context, method, route string, params url.Values, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(route, params), reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.authorize(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", route, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read response of %s: %w", route, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP %d: %s", route, resp.StatusCode, truncate(string(data), 200))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response of %s: %w", route, err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
