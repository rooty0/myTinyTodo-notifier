package mtt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeServer emulates the parts of the myTinyTodo API the client uses.
type fakeServer struct {
	password  string
	token     string
	loggedIn  bool
	tasksJSON string
	lastQuery map[string]string
	// basicUser/basicPass, when set, put the whole server behind
	// HTTP Basic Auth like a web-server htpasswd layer would.
	basicUser string
	basicPass string
	// pathInfo switches the server to MTT_API_USE_PATH_INFO mode: routes
	// live under /api/ and api.php?_path= requests fail like on a real
	// server without PATH_INFO set.
	pathInfo bool
	// omitAPIURL drops the apiUrl entry from the index page.
	omitAPIURL bool
}

// route extracts the API route of a request, honoring the routing style.
func (f *fakeServer) route(r *http.Request) (string, bool) {
	if f.pathInfo {
		if p, ok := strings.CutPrefix(r.URL.Path, "/api/"); ok {
			return "/" + p, true
		}
		return "", false
	}
	if r.URL.Path == "/api.php" {
		return r.URL.Query().Get("_path"), true
	}
	return "", false
}

func (f *fakeServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.basicUser != "" {
			user, pass, ok := r.BasicAuth()
			if !ok || user != f.basicUser || pass != f.basicPass {
				w.Header().Set("WWW-Authenticate", `Basic realm="todo"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		if f.pathInfo && r.URL.Path == "/api.php" {
			// real servers in this mode fail with a PATH_INFO warning
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `Warning: 'Undefined array key "PATH_INFO"'`)
			return
		}
		route, ok := f.route(r)
		if !ok {
			// index page: publishes the API endpoint and issues the
			// anonymous token cookie
			http.SetCookie(w, &http.Cookie{Name: "mtt-token", Value: "anon-token", Path: "/"})
			apiURL := "api.php?_path=/"
			if f.pathInfo {
				apiURL = "/api/"
			}
			if f.omitAPIURL {
				fmt.Fprint(w, "<html></html>")
				return
			}
			fmt.Fprintf(w, `<html><script>var opts = {"token":"x","apiUrl":%q};</script></html>`, apiURL)
			return
		}
		switch route {
		case "/session":
			if f.password == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{"disabled": 1})
				return
			}
			f.token = "session-token"
			_ = json.NewEncoder(w).Encode(map[string]any{"token": f.token, "session": "sid"})
		case "/login":
			if r.Header.Get("MTT-Token") != f.token {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			var body struct {
				Password string `json:"password"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Password != f.password {
				_ = json.NewEncoder(w).Encode(map[string]any{"logged": 0})
				return
			}
			f.token = "logged-token"
			f.loggedIn = true
			_ = json.NewEncoder(w).Encode(map[string]any{"logged": 1, "token": f.token})
		case "/tasks":
			expected := f.token
			if f.password == "" {
				expected = "anon-token"
			}
			if r.Header.Get("MTT-Token") != expected {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, "Access denied! No token provided.")
				return
			}
			f.lastQuery = map[string]string{
				"list":  r.URL.Query().Get("list"),
				"compl": r.URL.Query().Get("compl"),
				"t":     r.URL.Query().Get("t"),
			}
			fmt.Fprint(w, f.tasksJSON)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

const sampleTasks = `{"total":3,"list":[
  {"id":1,"titleText":"pay rent","dueInt":20260705},
  {"id":2,"titleText":"no due","dueInt":33330000},
  {"id":3,"titleText":"old one","dueInt":20260620}
]}`

func TestLoginAndTasksWithPassword(t *testing.T) {
	fake := &fakeServer{password: "secret", tasksJSON: sampleTasks}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	c, err := NewClient(srv.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("login: %v", err)
	}
	if !fake.loggedIn {
		t.Fatal("server did not register a login")
	}

	tasks, err := c.Tasks(context.Background(), -1, "ongoing")
	if err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if fake.lastQuery["list"] != "-1" || fake.lastQuery["compl"] != "0" || fake.lastQuery["t"] != "ongoing" {
		t.Errorf("query = %v", fake.lastQuery)
	}
	if len(tasks) != 3 {
		t.Fatalf("got %d tasks, want 3", len(tasks))
	}
	want := time.Date(2026, 7, 5, 0, 0, 0, 0, time.Local)
	if !tasks[0].Due.Equal(want) {
		t.Errorf("task due = %v, want %v", tasks[0].Due, want)
	}
	if tasks[1].HasDue() {
		t.Errorf("task without due date parsed as %v", tasks[1].Due)
	}
	if tasks[0].Title != "pay rent" {
		t.Errorf("title = %q", tasks[0].Title)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	fake := &fakeServer{password: "secret"}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	c, err := NewClient(srv.URL, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login(context.Background()); err == nil {
		t.Fatal("expected login error")
	}
}

func TestAnonymousLogin(t *testing.T) {
	fake := &fakeServer{tasksJSON: `{"total":0,"list":[]}`}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	c, err := NewClient(srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := c.Tasks(context.Background(), -1, ""); err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if fake.lastQuery["t"] != "" {
		t.Errorf("unexpected tag filter %q", fake.lastQuery["t"])
	}
}

func TestPathInfoAPIStyle(t *testing.T) {
	// Server set up with MTT_API_USE_PATH_INFO: the index page advertises
	// /api/ and api.php?_path= requests fail with HTTP 500.
	fake := &fakeServer{password: "secret", tasksJSON: sampleTasks, pathInfo: true}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	c, err := NewClient(srv.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("login: %v", err)
	}
	tasks, err := c.Tasks(context.Background(), 3, "ongoing")
	if err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("got %d tasks, want 3", len(tasks))
	}
	if fake.lastQuery["list"] != "3" || fake.lastQuery["t"] != "ongoing" {
		t.Errorf("query = %v", fake.lastQuery)
	}
}

func TestIndexWithoutAPIURL(t *testing.T) {
	// An index page without an apiUrl entry keeps the default
	// api.php?_path=/ style working.
	fake := &fakeServer{password: "secret", tasksJSON: sampleTasks, omitAPIURL: true}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	c, err := NewClient(srv.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := c.Tasks(context.Background(), -1, ""); err != nil {
		t.Fatalf("tasks: %v", err)
	}
}

func TestTasksDenied(t *testing.T) {
	fake := &fakeServer{password: "secret", tasksJSON: `{"total":0,"list":[],"denied":1}`}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	c, err := NewClient(srv.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Tasks(context.Background(), -1, ""); err == nil {
		t.Fatal("expected denied error")
	}
}

func TestBasicAuth(t *testing.T) {
	fake := &fakeServer{
		password:  "secret",
		tasksJSON: `{"total":0,"list":[]}`,
		basicUser: "webuser",
		basicPass: "webpass",
	}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	t.Run("credentials sent on every request", func(t *testing.T) {
		c, err := NewClient(srv.URL, "secret")
		if err != nil {
			t.Fatal(err)
		}
		c.SetBasicAuth("webuser", "webpass")
		if err := c.Login(context.Background()); err != nil {
			t.Fatalf("login: %v", err)
		}
		if _, err := c.Tasks(context.Background(), -1, ""); err != nil {
			t.Fatalf("tasks: %v", err)
		}
	})

	t.Run("missing credentials rejected", func(t *testing.T) {
		c, err := NewClient(srv.URL, "secret")
		if err != nil {
			t.Fatal(err)
		}
		err = c.Login(context.Background())
		if err == nil || !strings.Contains(err.Error(), "401") {
			t.Fatalf("error = %v, want HTTP 401", err)
		}
	})

	t.Run("wrong credentials rejected", func(t *testing.T) {
		c, err := NewClient(srv.URL, "secret")
		if err != nil {
			t.Fatal(err)
		}
		c.SetBasicAuth("webuser", "nope")
		err = c.Login(context.Background())
		if err == nil || !strings.Contains(err.Error(), "401") {
			t.Fatalf("error = %v, want HTTP 401", err)
		}
	})
}

func TestBasicAuthAnonymousIndexFetch(t *testing.T) {
	// Password-less install behind basic auth: the index-page fetch that
	// obtains the mtt-token cookie must carry the credentials too.
	fake := &fakeServer{
		tasksJSON: `{"total":0,"list":[]}`,
		basicUser: "webuser",
		basicPass: "webpass",
	}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	c, err := NewClient(srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	c.SetBasicAuth("webuser", "webpass")
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := c.Tasks(context.Background(), -1, ""); err != nil {
		t.Fatalf("tasks: %v", err)
	}
}

func TestNewClientRejectsBadURL(t *testing.T) {
	if _, err := NewClient("todo.example.com", ""); err == nil {
		t.Fatal("expected error for url without scheme")
	}
}

func TestDueIntToTime(t *testing.T) {
	tests := []struct {
		in   int
		want time.Time
	}{
		{20260702, time.Date(2026, 7, 2, 0, 0, 0, 0, time.Local)},
		{noDueDate, time.Time{}},
		{0, time.Time{}},
	}
	for _, tt := range tests {
		if got := dueIntToTime(tt.in); !got.Equal(tt.want) {
			t.Errorf("dueIntToTime(%d) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
