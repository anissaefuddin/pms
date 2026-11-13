// Package automation holds the WHEN/IF/THEN rules (AUT-02) and the engine that runs them (AUT-03). The rule language is closed:
// every trigger, condition and action is a fixed shape validated here, so a stored rule can never ask for anything the engine
// does not know how to do safely.
package automation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/events"
	"github.com/google/uuid"
)

const (
	MaxConditions = 10
	MaxActions    = 10
	MaxDueDays    = 30
)

type Trigger struct {
	Type       string `json:"type"`
	ToStatusID string `json:"to_status_id,omitempty"` // item.status_changed
	UserID     string `json:"user_id,omitempty"`      // item.assigned: only this user
	ItemType   string `json:"item_type,omitempty"`    // item.created: only this item type key
	DaysBefore *int   `json:"days_before,omitempty"`  // item.due_reached
	FormID     string `json:"form_id,omitempty"`      // form.submitted
}

type Condition struct {
	Field string          `json:"field"`
	Op    string          `json:"op"`
	Value json.RawMessage `json:"value,omitempty"`
}

type Action struct {
	Type      string          `json:"type"`
	Field     string          `json:"field,omitempty"`     // set_field
	Value     json.RawMessage `json:"value,omitempty"`     // set_field
	UserIDs   []string        `json:"user_ids,omitempty"`  // assign, notify
	Assignees bool            `json:"assignees,omitempty"` // notify
	Text      string          `json:"text,omitempty"`      // comment
	Message   string          `json:"message,omitempty"`   // notify
	Title     string          `json:"title,omitempty"`     // create_item
	TypeKey   string          `json:"type_key,omitempty"`  // create_item
	Kind      string          `json:"kind,omitempty"`      // create_item built-in class when no type_key: task, project, folder
}

type Definition struct {
	When Trigger     `json:"when"`
	If   []Condition `json:"if"`
	Then []Action    `json:"then"`
}

var (
	setFieldNames = []string{"importance", "due_date", "start_date", "effort_minutes", "duration_days", "status_id"}
	customField   = regexp.MustCompile(`^fields\.[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	condFields    = []string{"importance", "status_group", "item_type", "assignee", "due_date", "start_date"}
)

// ParseDefinition reads a definition strictly: unknown keys are an error, so a typo cannot silently disable a rule.
func ParseDefinition(raw []byte) (Definition, error) {
	var d Definition
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, fmt.Errorf("invalid rule: %v", err)
	}
	return d, d.Validate()
}

func isUUID(s string) bool { _, err := uuid.Parse(s); return err == nil }

// Validate checks a definition without touching the database. References to statuses, users and fields are checked by the caller.
func (d Definition) Validate() error {
	t := d.When
	switch t.Type {
	case events.ItemStatusChanged:
		if t.ToStatusID != "" && !isUUID(t.ToStatusID) {
			return fmt.Errorf("when.to_status_id must be a status id")
		}
	case events.ItemAssigned:
		if t.UserID != "" && !isUUID(t.UserID) {
			return fmt.Errorf("when.user_id must be a user id")
		}
	case events.ItemCreated:
		if t.ItemType != "" && !regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`).MatchString(t.ItemType) {
			return fmt.Errorf("when.item_type must be an item type key")
		}
	case events.ItemDueReached:
		if t.DaysBefore == nil || *t.DaysBefore < 0 || *t.DaysBefore > MaxDueDays {
			return fmt.Errorf("when.days_before must be 0 to %d", MaxDueDays)
		}
	case events.FormSubmitted:
		if t.FormID != "" && !isUUID(t.FormID) {
			return fmt.Errorf("when.form_id must be a form id")
		}
	default:
		return fmt.Errorf("when.type must be one of item.status_changed, item.assigned, item.created, item.due_reached, form.submitted")
	}
	if t.Type != events.ItemStatusChanged && t.ToStatusID != "" || t.Type != events.ItemAssigned && t.UserID != "" ||
		t.Type != events.ItemCreated && t.ItemType != "" || t.Type != events.ItemDueReached && t.DaysBefore != nil || t.Type != events.FormSubmitted && t.FormID != "" {
		return fmt.Errorf("when has a setting that does not belong to %s", t.Type)
	}
	if len(d.If) > MaxConditions {
		return fmt.Errorf("at most %d conditions", MaxConditions)
	}
	for i, c := range d.If {
		if err := c.validate(); err != nil {
			return fmt.Errorf("if[%d]: %v", i, err)
		}
	}
	if len(d.Then) < 1 || len(d.Then) > MaxActions {
		return fmt.Errorf("a rule needs 1 to %d actions", MaxActions)
	}
	for i, a := range d.Then {
		if err := a.validate(); err != nil {
			return fmt.Errorf("then[%d]: %v", i, err)
		}
	}
	return nil
}

func jsonString(raw json.RawMessage) (string, bool) {
	var s string
	return s, json.Unmarshal(raw, &s) == nil
}

