package items

import (
	"encoding/json"
	"net/url"
	"strings"
)

// descriptionText validates the supported ProseMirror subset and extracts text for search.
func descriptionText(raw json.RawMessage) (string, error) {
	var text strings.Builder
	var visit func(json.RawMessage, int) bool
	visit = func(raw json.RawMessage, depth int) bool {
		var n struct {
			Type    string
			Text    string
			Content []json.RawMessage
			Marks   []struct {
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
