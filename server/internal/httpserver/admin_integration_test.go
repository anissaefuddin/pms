package httpserver_test

import "testing"

func TestAdminSeatAndLifecycle(t *testing.T) {
	f := newFixtureWithSeats(t, 3)
	admin, member := f.person("admin"), f.person("regular")
	invite := map[string]string{"email": "pilot-new@example.test", "user_type": "viewer", "message": "Welcome to pilot"}
	path := "/users/invitations"
	if got := f.call(member, "POST", path, invite); got.status != 403 {
		t.Fatalf("non-admin invite: %+v", got)
	}
	if got := f.call(admin, "POST", path, invite); got.status != 422 || got.body["code"] != "seats_exhausted" {
		t.Fatalf("full seats: %+v", got)
	}
	if got := f.call(admin, "PATCH", "/users/"+admin.id, map[string]string{"user_type": "viewer"}); got.status != 409 || got.body["code"] != "last_admin" {
		t.Fatalf("last admin demotion: %+v", got)
	}
	if got := f.call(admin, "POST", "/users/"+admin.id+"/deactivate", nil); got.status != 409 || got.body["code"] != "last_admin" {
		t.Fatalf("last admin deactivation: %+v", got)
	}
	if got := f.call(admin, "POST", "/users/"+member.id+"/deactivate", nil); got.status != 200 || got.body["status"] != "deactivated" {
		t.Fatalf("deactivate: %+v", got)
	}
	if got := f.call(member, "GET", "/me", nil); got.status != 401 {
		t.Fatalf("old session: %+v", got)
	}
	created := f.call(admin, "POST", path, invite)
	if created.status != 201 || created.body["status"] != "invited" {
		t.Fatalf("invite: %+v", created)
	}
	id := created.body["id"].(string)
	if got := f.call(admin, "POST", "/users/"+id+"/invitation/resend", nil); got.status != 204 {
		t.Fatalf("resend: %+v", got)
	}
	if got := f.queryID(`SELECT count(*)::text FROM river_job WHERE kind='send_invitation_email' AND args->>'user_id'=$1
AND NOT (args ? 'url') AND NOT (args ? 'token')`, id); got != "2" {
		t.Fatalf("invite/resend job arguments: %s", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM user_tokens WHERE user_id=$1", id); got != "0" {
		t.Fatalf("pre-generated invite tokens: %s", got)
	}
	if got := f.call(admin, "GET", "/seats", nil); got.status != 200 || got.body["used"] != float64(1) || got.body["pending"] != float64(2) || got.body["available"] != float64(0) {
		t.Fatalf("seat totals: %+v", got)
	}
	if got := f.call(admin, "POST", "/users/"+member.id+"/reactivate", nil); got.status != 422 || got.body["code"] != "seats_exhausted" {
		t.Fatalf("reactivate without seat: %+v", got)
	}
	if got := f.call(admin, "POST", "/users/"+id+"/deactivate", nil); got.status != 200 {
		t.Fatalf("cancel invite: %+v", got)
	}
	if got := f.call(admin, "POST", "/users/"+member.id+"/reactivate", nil); got.status != 200 || got.body["status"] != "active" {
		t.Fatalf("reactivate with seat: %+v", got)
	}
	if got := f.call(admin, "GET", "/users/types", nil); got.status != 200 || got.body["user_types"] == nil {
		t.Fatalf("policy matrix: %+v", got)
	}
	group := f.call(admin, "POST", "/groups", map[string]any{"name": "Pilot team", "member_ids": []string{member.id}})
	if group.status != 201 {
		t.Fatalf("create group: %+v", group)
	}
	if got := f.call(admin, "DELETE", "/groups/"+group.body["id"].(string), nil); got.status != 204 {
		t.Fatalf("delete group: %+v", got)
	}
}
