package automation

import "encoding/json"

// Template is a starting point a Space editor can take into the builder. Templates use no ids, so they work in any Space; the
// editor asks for the statuses, people and fields they need.
type Template struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Definition  json.RawMessage `json:"definition"`
}

// Templates are the built-in starting points (AUT-17). They are data, validated by the tests.
func Templates() []Template {
	return []Template{
		{ID: "thank-approver", Name: "Thank the approver", Description: "Comment on the item when it is approved.",
			Definition: json.RawMessage(`{"when":{"type":"approval.decided","decision":"approved"},"if":[],"then":[{"type":"comment","text":"Approved. Thank you."}]}`)},
		{ID: "due-soon", Name: "Remind assignees before the deadline", Description: "Notify the assignees two days before the due date.",
			Definition: json.RawMessage(`{"when":{"type":"item.due_reached","days_before":2},"if":[{"field":"assignee","op":"not_empty"}],"then":[{"type":"notify","assignees":true,"message":"{item.title} is due in 2 days."}]}`)},
		{ID: "flag-overdue", Name: "Flag overdue work", Description: "Set importance to high once an item is one day past due.",
			Definition: json.RawMessage(`{"when":{"type":"item.due_reached","days_before":-1},"if":[],"then":[{"type":"set_field","field":"importance","value":"high"}]}`)},
		{ID: "review-subtask", Name: "Open a review subtask", Description: "Create a review subtask under each new task.",
			Definition: json.RawMessage(`{"when":{"type":"item.created"},"if":[{"field":"item_type","op":"eq","value":"task"}],"then":[{"type":"create_item","title":"Review: {item.title}","kind":"task","under":"item"}]}`)},
	}
}
