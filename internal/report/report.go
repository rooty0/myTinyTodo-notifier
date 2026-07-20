// Package report selects the tasks worth notifying about and renders
// them into a message using a text/template.
package report

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/rooty0/myTinyTodo-notifier/internal/mtt"
)

//go:embed templates/message.tmpl
var defaultTemplates embed.FS

// Data is the input handed to the message template.
type Data struct {
	Ongoing []Item
	Due     []Item
}

// Empty reports whether there is nothing to notify about.
func (d Data) Empty() bool { return len(d.Ongoing) == 0 && len(d.Due) == 0 }

// Item is a single task as seen by the template.
type Item struct {
	Title string
	// Due is set only for items of the Due section.
	Due      time.Time
	Overdue  bool
	DueToday bool
}

// Build classifies tasks for the notification. All ongoing tasks are kept
// as-is; due tasks are kept when their due date falls on or before
// now+thresholdDays (overdue tasks included, tasks without a due date skipped).
func Build(ongoing, due []mtt.Task, now time.Time, thresholdDays int) Data {
	var data Data
	for _, t := range ongoing {
		data.Ongoing = append(data.Ongoing, Item{Title: t.Title})
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := today.AddDate(0, 0, thresholdDays)
	for _, t := range due {
		if !t.HasDue() || t.Due.After(cutoff) {
			continue
		}
		data.Due = append(data.Due, Item{
			Title:    t.Title,
			Due:      t.Due,
			Overdue:  t.Due.Before(today),
			DueToday: t.Due.Equal(today),
		})
	}
	return data
}

// LoadTemplate parses the message template from path, or the embedded
// default when path is empty.
func LoadTemplate(path string) (*template.Template, error) {
	if path == "" {
		tmpl, err := template.ParseFS(defaultTemplates, "templates/message.tmpl")
		if err != nil {
			return nil, fmt.Errorf("parse embedded template: %w", err)
		}
		return tmpl, nil
	}
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}
	return tmpl, nil
}

// Render executes the template and returns the notification message.
func Render(tmpl *template.Template, data Data) (string, error) {
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render message: %w", err)
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}
