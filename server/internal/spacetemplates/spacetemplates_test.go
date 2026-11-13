package spacetemplates

import "testing"

func TestCatalogIsConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, tpl := range All() {
		if tpl.ID == "" || seen[tpl.ID] || tpl.Name == "" || tpl.Category == "" || tpl.Version < 1 {
			t.Fatalf("bad template header: %+v", tpl.Summary())
		}
		seen[tpl.ID] = true
		if !tpl.Summary().Compatible {
			t.Errorf("%s is newer than this server", tpl.ID)
		}
		for _, form := range tpl.Forms {
			if form.Project < -1 || form.Project >= len(tpl.Projects) {
				t.Errorf("%s: form %q points at project %d", tpl.ID, form.Name, form.Project)
			}
		}
	}
	newer := Template{Format: Format + 1}
	if newer.Summary().Compatible {
		t.Error("a template with a newer format must be reported incompatible")
	}
	if _, ok := Find("nope"); ok {
		t.Error("unknown id found")
	}
}
