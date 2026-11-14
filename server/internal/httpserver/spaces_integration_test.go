package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/httpserver"
	mailqueue "github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// The capability tables of docs/ARCHITECTURE.md §6.1, typed in by hand and independent of package policy.
var (
	docRoleCaps = map[string][]string{
		"read":    {"view"},
		"limited": {"view", "comment", "change_status", "attach"},
		"editor":  {"view", "comment", "change_status", "attach", "edit", "create"},
		"full":    {"view", "comment", "change_status", "attach", "edit", "create", "delete", "share"},
	}
	docCeilings = map[string][]string{
		"admin":        docRoleCaps["full"],
		"regular":      docRoleCaps["full"],
		"external":     docRoleCaps["editor"],
		"collaborator": docRoleCaps["limited"],
		"viewer":       {"view", "comment"},
	}
	canonicalOrder = docRoleCaps["full"]
)

// expectedCaps is the document's rule: union of the roles, cut by the ceiling, in canonical order.
func expectedCaps(userType string, roles ...string) []string {
	have := map[string]bool{}
	for _, r := range roles {
		for _, c := range docRoleCaps[r] {
			have[c] = true
		}
	}
	allowed := map[string]bool{}
	for _, c := range docCeilings[userType] {
		allowed[c] = true
	}
	out := []string{}
	for _, c := range canonicalOrder {
		if have[c] && allowed[c] {
			out = append(out, c)
		}
	}
	return out
}

// testTwoFactorKeys are the 2FA keys of fixtures; tests of key handling replace them before creating a fixture.
var testTwoFactorKeys = twofactor.Keys{Current: bytes.Repeat([]byte{7}, 32)}

type person struct {
	id, userType string
	cookie       *http.Cookie
}

type fixture struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	srv  *httptest.Server
	n    int
	// files is the directory attachments are stored in.
	files string
	queue *mailqueue.Queue
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return newFixtureWithSeats(t, 1000)
}

func newFixtureWithSeats(t *testing.T, maxSeats int) *fixture {
	t.Helper()
	return newFixtureWith(t, maxSeats, nil)
}

func newFixtureWith(t *testing.T, maxSeats int, tweak func(*config.Config)) *fixture {
	t.Helper()
	dbURL := testdb.New(t)
	registerer, gatherer := prometheus.DefaultRegisterer, prometheus.DefaultGatherer
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registry, registry
	t.Cleanup(func() { prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registerer, gatherer })
	ctx := context.Background()
	pool, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	files := t.TempDir()
	cfg := config.Config{DatabaseURL: dbURL, PublicURL: publicURL, MaxSeats: maxSeats, TrustProxy: true, FilesDir: files, MaxUploadMB: 1, TwoFactorKey: testTwoFactorKeys.Current, TwoFactorKeyPrevious: testTwoFactorKeys.Previous, SSOKey: bytes.Repeat([]byte{5}, 32)}
	if tweak != nil {
		tweak(&cfg)
	}
	queue, err := mailqueue.New(pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bootstrap.Run(ctx, pool, "Acme", "boot@example.com", func(context.Context, pgx.Tx, string, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpserver.New(pool, cfg, queue))
	t.Cleanup(srv.Close)
	return &fixture{t: t, ctx: ctx, pool: pool, srv: srv, files: files, queue: queue}
}

func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatalf("%s: %v", sql, err)
	}
}

func (f *fixture) queryID(sql string, args ...any) string {
	f.t.Helper()
	var id string
	if err := f.pool.QueryRow(f.ctx, sql, args...).Scan(&id); err != nil {
		f.t.Fatalf("%s: %v", sql, err)
	}
	return id
}

// person creates an active user of the given type with a Personal space and a live session.
func (f *fixture) person(userType string) *person {
	f.t.Helper()
	f.n++
	email := fmt.Sprintf("%s%d@example.com", userType, f.n)
	id := f.queryID("INSERT INTO users(email,first_name,last_name,password_hash,user_type,status) VALUES ($1,'T','User','x',$2::user_type,'active') RETURNING id::text", email, userType)
	f.exec("INSERT INTO spaces(name,kind,owner_id) VALUES ('Personal','personal',$1)", id)
	token, hash, _ := security.Token()
	f.exec("INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) VALUES ($1,$2,now()+interval '1 day',now()+interval '2 days')", hash, id)
	return &person{id: id, userType: userType, cookie: &http.Cookie{Name: "pms_session", Value: token}}
}

