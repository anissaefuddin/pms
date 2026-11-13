package automation

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/events"
)

// Facts are the values of the item a rule looks at, as they are when the rule runs (not when the event happened).
type Facts struct {
	Importance  string
	StatusGroup string
	ItemType    string
	Assignees   []string
	DueDate     string // YYYY-MM-DD or empty
	StartDate   string
	Fields      map[string]json.RawMessage
}

// Holds evaluates one condition. A malformed or unknown condition never holds, so a bad rule does nothing instead of everything.
func (c Condition) Holds(f Facts, today string) bool {
	str := func() string { s, _ := jsonString(c.Value); return s }
	switch c.Field {
	case "importance":
		return (f.Importance == str()) == (c.Op == "eq") && (c.Op == "eq" || c.Op == "neq")
	case "status_group":
		return (f.StatusGroup == str()) == (c.Op == "eq") && (c.Op == "eq" || c.Op == "neq")
	case "item_type":
		return (f.ItemType == str()) == (c.Op == "eq") && (c.Op == "eq" || c.Op == "neq")
	case "assignee":
		switch c.Op {
		case "has":
			return slices.Contains(f.Assignees, str())
		case "has_not":
			return !slices.Contains(f.Assignees, str())
		case "empty":
			return len(f.Assignees) == 0
		case "not_empty":
			return len(f.Assignees) > 0
		}
	case "due_date", "start_date":
		date := f.DueDate
		if c.Field == "start_date" {
			date = f.StartDate
		}
		switch c.Op {
		case "set":
			return date != ""
		case "unset":
			return date == ""
		case "before", "after":
			limit := str()
			if limit == "today" {
				limit = today
			}
			if date == "" {
				return false
			}
			if c.Op == "before" {
				return date < limit
			}
			return date > limit
		}
	default:
		raw, present := f.Fields[strings.TrimPrefix(c.Field, "fields.")]
		present = present && string(raw) != "null"
		switch c.Op {
		case "set":
			return present
		case "unset":
			return !present
		case "eq", "neq":
			return (present && sameValue(raw, c.Value)) == (c.Op == "eq")
		case "gt", "lt":
			var a, b float64
			if !present || json.Unmarshal(raw, &a) != nil || json.Unmarshal(c.Value, &b) != nil {
				return false
			}
			return (c.Op == "gt" && a > b) || (c.Op == "lt" && a < b)
		}
	}
	return false
}

func sameValue(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	nx, _ := json.Marshal(x)
	ny, _ := json.Marshal(y)
	return string(nx) == string(ny)
}

// AllHold is true when every condition holds (an empty list always holds).
func (d Definition) AllHold(f Facts, today string) bool {
	for _, c := range d.If {
		if !c.Holds(f, today) {
			return false
		}
	}
	return true
}

// Matches says whether an event is what the rule's trigger waits for. facts supply the item type for item.created.
func (t Trigger) Matches(e events.Event, f Facts) bool {
	if t.Type != e.Type {
		return false
	}
	switch t.Type {
	case events.ItemStatusChanged:
		to, _ := e.Payload["to"].(string)
		return t.ToStatusID == "" || t.ToStatusID == to
	case events.ItemAssigned:
		ids, _ := e.Payload["user_ids"].([]any)
		if t.UserID == "" {
			return true
		}
		for _, id := range ids {
			if id == t.UserID {
				return true
			}
		}
		return false
	case events.ItemCreated:
		return t.ItemType == "" || t.ItemType == f.ItemType
	case events.ItemDueReached:
		days, _ := e.Payload["days_before"].(float64)
		return t.DaysBefore != nil && float64(*t.DaysBefore) == days
	case events.FormSubmitted:
		form, _ := e.Payload["form_id"].(string)
		return t.FormID == "" || t.FormID == form
	}
	return false
}
