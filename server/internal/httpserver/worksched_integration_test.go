package httpserver_test

import "testing"

func TestWorkSchedule(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	const path = "/settings/work-schedule"

	got := f.call(member, "GET", path, nil)
	if got.status != 200 || got.header.Get("ETag") != `"1"` || got.body["daily_hours"] != float64(8) {
		t.Fatalf("default schedule: %+v etag=%q", got.body, got.header.Get("ETag"))
	}
	days := got.body["working_days"].([]any)
	if len(days) != 5 || days[0] != float64(1) || days[4] != float64(5) {
		t.Fatalf("default working days: %v", days)
	}
	body := func(mod func(map[string]any)) map[string]any {
		b := map[string]any{"working_days": []int{1, 2, 3, 4, 6}, "daily_hours": 6,
			"exceptions": []map[string]string{{"date": "2026-12-25", "name": "Christmas"}, {"date": "2026-12-24", "name": "Eve"}}}
		if mod != nil {
			mod(b)
		}
		return b
	}
	f.want(f.call(member, "PUT", path, body(nil), "If-Match", `"1"`), 403, "non-admin write")
	f.want(f.call(admin, "PUT", path, body(nil)), 428, "missing If-Match")
	for name, mod := range map[string]func(map[string]any){
		"no working days":  func(b map[string]any) { b["working_days"] = []int{} },
		"duplicate day":    func(b map[string]any) { b["working_days"] = []int{1, 1} },
		"day out of range": func(b map[string]any) { b["working_days"] = []int{7} },
		"zero hours":       func(b map[string]any) { b["daily_hours"] = 0 },
		"25 hours":         func(b map[string]any) { b["daily_hours"] = 25 },
		"bad date":         func(b map[string]any) { b["exceptions"] = []map[string]string{{"date": "2026-02-30", "name": "x"}} },
		"duplicate holiday": func(b map[string]any) {
			b["exceptions"] = []map[string]string{{"date": "2026-12-25", "name": "a"}, {"date": "2026-12-25", "name": "b"}}
		},
	} {
		f.want(f.call(admin, "PUT", path, body(mod), "If-Match", `"1"`), 422, name)
	}
	if got := f.call(admin, "GET", path, nil); got.header.Get("ETag") != `"1"` {
		t.Fatalf("rejected writes must not bump the version: %q", got.header.Get("ETag"))
	}

	saved := f.call(admin, "PUT", path, body(nil), "If-Match", `"1"`)
	if saved.status != 200 || saved.header.Get("ETag") != `"2"` || saved.body["daily_hours"] != float64(6) {
		t.Fatalf("save: %+v", saved)
	}
	ex := saved.body["exceptions"].([]any)
	if len(ex) != 2 || ex[0].(map[string]any)["date"] != "2026-12-24" { // sorted by date
		t.Fatalf("exceptions: %v", ex)
	}
	f.want(f.call(admin, "PUT", path, body(nil), "If-Match", `"1"`), 409, "stale version")
	cleared := f.call(admin, "PUT", path, body(func(b map[string]any) { b["exceptions"] = []map[string]string{} }), "If-Match", `"2"`)
	if cleared.status != 200 || len(cleared.body["exceptions"].([]any)) != 0 {
		t.Fatalf("clear holidays: %+v", cleared)
	}
}
