package httpserver_test

import (
	"fmt"
	"net/url"
	"slices"
	"testing"
	"time"
)

func TestSearchPrefixAndScopesRespectVisibility(t *testing.T) {
	f := newFixture(t)
	owner, member, external := f.person("admin"), f.person("regular"), f.person("external")
	first, second, hidden := f.space("Visible one"), f.space("Visible two"), f.space("Hidden")
	f.share(first, "user", member.id, "read")
	f.share(second, "user", member.id, "read")
	f.share(first, "user", external.id, "read")
	f.exec("UPDATE users SET first_name='Roadmap',last_name='Owner' WHERE id=$1", owner.id)

	item := func(kind, title, space string, parent *string) string {
		t.Helper()
		id := f.queryID("INSERT INTO items(type,title,created_by) VALUES ($1,$2,$3) RETURNING id::text", kind, title, owner.id)
		f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", id, space, parent)
		return id
	}
	project := item("project", "Roadmap launch", first, nil)
	folder := item("folder", "Design drafts", first, &project)
	nested := item("task", "Roadmap preparation", first, &folder)
	outside := item("task", "Roadmap marketing", first, nil)
	otherSpace := item("task", "Roadmap retro", second, nil)
	secret := item("task", "Roadmap private", hidden, nil)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, hidden)
	secretPlacement := item("task", "Roadmap hidden project task", hidden, &project)
	deleted := item("task", "Roadmap deleted", first, nil)
	f.exec("UPDATE items SET deleted_at=now() WHERE id=$1", deleted)
	f.exec(`INSERT INTO comments(id,item_id,author_id,body,body_text) VALUES
(gen_random_uuid(),$1,$3,'{"type":"doc"}','A long private roadmap note'),
(gen_random_uuid(),$2,$3,'{"type":"doc"}','Review the roadmap checklist')`, secret, nested, owner.id)

	check := func(p *person, path string, expectedItems []string, comments, users int) {
		t.Helper()
		reply := f.call(p, "GET", path, nil)
		if reply.status != 200 {
			t.Fatalf("%s: status %d %v", path, reply.status, reply.body)
		}
		var ids []string
		for _, value := range reply.body["items"].([]any) {
			ids = append(ids, value.(map[string]any)["id"].(string))
		}
		slices.Sort(ids)
		slices.Sort(expectedItems)
		if !slices.Equal(ids, expectedItems) || len(reply.body["comments"].([]any)) != comments || len(reply.body["users"].([]any)) != users {
			t.Fatalf("%s: items %v expected %v, body %+v", path, ids, expectedItems, reply.body)
		}
		if comments > 0 {
			comment := reply.body["comments"].([]any)[0].(map[string]any)
			if comment["item_id"] != nested || comment["snippet"] != "Review the roadmap checklist" {
				t.Fatalf("unexpected comment snippet: %v", comment)
			}
		}
	}
	check(member, "/search?q=roadm", []string{project, nested, outside, otherSpace}, 1, 1)
	check(member, "/search?q=roadm&space_id="+first, []string{project, nested, outside}, 1, 1)
	check(member, "/search?q=roadm&container_id="+project, []string{project, nested}, 1, 1)
	check(member, "/search?q=roadm+prep", []string{nested}, 0, 0)
	check(external, "/search?q=roadm", []string{project, nested, outside}, 1, 0)
	check(member, "/search?q="+url.QueryEscape("'"), nil, 0, 0)

	for _, path := range []string{"/search?q=roadm&space_id=" + hidden, "/search?q=roadm&container_id=" + project + "&space_id=" + hidden} {
		if reply := f.call(member, "GET", path, nil); reply.status != 404 {
			t.Fatalf("inaccessible scope %s: %+v", path, reply)
		}
	}
	if reply := f.call(owner, "GET", "/search?q=roadm&container_id="+project+"&space_id="+hidden, nil); reply.status != 200 || len(reply.body["items"].([]any)) != 2 {
		t.Fatalf("admin should see two matches in the second project placement, including %s: %+v", secretPlacement, reply)
	}
}

func TestSearchTenThousandItemsP95(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	visible, hidden := f.space("Visible"), f.space("Hidden")
	f.share(visible, "user", member.id, "read")
	f.exec(`WITH dataset AS (
  INSERT INTO items(type,title,created_by)
  SELECT CASE WHEN n % 10 = 0 THEN 'project'::item_type ELSE 'task'::item_type END,
      CASE WHEN n % 3 = 0 THEN 'Roadmap review ' ELSE 'Roadmap rollout ' END || n,
      $1 FROM generate_series(1,10000) n
  RETURNING id,seq
) INSERT INTO item_locations(item_id,space_id,rank)
SELECT id, CASE WHEN seq % 3 = 0 THEN $2::uuid ELSE $3::uuid END, 'U' FROM dataset`, admin.id, visible, hidden)
	f.exec(`INSERT INTO comments(id,item_id,author_id,body,body_text)
SELECT gen_random_uuid(),id,$1,'{"type":"doc"}','Roadmap review progress notes' FROM items WHERE seq % 20 = 0`, admin.id)
	f.exec("ANALYZE items")
	f.exec("ANALYZE comments")
	f.exec("ANALYZE item_locations")

	queries := []string{"roadm", "review", "roll", "not-found"}
	durations := make([]time.Duration, 0, 24)
	for i := -4; i < 24; i++ {
		start := time.Now()
		reply := f.call(member, "GET", fmt.Sprintf("/search?limit=5&q=%s", queries[(i+4)%len(queries)]), nil)
		if reply.status != 200 {
			t.Fatalf("unexpected response %+v", reply)
		}
		if i >= 0 {
			durations = append(durations, time.Since(start))
		}
	}
	slices.Sort(durations)
	p95 := durations[22] // nearest-rank p95 for 24 samples.
	t.Logf("10,000 items and 500 comments, 24 HTTP queries after 4 warmups: p95=%v max=%v", p95, durations[23])
	if p95 >= 800*time.Millisecond {
		t.Fatalf("search p95 = %v, want < 800ms", p95)
	}
}
