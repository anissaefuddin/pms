package httpserver_test

import (
	"net/http"
	"testing"
)

// B3-09 and B3-10: a dependency joins two items of one stable owner, a legacy cross-owner link never moves scheduling, and a
// blueprint copy takes its destination's owner, dropping assignees who are not members of that Customer.

// DR-01..DR-06: dependency create and update need one owner; delete stays open; scheduling follows only compatible edges.
func TestDependencyRequiresOneOwner(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	clientA := f.createClient(admin, "DR-A")["id"].(string)
	clientB := f.createClient(admin, "DR-B")["id"].(string)
	member := f.person("regular")
	dual := f.person("regular") // member of both Customers, so it can see items of both
	f.addMember(admin, clientA, member)
	f.addMember(admin, clientA, dual)
	f.addMember(admin, clientB, dual)
	spaceA := f.scopedSpace("DR A", "client", clientA)
	spaceB := f.scopedSpace("DR B", "client", clientB)
	internal := f.internalSpace("DR internal")
	for _, s := range []string{spaceA, spaceB, internal} {
		f.share(s, "user", member.id, "full")
		f.share(s, "user", dual.id, "full")
	}
	a1 := f.itemIn(spaceA, "client", clientA, member.id, "A1")
	a2 := f.itemIn(spaceA, "client", clientA, member.id, "A2")
	b1 := f.itemIn(spaceB, "client", clientB, dual.id, "B1")
	in1 := f.itemIn(internal, "internal", "", admin.id, "Internal")
	count := func() string { return f.queryID("SELECT count(*)::text FROM item_dependencies") }
	// scheduledItem always writes scope 'internal', so a client-owned scheduled item is built here with its Customer.
	clientItem := func(space, client, title, start, due string) string {
		id := f.queryID(`INSERT INTO items(type,title,created_by,start_date,due_date,duration_days,is_milestone,scope_state,client_id)
VALUES ('task',$1,$2,$3::text::date,$4::text::date,1,false,'client',$5::uuid) RETURNING id::text`, title, member.id, start, due, client)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		return id
	}

	t.Run("DR-01 internal to client is refused and writes nothing", func(t *testing.T) {
		f := f.on(t)
		f.want(f.call(member, http.MethodPost, dependencyPath(a1), map[string]any{"predecessor_id": in1, "type": "FS", "lag": 0}),
			http.StatusUnprocessableEntity, "cross-owner link")
		if got := count(); got != "0" {
			t.Fatalf("a refused link was written: %s rows", got)
		}
	})
	t.Run("DR-02 two different Customers are refused", func(t *testing.T) {
		f := f.on(t)
		f.want(f.call(dual, http.MethodPost, dependencyPath(a1), map[string]any{"predecessor_id": b1, "type": "FS", "lag": 0}),
			http.StatusUnprocessableEntity, "cross-Customer link")
		if got := count(); got != "0" {
			t.Fatalf("a refused link was written: %s rows", got)
		}
	})
	t.Run("DR-03 two items of one Customer are accepted", func(t *testing.T) {
		f := f.on(t)
		f.want(f.call(member, http.MethodPost, dependencyPath(a2), map[string]any{"predecessor_id": a1, "type": "FS", "lag": 0}),
			http.StatusCreated, "same-owner link")
	})
	t.Run("DR-04 an unresolved endpoint is refused", func(t *testing.T) {
		f := f.on(t)
		legacy := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Legacy',$1) RETURNING id::text", admin.id)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'a')", legacy, spaceA)
		before := count()
		res := f.call(member, http.MethodPost, dependencyPath(a1), map[string]any{"predecessor_id": legacy, "type": "FS", "lag": 0})
		if res.status == http.StatusCreated || count() != before {
			t.Fatalf("an unresolved item must not link (status %d)", res.status)
		}
	})
	t.Run("DR-05 a legacy cross-owner link can be removed but not edited", func(t *testing.T) {
		f := f.on(t)
		legacy := f.queryID("INSERT INTO item_dependencies(predecessor_id,successor_id,type,lag) VALUES($1,$2,'FS',0) RETURNING id::text", in1, a2)
		f.want(f.call(member, http.MethodPatch, "/dependencies/"+legacy, map[string]any{"type": "SS"}, "If-Match", `"1"`),
			http.StatusUnprocessableEntity, "edit a cross-owner link")
		f.want(f.call(member, http.MethodDelete, "/dependencies/"+legacy, nil), http.StatusNoContent, "remove a cross-owner link")
	})
	t.Run("DR-06 scheduling moves a compatible successor and never a legacy cross-owner one", func(t *testing.T) {
		f := f.on(t)
		p := clientItem(spaceA, clientA, "Pred", "2026-01-09", "2026-01-09")
		s := clientItem(spaceA, clientA, "Succ", "2026-01-13", "2026-01-13")
		addSchedulingDependency(f, member, p, s, "FS", 0)
		moved := f.call(member, http.MethodPatch, "/items/"+p, map[string]any{"start_date": "2026-01-14", "due_date": "2026-01-14"}, "If-Match", `"1"`)
		f.want(moved, http.StatusOK, "move a same-owner predecessor")
		wantScheduledChanges(t, moved, scheduledChange{s, "2026-01-15", "2026-01-15", 2})

		legacyP := scheduledItem(f, member, internal, "Legacy pred", "2026-01-09", "2026-01-09", 1)
		legacyS := clientItem(spaceA, clientA, "Legacy succ", "2026-01-13", "2026-01-13")
		f.exec("INSERT INTO item_dependencies(predecessor_id,successor_id,type,lag) VALUES($1,$2,'FS',0)", legacyP, legacyS)
		res := f.call(member, http.MethodPatch, "/items/"+legacyP, map[string]any{"start_date": "2026-01-14", "due_date": "2026-01-14"}, "If-Match", `"1"`)
		f.want(res, http.StatusOK, "move a predecessor across owners")
		wantScheduledChanges(t, res)
		if got := f.queryID("SELECT due_date::text FROM items WHERE id=$1", legacyS); got != "2026-01-13" {
			t.Fatalf("a legacy cross-owner link moved its successor to %s", got)
		}
	})
}