func (f *fixture) space(name string) string {
	f.t.Helper()
	return f.queryID("INSERT INTO spaces(name,kind) VALUES ($1,'shared') RETURNING id::text", name)
}

func (f *fixture) group(name string, members ...*person) string {
	f.t.Helper()
	id := f.queryID("INSERT INTO groups(name) VALUES ($1) RETURNING id::text", name)
	for _, m := range members {
		f.exec("INSERT INTO group_members(group_id,user_id) VALUES ($1,$2)", id, m.id)
	}
	return id
}

func (f *fixture) share(spaceID, principalType, principalID, role string) {
	f.t.Helper()
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,$2::principal_type,$3,$4::share_role)", spaceID, principalType, principalID, role)
}

type reply struct {
	status int
	body   map[string]any
	header http.Header
}

func (f *fixture) call(p *person, method, path string, body any, headers ...string) reply {
	f.t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, f.srv.URL+"/api/v1"+path, rdr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", publicURL)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	if p != nil {
		req.AddCookie(p.cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	out := reply{status: resp.StatusCode, header: resp.Header}
	_ = json.NewDecoder(resp.Body).Decode(&out.body)
	return out
}

func (r reply) items() []map[string]any {
	list, _ := r.body["items"].([]any)
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		out = append(out, v.(map[string]any))
	}
	return out
}

func (r reply) caps() []string {
	list, _ := r.body["my_capabilities"].([]any)
	out := []string{}
	for _, v := range list {
		out = append(out, v.(string))
	}
	return out
}

func (f *fixture) want(r reply, status int, what string) {
	f.t.Helper()
	if r.status != status {
		f.t.Fatalf("%s: status %d, want %d (%v)", what, r.status, status, r.body)
	}
}

func idsOf(items []map[string]any) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it["id"].(string))
	}
	return out
}

