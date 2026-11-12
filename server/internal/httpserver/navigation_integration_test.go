package httpserver_test

import (
	"strconv"
	"testing"
)

func TestSharedNavigationFiltersAndPaginates(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	hidden, visible := f.space("Hidden"), f.space("Visible")
	f.share(visible, "user", member.id, "read")
	personal := f.queryID("SELECT id::text FROM spaces WHERE owner_id=$1 AND kind='personal'", admin.id)
	add := func(space string, shared bool, rank string) string {
		id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Navigation',$1) RETURNING id::text", admin.id)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,$3)", id, space, rank)
		if shared {
			f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2,'read')", id, member.id)
		}
		return id
	}
	first, second := add(hidden, true, "F"), add(hidden, true, "G")
	add(hidden, false, "H")
	add(visible, true, "F")
	add(personal, false, "F")
	page := f.call(member, "GET", "/me/shared?limit=1", nil)
	if page.status != 200 || len(page.items()) != 1 || page.body["next_cursor"] == nil {
		t.Fatalf("shared page: %+v", page)
	}
	next := f.call(member, "GET", "/me/shared?limit=1&cursor="+page.body["next_cursor"].(string), nil)
	if next.status != 200 || len(next.items()) != 1 || next.body["next_cursor"] != nil {
		t.Fatalf("next shared page: %+v", next)
	}
	got := map[any]bool{page.items()[0]["id"]: true, next.items()[0]["id"]: true}
	if !got[first] || !got[second] {
		t.Fatalf("unexpected shared items: %v", got)
	}
	me := f.call(member, "GET", "/me", nil)
	if me.status != 200 || me.body["account_name"] != "Acme" {
		t.Fatalf("workspace name: %+v", me)
	}
}

func TestRateLimitResponseIncludesRetryAfter(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 6; i++ {
		res := f.call(nil, "POST", "/auth/password/forgot", map[string]string{"email": "unknown@example.com"})
		if i < 5 && res.status != 202 {
			t.Fatalf("attempt %d: %+v", i+1, res)
		}
		if i == 5 {
			seconds, err := strconv.Atoi(res.header.Get("Retry-After"))
			if res.status != 429 || res.body["code"] != "rate_limited" || err != nil || seconds < 1 || seconds > 60 {
				t.Fatalf("rate limited response: %+v", res)
			}
		}
	}
}