// B3-09: a blueprint copy takes its destination's owner, drops non-members of a client destination, and is refused whole when
// the destination is archived, unresolved, or spans two owners.
func TestBlueprintCopyTakesDestinationOwner(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "BP-A")["id"].(string)
	archived := f.createClient(admin, "BP-ARCH")["id"].(string)
	planner := f.person("regular")
	outsider := f.person("regular")
	f.addMember(admin, client, planner)
	f.addMember(admin, archived, planner)
	library := f.internalSpace("BP library")
	clientSpace := f.scopedSpace("BP client", "client", client)
	archivedSpace := f.scopedSpace("BP archived", "client", archived)
	unresolved := f.unresolvedSpace("BP unresolved")
	mixed := f.internalSpace("BP mixed")
	for _, s := range []string{library, clientSpace, archivedSpace, mixed} {
		f.share(s, "user", planner.id, "full")
	}
	source := f.itemIn(library, "internal", "", planner.id, "Source")
	f.want(f.call(planner, http.MethodPatch, "/items/"+source, map[string]any{"assignee_ids": []string{planner.id, outsider.id}}, "If-Match", `"1"`),
		http.StatusOK, "assign a member and an outsider")
	saved := f.call(planner, http.MethodPost, "/spaces/"+library+"/blueprints", map[string]any{"name": "Kickoff", "source_item_id": source})
	f.want(saved, http.StatusCreated, "save blueprint")
	bp := saved.body["id"].(string)
	use := func(space, title string) reply {
		return f.call(planner, http.MethodPost, "/blueprints/"+bp+"/items", map[string]any{"title": title, "locations": []map[string]any{{"space_id": space}}})
	}
	rootsIn := func(space, title string) string {
		return f.queryID("SELECT count(*)::text FROM item_locations l JOIN items i ON i.id=l.item_id WHERE l.space_id=$1 AND i.title=$2", space, title)
	}

	t.Run("BP-01 copy into a client Space keeps only the Customer's members", func(t *testing.T) {
		f := f.on(t)
		res := use(clientSpace, "Copied")
		f.want(res, http.StatusCreated, "copy into client Space")
		id := res.body["id"].(string)
		if scope := f.queryID("SELECT scope_state FROM items WHERE id=$1", id); scope != "client" {
			t.Fatalf("copy takes the destination scope, got %s", scope)
		}
		if n := f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1 AND user_id=$2", id, outsider.id); n != "0" {
			t.Fatalf("an outsider was copied onto a client item")
		}
		if n := f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1 AND user_id=$2", id, planner.id); n != "1" {
			t.Fatalf("a member assignee was dropped: %s", n)
		}
	})
	t.Run("BP-02 an archived Customer refuses the copy and writes nothing", func(t *testing.T) {
		f := f.on(t)
		f.want(f.call(admin, http.MethodPost, "/clients/"+archived+"/archive", nil), http.StatusOK, "archive")
		f.want(use(archivedSpace, "Late"), http.StatusConflict, "archived Customer (customer_archived)")
		if got := rootsIn(archivedSpace, "Late"); got != "0" {
			t.Fatalf("a refused copy wrote %s items", got)
		}
	})
	t.Run("BP-03 an unresolved destination is refused", func(t *testing.T) {
		f := f.on(t)
		f.want(use(unresolved, "Unknown"), http.StatusUnprocessableEntity, "unresolved destination")
		if got := rootsIn(unresolved, "Unknown"); got != "0" {
			t.Fatalf("a refused copy wrote %s items", got)
		}
	})
	t.Run("BP-04 a copy across two owners is refused as a whole", func(t *testing.T) {
		f := f.on(t)
		res := f.call(planner, http.MethodPost, "/blueprints/"+bp+"/items", map[string]any{
			"title": "Split", "locations": []map[string]any{{"space_id": mixed}, {"space_id": clientSpace}},
		})
		f.want(res, http.StatusUnprocessableEntity, "two owners in one copy")
		if a, b := rootsIn(mixed, "Split"), rootsIn(clientSpace, "Split"); a != "0" || b != "0" {
			t.Fatalf("a refused copy wrote items: internal %s, client %s", a, b)
		}
	})
}