func (c Condition) validate() error {
	isField := customField.MatchString(c.Field)
	if !slices.Contains(condFields, c.Field) && !isField {
		return fmt.Errorf("field must be one of %s or fields.<custom field id>", strings.Join(condFields, ", "))
	}
	needsValue := func() error {
		if len(c.Value) == 0 || string(c.Value) == "null" {
			return fmt.Errorf("%s needs a value", c.Op)
		}
		return nil
	}
	oneOf := func(ops ...string) error {
		if !slices.Contains(ops, c.Op) {
			return fmt.Errorf("%s accepts %s", c.Field, strings.Join(ops, ", "))
		}
		return nil
	}
	switch {
	case c.Field == "importance":
		if err := oneOf("eq", "neq"); err != nil {
			return err
		}
		if v, ok := jsonString(c.Value); !ok || !slices.Contains([]string{"high", "normal", "low"}, v) {
			return fmt.Errorf("importance must be high, normal or low")
		}
	case c.Field == "status_group":
		if err := oneOf("eq", "neq"); err != nil {
			return err
		}
		if v, ok := jsonString(c.Value); !ok || !slices.Contains([]string{"active", "completed", "deferred", "cancelled"}, v) {
			return fmt.Errorf("status_group must be active, completed, deferred or cancelled")
		}
	case c.Field == "item_type":
		if err := oneOf("eq", "neq"); err != nil {
			return err
		}
		if v, ok := jsonString(c.Value); !ok || !regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`).MatchString(v) {
			return fmt.Errorf("item_type must be an item type key")
		}
	case c.Field == "assignee":
		if err := oneOf("has", "has_not", "empty", "not_empty"); err != nil {
			return err
		}
		if c.Op == "has" || c.Op == "has_not" {
			if v, ok := jsonString(c.Value); !ok || !isUUID(v) {
				return fmt.Errorf("assignee needs a user id")
			}
		} else if len(c.Value) != 0 {
			return fmt.Errorf("%s takes no value", c.Op)
		}
	case c.Field == "due_date" || c.Field == "start_date":
		if err := oneOf("set", "unset", "before", "after"); err != nil {
			return err
		}
		if c.Op == "before" || c.Op == "after" {
			if v, ok := jsonString(c.Value); !ok || (v != "today" && !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(v)) {
				return fmt.Errorf("give a date as YYYY-MM-DD or today")
			}
		} else if len(c.Value) != 0 {
			return fmt.Errorf("%s takes no value", c.Op)
		}
	default: // a custom field
		if err := oneOf("eq", "neq", "gt", "lt", "set", "unset"); err != nil {
			return err
		}
		if c.Op == "set" || c.Op == "unset" {
			if len(c.Value) != 0 {
				return fmt.Errorf("%s takes no value", c.Op)
			}
		} else if err := needsValue(); err != nil {
			return err
		}
	}
	return nil
}

func (a Action) validate() error {
	text := func(label, s string, max int) error {
		if n := utf8.RuneCountInString(strings.TrimSpace(s)); n < 1 || utf8.RuneCountInString(s) > max || strings.ContainsRune(s, 0) {
			return fmt.Errorf("%s must be 1 to %d characters", label, max)
		}
		return nil
	}
	users := func(ids []string) error {
		if len(ids) > 20 {
			return fmt.Errorf("at most 20 users")
		}
		for _, id := range ids {
			if !isUUID(id) {
				return fmt.Errorf("user_ids must be user ids")
			}
		}
		return nil
	}
	unused := func(cond bool, what string) error {
		if cond {
			return fmt.Errorf("%s does not belong to %s", what, a.Type)
		}
		return nil
	}
	switch a.Type {
	case "set_field":
		if !slices.Contains(setFieldNames, a.Field) && !customField.MatchString(a.Field) {
			return fmt.Errorf("field must be one of %s or fields.<custom field id>", strings.Join(setFieldNames, ", "))
		}
		if len(a.Value) == 0 {
			return fmt.Errorf("set_field needs a value (null clears it)")
		}
		return unused(len(a.UserIDs) > 0 || a.Text != "" || a.Title != "" || a.Message != "" || a.Assignees, "that setting")
	case "assign":
		if len(a.UserIDs) < 1 {
			return fmt.Errorf("assign needs at least one user")
		}
		if err := users(a.UserIDs); err != nil {
			return err
		}
		return unused(a.Field != "" || len(a.Value) > 0 || a.Text != "" || a.Title != "" || a.Message != "" || a.Assignees, "that setting")
	case "comment":
		if err := text("text", a.Text, 2000); err != nil {
			return err
		}
		return unused(a.Field != "" || len(a.Value) > 0 || len(a.UserIDs) > 0 || a.Title != "" || a.Message != "" || a.Assignees, "that setting")
	case "notify":
		if len(a.UserIDs) == 0 && !a.Assignees {
			return fmt.Errorf("notify needs user_ids or assignees")
		}
		if err := users(a.UserIDs); err != nil {
			return err
		}
		if err := text("message", a.Message, 500); err != nil {
			return err
		}
		return unused(a.Field != "" || len(a.Value) > 0 || a.Text != "" || a.Title != "", "that setting")
	case "create_item":
		if err := text("title", a.Title, 500); err != nil {
			return err
		}
		if a.TypeKey != "" && !regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`).MatchString(a.TypeKey) {
			return fmt.Errorf("type_key must be an item type key")
		}
		if a.Kind != "" && !slices.Contains([]string{"task", "project", "folder"}, a.Kind) {
			return fmt.Errorf("kind must be task, project or folder")
		}
		return unused(a.Field != "" || len(a.Value) > 0 || len(a.UserIDs) > 0 || a.Text != "" || a.Message != "" || a.Assignees, "that setting")
	}
	return fmt.Errorf("type must be one of set_field, assign, comment, notify, create_item")
}