// TestAuthorizationMatrix is S1.1.4 for Spaces: every user type × share role × path (direct share, share through a
// group) is checked against the tables of the document, through the real endpoints. The paths "through an ancestor" and
// "through a second location" need items and arrive with S1.2.2.
func TestAuthorizationMatrix(t *testing.T) {
	f := newFixture(t)
	types := []string{"admin", "regular", "external", "collaborator", "viewer"}
	roles := []string{"read", "limited", "editor", "full"}
	for _, ut := range types {
		for _, role := range roles {
			for _, path := range []string{"direct", "group"} {
				name := fmt.Sprintf("%s/%s/%s", ut, role, path)
				t.Run(name, func(t *testing.T) {
					f.t = t
					p := f.person(ut)
					space := f.space(name)
					if path == "direct" {
						f.share(space, "user", p.id, role)
					} else {
						f.share(space, "group", f.group(name, p), role)
					}
					want := expectedCaps(ut, role)
					if ut == "admin" {
						want = docRoleCaps["full"] // D-17: admin is full in every shared space
					}
					got := f.call(p, "GET", "/spaces/"+space, nil)
					f.want(got, 200, "GET space")
					if !reflect.DeepEqual(got.caps(), want) {
						t.Fatalf("my_capabilities = %v, want %v", got.caps(), want)
					}
					has := func(c string) bool {
						for _, w := range want {
							if w == c {
								return true
							}
						}
						return false
					}
					status := func(allowed bool) int {
						if allowed {
							return 200
						}
						return 403
					}
					patch := f.call(p, "PATCH", "/spaces/"+space, map[string]any{"description": "x"}, "If-Match", `"1"`)
					f.want(patch, status(has("edit")), "PATCH (edit)")
					list := f.call(p, "GET", "/spaces/"+space+"/shares", nil)
					f.want(list, 200, "GET shares (view)")
					current := []map[string]any{}
					for _, row := range list.items() { // a PUT carries ShareInput only: principal_type, principal_id, role
						current = append(current, map[string]any{"principal_type": row["principal_type"], "principal_id": row["principal_id"], "role": row["role"]})
					}
					put := f.call(p, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": current}, "If-Match", list.header.Get("ETag"))
					f.want(put, status(has("share")), "PUT shares (share)")
					del := f.call(p, "DELETE", "/spaces/"+space, nil)
					if has("delete") {
						f.want(del, 204, "DELETE (delete)")
						f.want(f.call(p, "GET", "/spaces/"+space, nil), 404, "GET after delete")
					} else {
						f.want(del, 403, "DELETE (delete)")
					}
				})
			}
		}
	}
}

func TestSpaceAccessRules(t *testing.T) {
	f := newFixture(t)
	owner, other, admin := f.person("regular"), f.person("regular"), f.person("admin")

	// create: creator gets full access and the first version
	created := f.call(owner, "POST", "/spaces", map[string]any{"name": "  Marketing  ", "description": "Plans"})
	f.want(created, 201, "create")
	space := created.body["id"].(string)
	if created.header.Get("ETag") != `"1"` || created.body["name"] != "Marketing" || created.body["kind"] != "shared" || !reflect.DeepEqual(created.caps(), docRoleCaps["full"]) {
		t.Fatalf("created space: %v etag=%q", created.body, created.header.Get("ETag"))
	}
	for _, ut := range []string{"external", "collaborator", "viewer"} {
		f.want(f.call(f.person(ut), "POST", "/spaces", map[string]any{"name": "Nope"}), 403, ut+" cannot create")
	}
	for _, bad := range []map[string]any{{"name": ""}, {"name": "   "}, {"name": strings.Repeat("n", 121)}, {"name": "ok", "description": strings.Repeat("d", 1001)}} {
		f.want(f.call(owner, "POST", "/spaces", bad), 422, fmt.Sprint("create with ", len(fmt.Sprint(bad))))
	}

	// invisible to a user without a share: every route says 404, never 403
	for _, c := range []struct{ method, path string }{
		{"GET", "/spaces/" + space}, {"PATCH", "/spaces/" + space}, {"DELETE", "/spaces/" + space},
		{"GET", "/spaces/" + space + "/shares"}, {"PUT", "/spaces/" + space + "/shares"},
	} {
		var body any
		if c.method == "PATCH" {
			body = map[string]any{"name": "x"}
		}
		if c.method == "PUT" {
			body = map[string]any{"shares": []any{}}
		}
		f.want(f.call(other, c.method, c.path, body, "If-Match", `"1"`), 404, "no share: "+c.method+" "+c.path)
	}
	f.want(f.call(other, "GET", "/spaces/00000000-0000-0000-0000-000000000000", nil), 404, "unknown id")
	f.want(f.call(other, "GET", "/spaces/not-a-uuid", nil), 404, "malformed id")
	f.want(f.call(nil, "GET", "/spaces", nil), 401, "no session")

	// admin: full access to the shared space without any share
	f.want(f.call(admin, "GET", "/spaces/"+space, nil), 200, "admin reads a shared space")

	// personal space: private even from admins; cannot be changed, deleted or shared
	var ownerPersonal string
	for _, it := range f.call(owner, "GET", "/spaces", nil).items() {
		if it["kind"] == "personal" {
			ownerPersonal = it["id"].(string)
		}
	}
	if ownerPersonal == "" {
		t.Fatal("owner has no personal space in the list")
	}
	f.want(f.call(admin, "GET", "/spaces/"+ownerPersonal, nil), 404, "admin cannot see someone's personal space")
	f.want(f.call(other, "GET", "/spaces/"+ownerPersonal, nil), 404, "other cannot see a personal space")
	for _, id := range idsOf(f.call(admin, "GET", "/spaces", nil).items()) {
		if id == ownerPersonal {
			t.Fatal("a personal space leaked into an admin's list")
		}
	}
	f.want(f.call(owner, "GET", "/spaces/"+ownerPersonal, nil), 200, "owner reads personal")
	f.want(f.call(owner, "PATCH", "/spaces/"+ownerPersonal, map[string]any{"name": "Mine"}, "If-Match", `"1"`), 403, "personal cannot be renamed")
	f.want(f.call(owner, "DELETE", "/spaces/"+ownerPersonal, nil), 403, "personal cannot be deleted")
	f.want(f.call(owner, "PUT", "/spaces/"+ownerPersonal+"/shares", map[string]any{"shares": []any{}}, "If-Match", `"1"`), 403, "personal cannot be shared")
	if got := f.call(owner, "GET", "/spaces/"+ownerPersonal+"/shares", nil); got.status != 200 || len(got.items()) != 0 {
		t.Fatalf("personal shares: %d %v", got.status, got.body)
	}
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,'user',$2,'read')", f.space("tmp"), other.id) // sanity: trigger allows shared spaces
	if _, err := f.pool.Exec(f.ctx, "INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,'user',$2,'read')", ownerPersonal, other.id); err == nil {
		t.Fatal("the database accepted a share on a personal space")
	}

	// list: personal first, then shared by name; only what the caller may see
	f.exec("UPDATE spaces SET name='Zebra' WHERE id=$1", f.space("tmp2"))
	if names := f.call(owner, "GET", "/spaces", nil).items(); len(names) != 2 || names[0]["kind"] != "personal" || names[1]["name"] != "Marketing" {
		t.Fatalf("owner list: %v", names)
	}

	// updates: If-Match is mandatory and versioned
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"name": "M"}), 428, "missing If-Match")
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"name": "M"}, "If-Match", `"abc"`), 422, "malformed If-Match")
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"name": "M"}, "If-Match", `"7"`), 409, "stale If-Match")
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"nope": 1}, "If-Match", `"1"`), 422, "unknown field")
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"name": ""}, "If-Match", `"1"`), 422, "empty name")
	ok := f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"name": "Growth", "description": nil}, "If-Match", `"1"`)
	f.want(ok, 200, "update")
	if ok.body["name"] != "Growth" || ok.body["description"] != nil || ok.header.Get("ETag") != `"2"` {
		t.Fatalf("update result: %v etag=%q", ok.body, ok.header.Get("ETag"))
	}
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"name": "Again"}, "If-Match", `"1"`), 409, "the old version no longer applies")

	// deleting hides the space from everyone, admins included
	f.want(f.call(owner, "DELETE", "/spaces/"+space, nil), 204, "delete")
	for _, p := range []*person{owner, admin} {
		f.want(f.call(p, "GET", "/spaces/"+space, nil), 404, "deleted space is gone")
	}
}

