package httpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

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

func TestProfileEmailAndTaskDefaultsAndAccountInfo(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	got := f.call(member, "GET", "/me", nil)
	if got.status != 200 || len(got.body["email_muted"].([]any)) != 0 || got.body["default_task_duration_days"] != nil {
		t.Fatalf("defaults: %+v", got)
	}
	if got := f.call(member, "PATCH", "/me", map[string]any{"email_muted": []string{"shared", "report", "shared"}, "default_task_duration_days": 3, "default_task_effort_minutes": 90}); got.status != 200 ||
		len(got.body["email_muted"].([]any)) != 2 || got.body["default_task_duration_days"] != float64(3) || got.body["default_task_effort_minutes"] != float64(90) {
		t.Fatalf("save: %+v", got)
	}
	for _, bad := range []map[string]any{{"email_muted": []string{"comment"}}, {"email_muted": nil}, {"default_task_duration_days": -1}, {"default_task_duration_days": 4000}, {"default_task_effort_minutes": "x"}} {
		if got := f.call(member, "PATCH", "/me", bad); got.status != 422 {
			t.Fatalf("invalid accepted %v: %+v", bad, got)
		}
	}
	if got := f.call(member, "PATCH", "/me", map[string]any{"email_muted": []string{}, "default_task_duration_days": nil}); got.status != 200 ||
		len(got.body["email_muted"].([]any)) != 0 || got.body["default_task_duration_days"] != nil || got.body["default_task_effort_minutes"] != float64(90) {
		t.Fatalf("clear: %+v", got)
	}
	account := f.call(member, "GET", "/account", nil)
	admins, _ := account.body["administrators"].([]any)
	if account.status != 200 || account.body["role"] != "regular" || len(admins) != 1 || admins[0].(map[string]any)["id"] != admin.id || account.body["seats"] == nil || account.body["joined_at"] == nil {
		t.Fatalf("account: %+v", account)
	}
	if resp, err := http.Get(f.srv.URL + "/api/v1/account"); err != nil || resp.StatusCode != 401 {
		t.Fatalf("anonymous account: %v %v", resp, err)
	}
}

func TestOnboardingIsAdminOnlyAndShownOnce(t *testing.T) {
	f := newFixture(t)
	admin, other, member := f.person("admin"), f.person("admin"), f.person("regular")
	if got := f.call(admin, "GET", "/me", nil); got.body["onboarding_pending"] != true {
		t.Fatalf("admin must see the wizard: %+v", got)
	}
	if got := f.call(member, "GET", "/me", nil); got.body["onboarding_pending"] != false {
		t.Fatalf("member must not: %+v", got)
	}
	for _, p := range []*person{member} {
		if got := f.call(p, "POST", "/onboarding/complete", nil); got.status != 403 {
			t.Fatalf("member complete: %+v", got)
		}
		if got := f.call(p, "PATCH", "/account", map[string]string{"name": "X"}); got.status != 403 {
			t.Fatalf("member rename: %+v", got)
		}
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", 121)} {
		if got := f.call(admin, "PATCH", "/account", map[string]string{"name": bad}); got.status != 422 {
			t.Fatalf("bad name %q accepted: %+v", bad, got)
		}
	}
	if got := f.call(admin, "PATCH", "/account", map[string]string{"name": "  Pilot Co "}); got.status != 200 || got.body["name"] != "Pilot Co" {
		t.Fatalf("rename: %+v", got)
	}
	if got := f.call(admin, "GET", "/me", nil); got.body["account_name"] != "Pilot Co" {
		t.Fatalf("account_name: %+v", got)
	}
	for range 2 { // idempotent
		if got := f.call(admin, "POST", "/onboarding/complete", nil); got.status != 204 {
			t.Fatalf("complete: %+v", got)
		}
	}
	for _, p := range []*person{admin, other} {
		if got := f.call(p, "GET", "/me", nil); got.body["onboarding_pending"] != false {
			t.Fatalf("wizard shown again: %+v", got)
		}
	}
}
