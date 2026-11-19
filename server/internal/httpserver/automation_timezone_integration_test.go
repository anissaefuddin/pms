package httpserver_test

import (
	"testing"
	"time"
)

// Decision: deadlines follow the workspace's time zone. An unknown zone falls back to UTC.
func TestDeadlinesFollowTheWorkspaceZone(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	e.engine.Now = func() time.Time { return time.Date(2026, 12, 1, 20, 0, 0, 0, time.UTC) } // 2 December in Jakarta
	e.rule(t, e.editor, "Due today", `{"when":{"type":"item.due_reached","days_before":0},"then":[{"type":"set_field","field":"importance","value":"high"}]}`)
	today := e.item(t, e.editor, "Due on the local day", map[string]any{"due_date": "2026-12-02"})
	yesterday := e.item(t, e.editor, "Due the day before", map[string]any{"due_date": "2026-12-01"})

	f.exec("UPDATE work_schedules SET timezone='Asia/Jakarta' WHERE id=1")
	if _, err := e.engine.EmitDue(f.ctx); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	if importanceOf(e, today["id"].(string)) != "high" || importanceOf(e, yesterday["id"].(string)) != "normal" {
		t.Fatal("in Jakarta the 2nd is today, not the 1st")
	}

	// A zone name the server does not know falls back to UTC, where it is still the 1st.
	f.exec("UPDATE work_schedules SET timezone='Not/AZone' WHERE id=1")
	fallback := e.item(t, e.editor, "Due by UTC", map[string]any{"due_date": "2026-12-01"})
	if _, err := e.engine.EmitDue(f.ctx); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	if importanceOf(e, fallback["id"].(string)) != "high" {
		t.Fatal("an unknown zone falls back to UTC")
	}
}