func TestSharesAreUnionedAndCappedAndValidated(t *testing.T) {
	f := newFixture(t)
	owner, member, viewer, ext := f.person("regular"), f.person("regular"), f.person("viewer"), f.person("external")
	space := f.call(owner, "POST", "/spaces", map[string]any{"name": "Team"}).body["id"].(string)
	staff := f.group("Staff", member, viewer)
	deactivated := f.person("regular")
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", deactivated.id)

	put := func(shares ...map[string]any) reply {
		etag := f.call(owner, "GET", "/spaces/"+space+"/shares", nil).header.Get("ETag") // the list as the dialog loaded it
		return f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": shares}, "If-Match", etag)
	}
	sh := func(pt, id, role string) map[string]any {
		return map[string]any{"principal_type": pt, "principal_id": id, "role": role}
	}
	ownerFull := sh("user", owner.id, "full")
	got := put(ownerFull, sh("user", member.id, "read"), sh("group", staff, "editor"), sh("user", viewer.id, "full"), sh("user", ext.id, "full"))
	f.want(got, 200, "replace shares")
	if len(got.items()) != 5 {
		t.Fatalf("share list: %v", got.items())
	}
	// union of a direct read and an editor share through a group
	if caps := f.call(member, "GET", "/spaces/"+space, nil).caps(); !reflect.DeepEqual(caps, expectedCaps("regular", "read", "editor")) {
		t.Fatalf("member caps %v", caps)
	}
	// ceilings cut a full share down
	if caps := f.call(viewer, "GET", "/spaces/"+space, nil).caps(); !reflect.DeepEqual(caps, []string{"view", "comment"}) {
		t.Fatalf("viewer with a full share: %v", caps)
	}
	if caps := f.call(ext, "GET", "/spaces/"+space, nil).caps(); !reflect.DeepEqual(caps, docRoleCaps["editor"]) {
		t.Fatalf("external with a full share: %v", caps)
	}
	// principal names come back for the dialog; nothing is inherited for a Space
	for _, it := range got.items() {
		if it["principal_name"] == "" || it["inherited_from"] != nil {
			t.Fatalf("share row %v", it)
		}
	}
	// removing the group share drops the editor part of the union
	f.want(put(ownerFull, sh("user", member.id, "read")), 200, "shrink")
	if caps := f.call(member, "GET", "/spaces/"+space, nil).caps(); !reflect.DeepEqual(caps, []string{"view"}) {
		t.Fatalf("member after shrink %v", caps)
	}
	f.want(f.call(viewer, "GET", "/spaces/"+space, nil), 404, "removed share means the space disappears")

	// validation
	f.want(put(ownerFull, sh("user", member.id, "read"), sh("user", member.id, "full")), 422, "duplicate principal")
	f.want(put(ownerFull, sh("user", "00000000-0000-0000-0000-000000000000", "read")), 422, "unknown user")
	f.want(put(ownerFull, sh("group", "00000000-0000-0000-0000-000000000000", "read")), 422, "unknown group")
	f.want(put(ownerFull, sh("user", deactivated.id, "read")), 422, "deactivated user")
	f.want(put(ownerFull, sh("user", member.id, "boss")), 422, "unknown role")
	f.want(put(ownerFull, sh("team", member.id, "read")), 422, "unknown principal type")
	f.want(put(ownerFull, sh("user", "nope", "read")), 422, "malformed principal id")
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{}, "If-Match", `"1"`), 422, "shares is required")
	if still := f.call(member, "GET", "/spaces/"+space, nil); still.status != 200 {
		t.Fatalf("a rejected replace must change nothing: %d", still.status)
	}

	// who may edit the list: a reader may look, not change
	f.want(f.call(member, "GET", "/spaces/"+space+"/shares", nil), 200, "reader can list")
	f.want(f.call(member, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []any{}}, "If-Match", `"1"`), 403, "reader cannot change shares")

	// creating with shares keeps the creator as full
	both := f.call(owner, "POST", "/spaces", map[string]any{"name": "Joint", "shares": []any{sh("user", member.id, "limited")}})
	f.want(both, 201, "create with shares")
	rows := f.call(owner, "GET", "/spaces/"+both.body["id"].(string)+"/shares", nil).items()
	if len(rows) != 2 {
		t.Fatalf("create with shares: %v", rows)
	}
}

