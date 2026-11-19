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
	MaxDueDays    = 30 // days before (or, negative, after) the due date
)

type Trigger struct {
	Type       string `json:"type"`
	ToStatusID string `json:"to_status_id,omitempty"` // item.status_changed
	UserID     string `json:"user_id,omitempty"`      // item.assigned: only this user
	ItemType   string `json:"item_type,omitempty"`    // item.created: only this item type key
	DaysBefore *int   `json:"days_before,omitempty"`  // item.due_reached: negative = overdue by that many days
	FormID     string `json:"form_id,omitempty"`      // form.submitted
	Decision   string `json:"decision,omitempty"`     // approval.decided: approved or rejected
	Days       *int   `json:"days,omitempty"`         // item.inactive: no activity for this many days (1 to 365)
	Calendar   string `json:"calendar,omitempty"`     // item.due_reached: "work" counts working days of the instance schedule
}

type Condition struct {
	Field string          `json:"field"`
	Op    string          `json:"op"`
	Value json.RawMessage `json:"value,omitempty"`
}

type Action struct {
	Type      string          `json:"type"`
	Field     string          `json:"field,omitempty"`        // set_field
	Value     json.RawMessage `json:"value,omitempty"`        // set_field
	UserIDs   []string        `json:"user_ids,omitempty"`     // assign, notify
	Assignees bool            `json:"assignees,omitempty"`    // notify
	Text      string          `json:"text,omitempty"`         // comment
	Message   string          `json:"message,omitempty"`      // notify
	Title     string          `json:"title,omitempty"`        // create_item
	TypeKey   string          `json:"type_key,omitempty"`     // create_item
	Kind      string          `json:"kind,omitempty"`         // create_item built-in class when no type_key: task, project, folder
	Under     string          `json:"under,omitempty"`        // create_item: "item" makes the new item a subitem of the item that triggered the rule
	Container string          `json:"container_id,omitempty"` // move: the project or folder in this Space to move the item into
	Followers bool            `json:"followers,omitempty"`    // mention: everyone following the item at run time
	GroupIDs  []string        `json:"group_ids,omitempty"`    // mention: every member of these groups at run time
}

type Definition struct {
	When Trigger     `json:"when"`
	If   []Condition `json:"if"`
	Then []Action    `json:"then"`
}

