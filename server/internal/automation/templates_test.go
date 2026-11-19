package automation

import "testing"

// AUT-17: every built-in template is a valid rule, so a Space editor never gets a template that the server refuses.
func TestEveryTemplateIsAValidRule(t *testing.T) {
	seen := map[string]bool{}
	for _, tpl := range Templates() {
		if seen[tpl.ID] {
			t.Fatalf("duplicate template %s", tpl.ID)
		}
		seen[tpl.ID] = true
		if _, err := ParseDefinition(tpl.Definition); err != nil {
			t.Fatalf("template %s: %v", tpl.ID, err)
		}
	}
	if len(seen) < 4 {
		t.Fatal("the built-in templates are missing")
	}
}