func TestDirectoryListing(t *testing.T) {
	f := newFixture(t)
	reg, admin, ext := f.person("regular"), f.person("admin"), f.person("external")
	f.exec("UPDATE users SET first_name='Ada', last_name='Lovelace' WHERE id=$1", reg.id)
	f.exec("INSERT INTO users(email,user_type,status) VALUES ('invited@example.com','regular','invited')")
	f.group("Staff", reg)

	got := f.call(reg, "GET", "/users?q=lovel", nil)
	f.want(got, 200, "search")
	if rows := got.items(); len(rows) != 1 || rows[0]["id"] != reg.id {
		t.Fatalf("search result %v", rows)
	}
	for _, it := range f.call(reg, "GET", "/users", nil).items() {
		if it["status"] != "active" {
			t.Fatalf("a regular user must only see active users: %v", it)
		}
	}
	seenInvited := false
	for _, it := range f.call(admin, "GET", "/users", nil).items() {
		if it["status"] == "invited" {
			seenInvited = true
		}
	}
	if !seenInvited {
		t.Fatal("an admin must see invited users too")
	}
	// paging: limit 1 gives a cursor that leads to a different user, wildcard characters in q are literal
	first := f.call(admin, "GET", "/users?limit=1", nil)
	cursor, _ := first.body["next_cursor"].(string)
	if cursor == "" || len(first.items()) != 1 {
		t.Fatalf("first page %v", first.body)
	}
	second := f.call(admin, "GET", "/users?limit=1&cursor="+cursor, nil)
	if len(second.items()) != 1 || second.items()[0]["id"] == first.items()[0]["id"] {
		t.Fatalf("second page %v", second.body)
	}
	if n := len(f.call(admin, "GET", "/users?q=%25", nil).items()); n != 0 {
		t.Fatalf("a literal %% must not match everything, got %d users", n)
	}
	f.want(f.call(admin, "GET", "/users?limit=0", nil), 422, "bad limit")
	f.want(f.call(admin, "GET", "/users?cursor=zzz", nil), 422, "bad cursor")
	// partner organisations cannot read the staff directory
	if n := len(f.call(ext, "GET", "/users", nil).items()); n != 0 {
		t.Fatalf("an external user saw %d users", n)
	}
	if n := len(f.call(ext, "GET", "/groups", nil).items()); n != 0 {
		t.Fatalf("an external user saw %d groups", n)
	}
	if groups := f.call(reg, "GET", "/groups", nil).items(); len(groups) != 1 || groups[0]["name"] != "Staff" {
		t.Fatalf("groups %v", groups)
	}
}

