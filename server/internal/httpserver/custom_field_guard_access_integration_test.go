package httpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

// A guard that requires a restricted field names it only to people who may read it (CF-D03): the refusal is the same
// for everyone, but the field's name is shown only to a grant holder.
func TestStatusGuardDoesNotNameRestrictedFieldToOthers(t *testing.T) {
	f := newFixture(t)
	admin, owner, editor := f.person("admin"), f.person("regular"), f.person("regular")
	space := f.space("Guard access")
	f.share(space, "user", owner.id, "full")
	f.share(space, "user", editor.id, "full")

	wf := f.call(admin, http.MethodGet, "/workflows", nil).items()[0]
	var done string
	for _, st := range wf["statuses"].([]any) {
		if m := st.(map[string]any); m["group"] == "completed" && done == "" {
			done = m["id"].(string)
		}
	}
	field := f.call(owner, http.MethodPost, "/spaces/"+space+"/custom-fields", map[string]any{"name": "Secret ticket", "type": "text"})
	f.want(field, 201, "field")
	fid := field.body["id"].(string)
	f.want(f.call(owner, http.MethodPut, "/spaces/"+space+"/custom-fields/"+fid+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": owner.id, "level": "edit"}}}), 200, "restrict to the owner")
	f.want(f.call(admin, http.MethodPatch, "/statuses/"+done, map[string]any{"required_fields": []string{fid}}), 200, "guard requires the field")

	item := f.call(editor, http.MethodPost, "/items", map[string]any{"type": "task", "title": "Guarded", "locations": []map[string]any{{"space_id": space}}})
	f.want(item, 201, "item")
	id := item.body["id"].(string)

	refusedFor := func(p *person) string {
		got := f.call(p, http.MethodPatch, "/items/"+id, map[string]any{"status_id": done}, "If-Match", `"1"`)
		if got.status != http.StatusUnprocessableEntity {
			t.Fatalf("the guard refuses: %+v", got.body)
		}
		var text []string
		for _, e := range got.body["errors"].([]any) {
			text = append(text, e.(map[string]any)["message"].(string))
		}
		return strings.Join(text, " | ")
	}
	if msg := refusedFor(editor); strings.Contains(msg, "Secret ticket") {
		t.Fatalf("a person without the grant is not told the field's name: %s", msg)
	}
	if msg := refusedFor(owner); !strings.Contains(msg, "Secret ticket") {
		t.Fatalf("the grant holder is told which field is missing: %s", msg)
	}
}