var (
	setFieldNames = []string{"importance", "due_date", "start_date", "effort_minutes", "duration_days", "status_id", "status_group"}
	customField   = regexp.MustCompile(`^fields\.[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	condFields    = []string{"importance", "status_group", "item_type", "assignee", "due_date", "start_date"}
)

// statusCategories are the workspace-level status categories a workspace rule may name (decision D1): the same groups every workflow has.
var statusCategories = []string{"active", "completed", "deferred", "cancelled"}

// validateForScope checks what only a scope can decide. A Space rule names statuses by id; a workspace rule names a category, so it
// works in every Space whatever its statuses are called.
func validateForScope(d Definition, scope string) error {
	for i, a := range d.Then {
		if a.Type != "set_field" {
			continue
		}
		switch {
		case scope == "workspace" && a.Field == "status_id":
			return fmt.Errorf("then[%d]: a workspace rule names a status category (status_group), not a status id", i)
		case scope != "workspace" && a.Field == "status_group":
			return fmt.Errorf("then[%d]: status_group is for workspace rules; a Space rule names a status id", i)
		case a.Field == "status_group":
			var v string
			if json.Unmarshal(a.Value, &v) != nil || !slices.Contains(statusCategories, v) {
				return fmt.Errorf("then[%d]: status_group must be active, completed, deferred or cancelled", i)
			}
		}
	}
	if scope == "workspace" && d.When.ToStatusID != "" {
		return fmt.Errorf("when.to_status_id names a status of one Space; a workspace rule uses no status trigger")
	}
	return nil
}

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
		if t.DaysBefore == nil || *t.DaysBefore < -MaxDueDays || *t.DaysBefore > MaxDueDays {
			return fmt.Errorf("when.days_before must be -%d to %d (negative means overdue)", MaxDueDays, MaxDueDays)
		}
		if t.Calendar != "" && t.Calendar != "work" {
			return fmt.Errorf("when.calendar must be work or empty")
		}
	case events.FormSubmitted:
		if t.FormID != "" && !isUUID(t.FormID) {
			return fmt.Errorf("when.form_id must be a form id")
		}
	case events.ApprovalDecided:
		if t.Decision != "" && t.Decision != "approved" && t.Decision != "rejected" {
			return fmt.Errorf("when.decision must be approved or rejected")
		}
	case events.CommentCreated:
	case events.ItemInactive:
		if t.Days == nil || *t.Days < 1 || *t.Days > 365 {
			return fmt.Errorf("when.days must be 1 to 365")
		}
	default:
		return fmt.Errorf("when.type must be one of item.status_changed, item.assigned, item.created, item.due_reached, form.submitted, approval.decided, comment.created")
	}
	if t.Type != events.ItemStatusChanged && t.ToStatusID != "" || t.Type != events.ItemAssigned && t.UserID != "" ||
		t.Type != events.ItemCreated && t.ItemType != "" || t.Type != events.ItemDueReached && t.DaysBefore != nil || t.Type != events.FormSubmitted && t.FormID != "" ||
		t.Type != events.ApprovalDecided && t.Decision != "" || t.Type != events.ItemInactive && t.Days != nil ||
		t.Type != events.ItemDueReached && t.Calendar != "" {
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

// anyOf checks an "is one of" value: a non-empty list (at most 10) of the allowed values.
func anyOf(raw json.RawMessage, allowed []string, message string) error {
	var list []string
	if json.Unmarshal(raw, &list) != nil || len(list) == 0 || len(list) > 10 {
		return fmt.Errorf("%s; give 1 to 10 values", "a list is needed: "+message)
	}
	for _, v := range list {
		if !slices.Contains(allowed, v) {
			return fmt.Errorf("%s", message)
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
		if err := oneOf("eq", "neq", "in"); err != nil {
			return err
		}
		if c.Op == "in" {
			return anyOf(c.Value, []string{"high", "normal", "low"}, "importance must be high, normal or low")
		}
		if v, ok := jsonString(c.Value); !ok || !slices.Contains([]string{"high", "normal", "low"}, v) {
			return fmt.Errorf("importance must be high, normal or low")
		}
	case c.Field == "status_group":
		if err := oneOf("eq", "neq", "in"); err != nil {
			return err
		}
		if c.Op == "in" {
			return anyOf(c.Value, []string{"active", "completed", "deferred", "cancelled"}, "status_group must be active, completed, deferred or cancelled")
		}
		if v, ok := jsonString(c.Value); !ok || !slices.Contains([]string{"active", "completed", "deferred", "cancelled"}, v) {
			return fmt.Errorf("status_group must be active, completed, deferred or cancelled")
		}
	case c.Field == "item_type":
		if err := oneOf("eq", "neq", "in"); err != nil {
			return err
		}
		if c.Op == "in" {
			var list []string
			if json.Unmarshal(c.Value, &list) != nil || len(list) == 0 || len(list) > 10 {
				return fmt.Errorf("item_type needs 1 to 10 type keys")
			}
			for _, v := range list {
				if !regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`).MatchString(v) {
					return fmt.Errorf("item_type must be an item type key")
				}
			}
			return nil
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
		return unused(len(a.UserIDs) > 0 || a.Text != "" || a.Title != "" || a.Message != "" || a.Assignees || a.Under != "" || a.Container != "", "that setting")
	case "assign":
		if len(a.UserIDs) < 1 {
			return fmt.Errorf("assign needs at least one user")
		}
		if err := users(a.UserIDs); err != nil {
			return err
		}
		return unused(a.Field != "" || len(a.Value) > 0 || a.Text != "" || a.Title != "" || a.Message != "" || a.Assignees || a.Under != "" || a.Container != "", "that setting")
	case "comment":
		if err := text("text", a.Text, 2000); err != nil {
			return err
		}
		return unused(a.Field != "" || len(a.Value) > 0 || len(a.UserIDs) > 0 || a.Title != "" || a.Message != "" || a.Assignees || a.Under != "" || a.Container != "", "that setting")
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
		return unused(a.Field != "" || len(a.Value) > 0 || a.Text != "" || a.Title != "" || a.Under != "" || a.Container != "", "that setting")
	case "mention":
		if err := text("text", a.Text, 2000); err != nil {
			return err
		}
		if len(a.UserIDs)+len(a.GroupIDs) == 0 && !a.Followers {
			return fmt.Errorf("mention needs a user, a group, or followers")
		}
		if err := users(a.UserIDs); err != nil {
			return err
		}
		if len(a.GroupIDs) > 5 {
			return fmt.Errorf("at most 5 groups")
		}
		for _, id := range a.GroupIDs {
			if !isUUID(id) {
				return fmt.Errorf("group_ids must be group ids")
			}
		}
		return unused(a.Field != "" || len(a.Value) > 0 || a.Title != "" || a.Message != "" || a.Assignees || a.Under != "" || a.Kind != "" || a.TypeKey != "" || a.Container != "", "that setting")
	case "move":
		if !isUUID(a.Container) {
			return fmt.Errorf("move needs container_id, a project or folder in this Space")
		}
		return unused(a.Field != "" || len(a.Value) > 0 || len(a.UserIDs) > 0 || a.Text != "" || a.Title != "" || a.Message != "" || a.Assignees || a.Under != "" || a.Kind != "" || a.TypeKey != "", "that setting")
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
		if a.Under != "" && a.Under != "item" {
			return fmt.Errorf("under must be item")
		}
		return unused(a.Field != "" || len(a.Value) > 0 || len(a.UserIDs) > 0 || a.Text != "" || a.Message != "" || a.Assignees || a.Container != "", "that setting")
	}
	return fmt.Errorf("type must be one of set_field, assign, comment, notify, create_item, mention, move")
}