// B3-10: a relation field links items of one owner only. A target of another owner reads as unavailable, like a hidden item.
func TestRelationTargetsShareOwner(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	client := f.createClient(admin, "RF-A")["id"].(string)
	member := f.person("regular")
	f.addMember(admin, client, member)
	internal := f.internalSpace("RF internal")
	clientSpace := f.scopedSpace("RF client", "client", client)
	f.share(internal, "user", admin.id, "full")
	f.share(internal, "user", member.id, "full")
	f.share(clientSpace, "user", member.id, "full")
	field := f.call(admin, http.MethodPost, "/spaces/"+internal+"/custom-fields", map[string]any{"name": "Blocks", "type": "relation"})
	f.want(field, http.StatusCreated, "relation field")
	fid := field.body["id"].(string)
	source := f.itemIn(internal, "internal", "", admin.id, "Source")
	sameOwner := f.itemIn(internal, "internal", "", admin.id, "Same owner")
	clientItem := f.itemIn(clientSpace, "client", client, member.id, "Client item")
	set := func(target string) reply {
		return f.call(member, http.MethodPatch, "/items/"+source, map[string]any{"fields": map[string]any{fid: []string{target}}}, "If-Match", `"1"`)
	}

	f.want(set(clientItem), http.StatusUnprocessableEntity, "relate an internal item to a client item")
	if n := f.queryID("SELECT count(*)::text FROM item_relations WHERE from_item_id=$1", source); n != "0" {
		t.Fatalf("a refused relation was written: %s rows", n)
	}
	f.want(set(sameOwner), http.StatusOK, "relate two internal items")
}