// The review findings of S1.1.x: group membership is honoured, stale share lists lose, and edge cases stay safe.
func TestSharesReviewFindings(t *testing.T) {
	f := newFixture(t)
	owner, member, outsider, ext := f.person("regular"), f.person("regular"), f.person("regular"), f.person("external")
	space := f.call(owner, "POST", "/spaces", map[string]any{"name": "Team"}).body["id"].(string)
	staff := f.group("Staff", member)
	f.share(space, "group", staff, "editor")
	f.share(space, "user", ext.id, "read")

	// a user who is NOT in the group must not get the group's access (404), a member does
	f.want(f.call(outsider, "GET", "/spaces/"+space, nil), 404, "outsider of a group share")
	f.want(f.call(member, "GET", "/spaces/"+space, nil), 200, "member of a group share")
	for _, id := range idsOf(f.call(outsider, "GET", "/spaces", nil).items()) {
		if id == space {
			t.Fatal("a group-shared space leaked into a non-member's list")
		}
	}

	// the share list carries a version; a stale list is refused so a revoked access cannot come back
	load := f.call(owner, "GET", "/spaces/"+space+"/shares", nil)
	staleETag := load.header.Get("ETag")
	rowsOf := func(r reply, drop string) []any {
		out := []any{}
		for _, row := range r.items() {
			if row["principal_id"] == drop {
				continue
			}
			out = append(out, map[string]any{"principal_type": row["principal_type"], "principal_id": row["principal_id"], "role": row["role"]})
		}
		return out
	}
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []any{}}), 428, "PUT without If-Match")
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": rowsOf(load, ext.id)}, "If-Match", staleETag), 200, "revoke the external user")
	// someone else still holds the list loaded before the revoke and adds a person: it must not undo the revoke
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": rowsOf(load, "")}, "If-Match", staleETag), 409, "stale list")
	f.want(f.call(ext, "GET", "/spaces/"+space, nil), 404, "the revoked user stays revoked")
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []any{}}, "If-Match", `"3000000000"`), 409, "version beyond int range")
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []any{}}, "If-Match", `*`), 422, "If-Match star")

	// a rejected replace is atomic: a request that drops a share and also holds a bad principal changes nothing
	fresh := f.call(owner, "GET", "/spaces/"+space+"/shares", nil)
	bad := append(rowsOf(fresh, staff), map[string]any{"principal_type": "user", "principal_id": "00000000-0000-0000-0000-000000000000", "role": "read"})
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": bad}, "If-Match", fresh.header.Get("ETag")), 422, "partly bad replace")
	f.want(f.call(member, "GET", "/spaces/"+space, nil), 200, "the dropped share must still be there after a rejected replace")

	// edge cases
	etag := f.call(owner, "GET", "/spaces/"+space, nil).header.Get("ETag")
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{}, "If-Match", etag), 422, "empty patch")
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"name": "x"}, "If-Match", `"3000000000"`), 409, "version beyond int range on PATCH")
	lower := f.call(owner, "POST", "/spaces", map[string]any{"name": "Mine", "shares": []any{map[string]any{"principal_type": "user", "principal_id": owner.id, "role": "read"}}})
	f.want(lower, 201, "create listing the creator as reader")
	if !reflect.DeepEqual(lower.caps(), docRoleCaps["full"]) {
		t.Fatalf("the creator must stay full: %v", lower.caps())
	}

	// deleted spaces answer 404 for every write
	f.want(f.call(owner, "DELETE", "/spaces/"+space, nil), 204, "delete")
	f.want(f.call(owner, "PATCH", "/spaces/"+space, map[string]any{"name": "y"}, "If-Match", etag), 404, "PATCH a deleted space")
	f.want(f.call(owner, "PUT", "/spaces/"+space+"/shares", map[string]any{"shares": []any{}}, "If-Match", etag), 404, "PUT shares of a deleted space")
	f.want(f.call(owner, "DELETE", "/spaces/"+space, nil), 404, "DELETE a deleted space")

	// a deactivated user's live session stops working at once
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", outsider.id)
	f.want(f.call(outsider, "GET", "/spaces", nil), 401, "deactivated user")
}
