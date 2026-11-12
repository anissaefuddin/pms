package items

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// descriptionText validates the supported ProseMirror subset and extracts text for search.
func descriptionText(raw json.RawMessage) (string, error) {
	var mentions []Mention
	return richText(raw, &mentions)
}

type Mention struct {
	Kind string
	ID   *uuid.UUID
}

func CommentText(raw json.RawMessage) (string, []Mention, error) {
	var mentions []Mention
	plain, err := richText(raw, &mentions)
	if err == nil && plain == "" {
		err = Invalid("comment cannot be empty")
	}
	return plain, mentions, err
}

func richText(raw json.RawMessage, mentions *[]Mention) (string, error) {
	var text strings.Builder
	var visit func(json.RawMessage, int) bool
	visit = func(raw json.RawMessage, depth int) bool {
		var n struct {
			Type    string
			Text    string
			Content []json.RawMessage
			Attrs   struct {
				Kind  string
				ID    string
				Label string
			}
			Marks []struct {
				Type  string
				Attrs map[string]json.RawMessage
			}
		}
		if depth > 32 || json.Unmarshal(raw, &n) != nil || (depth == 0) != (n.Type == "doc") {
			return false
		}
		switch n.Type {
		case "doc", "paragraph", "heading", "bulletList", "orderedList", "listItem", "blockquote", "codeBlock", "hardBreak", "horizontalRule":
			if n.Text != "" || len(n.Marks) != 0 {
				return false
			}
		case "text":
			if n.Text == "" || strings.ContainsRune(n.Text, 0) || len(n.Content) != 0 {
				return false
			}
			text.WriteString(n.Text)
		case "mention":
			if mentions == nil || n.Text != "" || len(n.Content) != 0 || len(n.Marks) != 0 || len(*mentions) >= 100 ||
				n.Attrs.Label == "" || len(n.Attrs.Label) > 320 || strings.ContainsRune(n.Attrs.Label, 0) {
				return false
			}
			m := Mention{Kind: n.Attrs.Kind}
			switch m.Kind {
			case "user", "group":
				id, err := uuid.Parse(n.Attrs.ID)
				if err != nil || id == uuid.Nil {
					return false
				}
				m.ID = &id
			case "followers":
				if n.Attrs.ID != "" {
					return false
				}
			default:
				return false
			}
			*mentions = append(*mentions, m)
			text.WriteString("@" + n.Attrs.Label)
		default:
			return false
		}
		for _, mark := range n.Marks {
			switch mark.Type {
			case "bold", "italic", "strike", "code", "underline":
			case "link":
				var href string
				if json.Unmarshal(mark.Attrs["href"], &href) != nil {
					return false
				}
				u, err := url.Parse(href)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "mailto") {
					return false
				}
			default:
				return false
			}
		}
		for _, child := range n.Content {
			if !visit(child, depth+1) {
				return false
			}
		}
		if n.Type != "text" && text.Len() > 0 {
			text.WriteByte(' ')
		}
		return true
	}
	if !visit(raw, 0) {
		return "", Invalid("description must be supported ProseMirror JSON")
	}
	return strings.TrimSpace(text.String()), nil
}
