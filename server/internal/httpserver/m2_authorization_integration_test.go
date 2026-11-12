package httpserver_test

import (
	"net/http"
	"testing"
)

// TestM2AuthorizationMatrix extends the policy matrix to dependencies and custom fields (S2.6.1) and probes IDOR.
func TestM2AuthorizationMatrix(t *testing.T) {
	f := newFixture(t)
	owner, editor, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	space, hidden := f.space("M2 matrix A"), f.space("M2 matrix B")
	f.share(space, "user", owner.id, "full")
	f.share(space, "user", editor.id, "editor")
	f.share(space, "user", reader.id, "read")
	f.share(hidden, "user", owner.id, "full")
	a, b := customFieldItem(f, owner, space, "A"), customFieldItem(f, owner, space, "B")
	secret := customFieldItem(f, owner, hidden, "Hidden predecessor")
	dep := f.call(owner, http.MethodPost, dependencyPath(b), dependencyInput(a, "FS"))
	f.want(dep, http.StatusCreated, "owner links A to B")
	depID := dep.body["id"].(string)
	field := f.call(owner, http.MethodPost, customFieldsPath(space), map[string]any{"name": "Score", "type": "number"})
	f.want(field, http.StatusCreated, "owner creates a field")
	fieldID := field.body["id"].(string)
	hiddenField := f.call(owner, http.MethodPost, customFieldsPath(hidden), map[string]any{"name": "Secret", "type": "number"})
	f.want(hiddenField, http.StatusCreated, "owner creates a field in the other Space")

	type probe struct {
		who          *person
		method, path string
		body         any
		headers      []string
		want         int
		what         string
	}
	for _, p := range []probe{
		// Dependencies: reading needs View on the item, writing needs Edit on the successor.
		{reader, "GET", dependencyPath(b), nil, nil, 200, "reader lists dependencies"},
		{outsider, "GET", dependencyPath(b), nil, nil, 404, "outsider cannot list dependencies"},
		{reader, "POST", dependencyPath(a), dependencyInput(b, "SS"), nil, 403, "reader cannot create a dependency"},
		{outsider, "POST", dependencyPath(a), dependencyInput(b, "SS"), nil, 404, "outsider cannot create a dependency"},
		{editor, "POST", dependencyPath(a), dependencyInput(secret, "FS"), nil, 404, "a hidden predecessor looks like it does not exist"},
		{reader, "PATCH", "/dependencies/" + depID, map[string]any{"type": "SS"}, nil, 403, "reader cannot change a dependency"},
		{outsider, "PATCH", "/dependencies/" + depID, map[string]any{"type": "SS"}, nil, 404, "outsider cannot probe a dependency id"},
		{outsider, "DELETE", "/dependencies/" + depID, nil, nil, 404, "outsider cannot delete a dependency"},
		{reader, "DELETE", "/dependencies/" + depID, nil, nil, 403, "reader cannot delete a dependency"},
		// Custom field definitions and values.
		{reader, "GET", customFieldsPath(space), nil, nil, 200, "reader lists definitions"},
		{outsider, "GET", customFieldsPath(space), nil, nil, 404, "outsider cannot list definitions"},
		{reader, "POST", customFieldsPath(space), map[string]any{"name": "X", "type": "text"}, nil, 403, "reader cannot create a definition"},
		{outsider, "POST", customFieldsPath(space), map[string]any{"name": "X", "type": "text"}, nil, 404, "outsider cannot create a definition"},
		{outsider, "PATCH", customFieldsPath(space) + "/" + fieldID, map[string]any{"name": "Hacked"}, nil, 404, "outsider cannot rename a definition"},
		{editor, "PATCH", customFieldsPath(hidden) + "/" + fieldID, map[string]any{"name": "Cross"}, nil, 404, "a field id under another Space's path is not found"},
		{outsider, "DELETE", customFieldsPath(space) + "/" + fieldID, nil, nil, 404, "outsider cannot archive a definition"},
		{reader, "PATCH", "/items/" + a, map[string]any{"fields": map[string]any{fieldID: 5}}, []string{"If-Match", `"1"`}, 403, "reader cannot set a value"},
		{outsider, "PATCH", "/items/" + a, map[string]any{"fields": map[string]any{fieldID: 5}}, []string{"If-Match", `"1"`}, 404, "outsider cannot set a value"},
		{editor, "PATCH", "/items/" + a, map[string]any{"fields": map[string]any{hiddenField.body["id"].(string): 5}}, []string{"If-Match", `"1"`}, 422, "a definition from another Space is refused"},
		{editor, "PATCH", "/items/" + a, map[string]any{"fields": map[string]any{fieldID: 5}}, []string{"If-Match", `"1"`}, 200, "editor sets a value"},
		// Locations, importance and the batch dependency query follow item permissions too.
		{reader, "PUT", "/items/" + a + "/locations", map[string]any{"locations": []any{map[string]any{"space_id": space}}}, nil, 403, "reader cannot replace locations"},
		{outsider, "PUT", "/items/" + a + "/locations", map[string]any{"locations": []any{map[string]any{"space_id": space}}}, nil, 404, "outsider cannot replace locations"},
		{reader, "PATCH", "/items/" + a, map[string]any{"importance": "high"}, []string{"If-Match", `"2"`}, 403, "reader cannot change importance"},
	} {
		got := f.call(p.who, p.method, p.path, p.body, p.headers...)
		if got.status != p.want {
			t.Errorf("%s: %s %s = %d, want %d (%v)", p.what, p.method, p.path, got.status, p.want, got.body)
		}
	}
	// Nothing above may have changed the definition or the dependency.
	if got := f.call(owner, "GET", dependencyPath(b), nil); len(got.items()) != 1 || got.items()[0]["type"] != "FS" {
		t.Errorf("dependency changed by a refused call: %+v", got.body)
	}
	if n := f.queryID("SELECT count(*)::text FROM custom_fields WHERE space_id=$1 AND name='Score' AND archived_at IS NULL", space); n != "1" {
		t.Errorf("definition changed by a refused call: %s", n)
	}
}
