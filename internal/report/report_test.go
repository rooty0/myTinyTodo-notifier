package report

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rooty0/myTinyTodo-notifier/internal/mtt"
)

var now = time.Date(2026, 7, 2, 15, 0, 0, 0, time.Local)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func TestBuild(t *testing.T) {
	ongoing := []mtt.Task{{Title: "keep going"}}
	due := []mtt.Task{
		{Title: "expired", Due: day(2026, 6, 20)},
		{Title: "today", Due: day(2026, 7, 2)},
		{Title: "soon", Due: day(2026, 7, 8)},
		{Title: "at threshold", Due: day(2026, 7, 9)},
		{Title: "too far", Due: day(2026, 7, 10)},
		{Title: "no due date"},
	}

	data := Build(ongoing, due, now, 7)

	if len(data.Ongoing) != 1 || data.Ongoing[0].Title != "keep going" {
		t.Errorf("ongoing = %+v", data.Ongoing)
	}
	if len(data.Due) != 4 {
		t.Fatalf("got %d due items, want 4: %+v", len(data.Due), data.Due)
	}
	byTitle := map[string]Item{}
	for _, item := range data.Due {
		byTitle[item.Title] = item
	}
	if item := byTitle["expired"]; !item.Overdue || item.DueToday {
		t.Errorf("expired item = %+v", item)
	}
	if item := byTitle["today"]; item.Overdue || !item.DueToday {
		t.Errorf("today item = %+v", item)
	}
	if item := byTitle["soon"]; item.Overdue || item.DueToday {
		t.Errorf("soon item = %+v", item)
	}
	if _, ok := byTitle["too far"]; ok {
		t.Error("task beyond threshold was included")
	}
}

func TestBuildEmpty(t *testing.T) {
	data := Build(nil, nil, now, 7)
	if !data.Empty() {
		t.Errorf("expected empty data, got %+v", data)
	}
}

func TestRenderDefaultTemplate(t *testing.T) {
	tmpl, err := LoadTemplate("")
	if err != nil {
		t.Fatal(err)
	}
	data := Data{
		Ongoing: []Item{{Title: "keep going"}},
		Due: []Item{
			{Title: "expired", Due: day(2026, 6, 20), Overdue: true},
			{Title: "today", Due: day(2026, 7, 2), DueToday: true},
			{Title: "soon", Due: day(2026, 7, 8)},
		},
	}
	got, err := Render(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	want := `Hi,

=== Ongoing tasks: ===
- keep going
=== Due date: ===
- "expired" EXPIRED on Saturday, June 20 (06/20)
- "today" DUE TODAY on Thursday, July 2 (07/02)
- "soon" due on Wednesday, July 8 (07/08)`
	if got != want {
		t.Errorf("rendered message:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderSkipsEmptySections(t *testing.T) {
	tmpl, err := LoadTemplate("")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Render(tmpl, Data{Ongoing: []Item{{Title: "solo"}}})
	if err != nil {
		t.Fatal(err)
	}
	want := `Hi,

=== Ongoing tasks: ===
- solo`
	if got != want {
		t.Errorf("rendered message:\n%s\nwant:\n%s", got, want)
	}
}

func TestLoadTemplateCustomFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "message.tmpl")
	if err := os.WriteFile(path, []byte("{{ len .Ongoing }} ongoing"), 0o600); err != nil {
		t.Fatal(err)
	}
	tmpl, err := LoadTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Render(tmpl, Data{Ongoing: []Item{{Title: "a"}, {Title: "b"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "2 ongoing" {
		t.Errorf("got %q", got)
	}
}

func TestLoadTemplateMissingFile(t *testing.T) {
	if _, err := LoadTemplate(filepath.Join(t.TempDir(), "nope.tmpl")); err == nil {
		t.Fatal("expected error for missing template file")
	}
}
