package httpserver_test

import "testing"

func TestProfilePreferencesPersist(t *testing.T) {
	f := newFixture(t)
	member := f.person("regular")
	values := map[string]string{
		"first_name": "Pilot", "last_name": "Member", "company": "Acme", "job_title": "Designer", "country": "ID",
		"location": "Jakarta", "department": "Product", "phone": "+62210000", "timezone": "Asia/Jakarta", "date_format": "dd/mm/yyyy",
		"time_format": "12h", "week_start": "sunday", "density": "comfortable", "theme": "dark", "comment_shortcut": "enter",
	}
	if got := f.call(member, "PATCH", "/me", values); got.status != 200 {
		t.Fatalf("patch: %+v", got)
	}
	reloaded := f.call(member, "GET", "/me", nil)
	for field, want := range values {
		if reloaded.body[field] != want {
			t.Errorf("%s = %v, want %q", field, reloaded.body[field], want)
		}
	}
	for _, bad := range []map[string]string{{"timezone": "Unknown/Zone"}, {"user_type": "admin"}, {"density": "invalid"}} {
		if got := f.call(member, "PATCH", "/me", bad); got.status != 422 {
			t.Fatalf("invalid profile accepted: %+v", got)
		}
	}
	if got := f.call(member, "PATCH", "/me", map[string]string{"company": ""}); got.status != 200 || got.body["company"] != nil {
		t.Fatalf("clear company: %+v", got)
	}
}
