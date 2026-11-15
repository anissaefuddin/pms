// Package spacetemplates holds the built-in Space templates (SPC-18): JSON files shipped in the binary.
// A template describes what a new Space starts with; creating it is done by the HTTP layer in one transaction.
package spacetemplates

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/anissaefuddin/pms/server/internal/forms"
)

// Format is the template file format this server understands. A template with a higher format was written for a
// newer server and is listed as incompatible instead of being half applied. Version is the template's own revision.
const Format = 1

//go:embed templates/*.json
var files embed.FS

type Option struct {
	Label string `json:"label"`
	Color string `json:"color"`
}

type Field struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"` // single_select or multi_select
	Options []Option `json:"options"`
}

type Task struct {
	Title        string `json:"title"`
	Importance   string `json:"importance,omitempty"`
	DurationDays *int   `json:"duration_days,omitempty"`
}

type Project struct {
	Title       string `json:"title"`
	DefaultView string `json:"default_view,omitempty"` // table, board, gantt, calendar, files or chart
	Tasks       []Task `json:"tasks"`
}

type View struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Config map[string]any `json:"config,omitempty"`
}

// Form saves its requests in the Project at Projects[Project], or in the Space itself when Project is -1.
type Form struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Project     int              `json:"project"`
	Definition  forms.Definition `json:"definition"`
	TitleKey    string           `json:"title_key"` // the question whose answer becomes the item title
}

type Template struct {
	Format       int       `json:"format"`
	Version      int       `json:"version"`
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Category     string    `json:"category"`
	Description  string    `json:"description"`
	Workflow     string    `json:"workflow,omitempty"` // name of an existing workflow for the projects; unknown names fall back to the default one
	CustomFields []Field   `json:"custom_fields"`
	Projects     []Project `json:"projects"`
	Views        []View    `json:"views"`
	Forms        []Form    `json:"forms"`
}

// Summary is what the picker shows.
type Summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Version     int    `json:"version"`
	Compatible  bool   `json:"compatible"`
	Counts      struct {
		Projects     int `json:"projects"`
		Tasks        int `json:"tasks"`
		Views        int `json:"views"`
		Forms        int `json:"forms"`
		CustomFields int `json:"custom_fields"`
	} `json:"counts"`
}

func (t Template) Summary() Summary {
	s := Summary{ID: t.ID, Name: t.Name, Category: t.Category, Description: t.Description, Version: t.Version, Compatible: t.Format <= Format}
	s.Counts.Projects, s.Counts.Views, s.Counts.Forms, s.Counts.CustomFields = len(t.Projects), len(t.Views), len(t.Forms), len(t.CustomFields)
	for _, p := range t.Projects {
		s.Counts.Tasks += len(p.Tasks)
	}
	return s
}

var all = mustLoad()

func mustLoad() []Template {
	entries, err := files.ReadDir("templates")
	if err != nil {
		panic(err)
	}
	var out []Template
	for _, entry := range entries {
		raw, err := files.ReadFile("templates/" + entry.Name())
		if err != nil {
			panic(err)
		}
		var t Template
		if err := json.Unmarshal(raw, &t); err != nil {
			panic(fmt.Sprintf("template %s: %v", entry.Name(), err))
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Category+out[i].Name < out[j].Category+out[j].Name })
	return out
}

func All() []Template { return all }

func Find(id string) (Template, bool) {
	for _, t := range all {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}
