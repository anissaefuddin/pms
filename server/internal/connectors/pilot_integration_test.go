package connectors_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/connectors"
	"github.com/anissaefuddin/pms/server/internal/connectors/gcal"
	"github.com/anissaefuddin/pms/server/internal/connectors/github"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
)

type pilot struct {
	t     *testing.T
	s     *connectors.Service
	admin string
	space string
	gh    *fakeGitHub
	gcal  *fakeGoogle
	ghSrv *httptest.Server
	gSrv  *httptest.Server
}

func newPilot(t *testing.T) *pilot {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	exec := func(sql string, args ...any) {
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	p := &pilot{t: t, gh: &fakeGitHub{issues: map[int]*ghIssue{}}, gcal: &fakeGoogle{events: map[string]map[string]any{}}}
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,user_type,first_name,last_name,password_hash,status) VALUES ('a@example.com','admin','A','A','x','active') RETURNING id::text`).Scan(&p.admin); err != nil {
		t.Fatal(err)
	}
	var wf string
	if err := pool.QueryRow(ctx, `INSERT INTO workflows(name,is_default) VALUES ('Default',true) RETURNING id::text`).Scan(&wf); err != nil {
		t.Fatal(err)
	}
	for i, st := range [][2]string{{"New", "active"}, {"Done", "completed"}} {
		exec(`INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,$2,$3::status_group,'slate',$4)`, wf, st[0], st[1], i)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO spaces(name,kind) VALUES ('Repo','shared') RETURNING id::text`).Scan(&p.space); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	p.ghSrv = httptest.NewServer(p.gh)
	p.gSrv = httptest.NewServer(p.gcal)
	t.Cleanup(p.ghSrv.Close)
	t.Cleanup(p.gSrv.Close)
	reg := &connectors.Registry{}
	reg.Register(&github.Connector{Base: p.ghSrv.URL, HTTP: connectors.DefaultHTTP})
	reg.Register(&gcal.Connector{Base: p.gSrv.URL, TokenURL: p.gSrv.URL + "/token", HTTP: connectors.DefaultHTTP})
	p.s = &connectors.Service{Pool: pool, Keys: twofactor.Keys{Current: key}, Registry: reg}
	return p
}

func (p *pilot) one(sql string, args ...any) (v string) {
	if err := p.s.Pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		p.t.Fatal(err)
	}
	return
}
func (p *pilot) exec(sql string, args ...any) {
	if _, err := p.s.Pool.Exec(context.Background(), sql, args...); err != nil {
		p.t.Fatal(err)
	}
}
func (p *pilot) installGH(cfg map[string]any) string {
	if cfg == nil {
		cfg = map[string]any{}
	}
	cfg["repo"], cfg["space_id"] = "o/r", p.space
	in, err := p.s.Install(context.Background(), p.admin, "github", "GH", cfg, map[string]string{"token": "tok"})
	if err != nil {
		p.t.Fatal(err)
	}
	return in.ID
}
func (p *pilot) installG() string {
	in, err := p.s.Install(context.Background(), p.admin, "google_calendar", "Cal", map[string]any{"calendar_id": "primary", "space_id": p.space}, map[string]string{"token": "tok"})
	if err != nil {
		p.t.Fatal(err)
	}
	return in.ID
}
func (p *pilot) sync(id string) connectors.SyncResult {
	r, err := p.s.Sync(context.Background(), id)
	if err != nil {
		p.t.Fatal(err)
	}
	return r
}
func (p *pilot) task(title string) string {
	id := p.one(`INSERT INTO items(type,title,created_by,status_id) VALUES('task',$1,$2,(SELECT id FROM statuses WHERE name='New')) RETURNING id::text`, title, p.admin)
	p.exec(`INSERT INTO item_locations(item_id,space_id,rank) VALUES($1,$2,$3)`, id, p.space, "a"+id[:8])
	return id
}
func (p *pilot) taskByTitle(title string) (id, body, status string) {
	p.t.Helper()
	id = p.one(`SELECT i.id::text FROM items i WHERE i.title=$1 AND i.deleted_at IS NULL`, title)
	return id, p.one(`SELECT description_text FROM items WHERE id=$1`, id), p.one(`SELECT s.name FROM items i JOIN statuses s ON s.id=i.status_id WHERE i.id=$1`, id)
}

// ---- fake GitHub ----

type ghIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      *string   `json:"body"`
	State     string    `json:"state"`
	UpdatedAt string    `json:"updated_at"`
	PR        *struct{} `json:"pull_request,omitempty"`
}

type fakeGitHub struct {
	mu     sync.Mutex
	issues map[int]*ghIssue
	next   int
	clock  int
	fail   int // when set, every request answers this status
	header map[string]string
	calls  int
	writes int
}

func (g *fakeGitHub) add(title, body, state string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	g.clock++
	b := body
	g.issues[g.next] = &ghIssue{Number: g.next, Title: title, Body: &b, State: state, UpdatedAt: fmt.Sprintf("t%04d", g.clock)}
	return g.next
}
func (g *fakeGitHub) edit(n int, f func(*ghIssue)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clock++
	f(g.issues[n])
	g.issues[n].UpdatedAt = fmt.Sprintf("t%04d", g.clock)
}

func (g *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	if r.Header.Get("Authorization") != "Bearer tok" {
		w.WriteHeader(401)
		return
	}
	if g.fail != 0 {
		for k, v := range g.header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(g.fail)
		_, _ = w.Write([]byte(`{"message":"nope"}`))
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/repos/o/r/issues")
	switch {
	case r.Method == "GET" && path == "":
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == 0 {
			page = 1
		}
		const size = 2 // small pages exercise pagination
		var all []*ghIssue
		for n := 1; n <= g.next; n++ {
			if i, ok := g.issues[n]; ok {
				all = append(all, i)
			}
		}
		lo, hi := (page-1)*size, page*size
		if lo > len(all) {
			lo = len(all)
		}
		if hi > len(all) {
			hi = len(all)
		}
		if hi < len(all) {
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/repos/o/r/issues?state=all&per_page=100&page=%d>; rel="next"`, r.Host, page+1))
		}
		_ = json.NewEncoder(w).Encode(all[lo:hi])
	case r.Method == "POST" && path == "":
		var in struct{ Title, Body string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		g.next++
		g.clock++
		g.writes++
		b := in.Body
		i := &ghIssue{Number: g.next, Title: in.Title, Body: &b, State: "open", UpdatedAt: fmt.Sprintf("t%04d", g.clock)}
		g.issues[g.next] = i
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(i)
	case r.Method == "PATCH":
		n, _ := strconv.Atoi(strings.TrimPrefix(path, "/"))
		i, ok := g.issues[n]
		if !ok {
			w.WriteHeader(404)
			return
		}
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		g.clock++
		g.writes++
		if v, ok := in["title"]; ok {
			i.Title = v
		}
		if v, ok := in["body"]; ok {
			i.Body = &v
		}
		if v, ok := in["state"]; ok {
			i.State = v
		}
		i.UpdatedAt = fmt.Sprintf("t%04d", g.clock)
		_ = json.NewEncoder(w).Encode(i)
	default:
		w.WriteHeader(404)
	}
}

// ---- fake Google Calendar ----

type fakeGoogle struct {
	mu      sync.Mutex
	events  map[string]map[string]any
	next    int
	log     []string
	fail    int
	failFor string // only requests whose body mentions this text fail with fail
	tokens  int
}

func (g *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if r.URL.Path == "/token" {
		g.tokens++
		_ = r.ParseForm()
		if r.Form.Get("refresh_token") != "rt" {
			w.WriteHeader(400)
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"fresh"}`))
		return
	}
	if a := r.Header.Get("Authorization"); a != "Bearer tok" && a != "Bearer fresh" {
		w.WriteHeader(401)
		return
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	if g.fail != 0 && (g.failFor == "" || strings.Contains(fmt.Sprint(body["summary"]), g.failFor)) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(g.fail)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/calendar/v3/calendars/primary/events")
	id = strings.TrimPrefix(id, "/")
	g.log = append(g.log, r.Method+" "+id)
	switch r.Method {
	case "POST":
		g.next++
		id = fmt.Sprintf("ev%d", g.next)
		body["id"] = id
		g.events[id] = body
		_ = json.NewEncoder(w).Encode(body)
	case "PATCH":
		ev, ok := g.events[id]
		if !ok {
			w.WriteHeader(404)
			return
		}
		for k, v := range body {
			ev[k] = v
		}
		_ = json.NewEncoder(w).Encode(ev)
	case "DELETE":
		if _, ok := g.events[id]; !ok {
			w.WriteHeader(410)
			return
		}
		delete(g.events, id)
		w.WriteHeader(204)
	}
}

func (g *fakeGoogle) writes() (n int) {
	for _, l := range g.log {
		if !strings.HasPrefix(l, "GET") {
			n++
		}
	}
	return
}

// ---- GitHub tests ----

func TestGitHubImportsOpenIssuesWithPaginationAndIsIdempotent(t *testing.T) {
	p := newPilot(t)
	p.gh.add("one", "body one\nline two", "open")
	p.gh.add("closed history", "", "closed")
	p.gh.add("two", "", "open")
	n := p.gh.add("a pull request", "", "open")
	p.gh.issues[n].PR = &struct{}{}
	p.gh.add("three", "x", "open")
	id := p.installGH(nil)
	if r := p.sync(id); r.Pulled != 3 {
		t.Fatalf("first sync: %+v", r)
	}
	if _, body, st := p.taskByTitle("one"); body != "body one\nline two" || st != "New" {
		t.Fatalf("imported task: %q %q", body, st)
	}
	if got := p.one(`SELECT count(*)::text FROM items WHERE title IN ('closed history','a pull request')`); got != "0" {
		t.Fatalf("closed issues and PRs must not be imported, got %s", got)
	}
	if r := p.sync(id); r.Pulled != 0 || r.Pushed != 0 || r.Conflicts != 0 || r.Unchanged != 3 {
		t.Fatalf("second sync must change nothing: %+v", r)
	}
	if got := p.one(`SELECT count(*)::text FROM items WHERE type='task'`); got != "3" {
		t.Fatalf("duplicates after re-sync: %s", got)
	}
}

func TestGitHubTwoWayStatusAndBody(t *testing.T) {
	p := newPilot(t)
	n := p.gh.add("bug", "details", "open")
	id := p.installGH(nil)
	p.sync(id)
	item, _, _ := p.taskByTitle("bug")
	// Remote closes and retitles -> the task completes.
	p.gh.edit(n, func(i *ghIssue) { i.State = "closed"; i.Title = "bug (fixed)" })
	if r := p.sync(id); r.Pulled != 1 {
		t.Fatalf("pull: %+v", r)
	}
	if _, _, st := p.taskByTitle("bug (fixed)"); st != "Done" {
		t.Fatalf("closed issue must complete the task, status %s", st)
	}
	// The task is reopened and rewritten in PMS -> the issue follows.
	p.exec(`UPDATE items SET status_id=(SELECT id FROM statuses WHERE name='New'),description_text='new text',version=version+1 WHERE id=$1`, item)
	if r := p.sync(id); r.Pushed != 1 {
		t.Fatalf("push: %+v", r)
	}
	if i := p.gh.issues[n]; i.State != "open" || *i.Body != "new text" {
		t.Fatalf("remote after push: %+v", i)
	}
	// Completing the task closes the issue.
	p.exec(`UPDATE items SET status_id=(SELECT id FROM statuses WHERE name='Done'),version=version+1 WHERE id=$1`, item)
	p.sync(id)
	if p.gh.issues[n].State != "closed" {
		t.Fatal("completed task must close the issue")
	}
}

func TestGitHubConflictNeverOverwritesSilently(t *testing.T) {
	p := newPilot(t)
	n := p.gh.add("shared", "", "open")
	id := p.installGH(nil)
	p.sync(id)
	item, _, _ := p.taskByTitle("shared")
	p.gh.edit(n, func(i *ghIssue) { i.Title = "from GitHub" })
	p.exec(`UPDATE items SET title='from PMS',version=version+1 WHERE id=$1`, item)
	writes := p.gh.writes
	if r := p.sync(id); r.Conflicts != 1 {
		t.Fatalf("want conflict: %+v", r)
	}
	if p.gh.issues[n].Title != "from GitHub" || p.gh.writes != writes || p.one(`SELECT title FROM items WHERE id=$1`, item) != "from PMS" {
		t.Fatal("a conflict must leave both sides untouched")
	}
	if r := p.sync(id); r.Pending != 1 || r.Conflicts != 0 {
		t.Fatalf("re-sync must not pile up conflicts: %+v", r)
	}
	list, _ := p.s.Conflicts(context.Background(), id, false)
	if len(list) != 1 || list[0].Differs != "title" {
		t.Fatalf("conflicts: %+v", list)
	}
	if err := p.s.Resolve(context.Background(), list[0].ID, "remote", p.admin); err != nil {
		t.Fatal(err)
	}
	if p.one(`SELECT title FROM items WHERE id=$1`, item) != "from GitHub" {
		t.Fatal("remote choice must update the task")
	}
}

func TestGitHubPushNewTasksOnlyWhenOptedIn(t *testing.T) {
	p := newPilot(t)
	p.task("local task")
	id := p.installGH(nil)
	p.sync(id)
	if len(p.gh.issues) != 0 {
		t.Fatal("tasks must not become issues unless push_new is set")
	}
	if _, err := p.s.Configure(context.Background(), id, nil, map[string]any{"repo": "o/r", "space_id": p.space, "push_new": true}, nil); err != nil {
		t.Fatal(err)
	}
	if r := p.sync(id); r.Pushed != 1 || len(p.gh.issues) != 1 {
		t.Fatalf("push_new: %+v issues=%d", r, len(p.gh.issues))
	}
	if r := p.sync(id); r.Pushed != 0 || len(p.gh.issues) != 1 {
		t.Fatalf("push_new must be idempotent: %+v", r)
	}
}

func TestGitHubErrorsAreIsolatedAndBackedOff(t *testing.T) {
	p := newPilot(t)
	p.gh.add("x", "", "open")
	bad := p.installGH(nil)
	p.gh.fail = 401
	if _, err := p.s.Sync(context.Background(), bad); err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("401 must explain the token: %v", err)
	}
	if p.one(`SELECT last_sync_ok::text FROM connector_installs WHERE id=$1`, bad) != "false" || p.one(`SELECT failures::text FROM connector_installs WHERE id=$1`, bad) != "1" {
		t.Fatal("failure not recorded")
	}
	if p.one(`SELECT (next_sync_at>now())::text FROM connector_installs WHERE id=$1`, bad) != "true" {
		t.Fatal("failure must push the next run out")
	}
	// 403 with an exhausted quota and 429 are rate limits that honour the remote's own wait.
	p.gh.fail, p.gh.header = 403, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": strconv.FormatInt(time.Now().Add(30*time.Minute).Unix(), 10)}
	var rl connectors.RateLimited
	if _, err := p.s.Sync(context.Background(), bad); !errors.As(err, &rl) || time.Until(rl.After) < 25*time.Minute {
		t.Fatalf("rate limit: %v", err)
	}
	if p.one(`SELECT (next_sync_at>now()+interval '25 minutes')::text FROM connector_installs WHERE id=$1`, bad) != "true" {
		t.Fatal("next run must wait for the reset")
	}
	p.gh.fail, p.gh.header = 429, nil
	if _, err := p.s.Sync(context.Background(), bad); !errors.As(err, &rl) {
		t.Fatalf("429: %v", err)
	}
	p.gh.fail = 403
	if _, err := p.s.Sync(context.Background(), bad); err == nil || errors.As(err, &rl) || !strings.Contains(err.Error(), "403") {
		t.Fatalf("plain 403 is a permission error, not a rate limit: %v", err)
	}
	// A broken install does not stop the others: RunDue keeps going and a healthy one syncs.
	p.gh.fail = 0
	p.exec(`UPDATE connector_installs SET next_sync_at=NULL`)
	good := p.installGH(nil) // a second install of the same repo: it imports the same open issue
	_ = good
	p.gh.fail = 500
	p.exec(`UPDATE connector_installs SET next_sync_at=NULL`)
	if ran := p.s.RunDue(context.Background()); ran != 2 {
		t.Fatalf("RunDue ran %d installs, want 2 despite failures", ran)
	}
	if ran := p.s.RunDue(context.Background()); ran != 0 {
		t.Fatalf("backed-off installs must wait, ran %d", ran)
	}
	p.gh.fail = 0
	p.exec(`UPDATE connector_installs SET next_sync_at=NULL`)
	if ran := p.s.RunDue(context.Background()); ran != 2 {
		t.Fatalf("recovery: ran %d", ran)
	}
	if p.one(`SELECT failures::text FROM connector_installs WHERE id=$1`, bad) != "0" {
		t.Fatal("success must reset the failure count")
	}
}

func TestGitHubWebhookMakesInstallDue(t *testing.T) {
	p := newPilot(t)
	id := p.installGH(nil)
	p.sync(id)
	if p.one(`SELECT (next_sync_at>now())::text FROM connector_installs WHERE id=$1`, id) != "true" {
		t.Fatal("precondition: next run is in the future")
	}
	if err := p.s.RequestSync(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if p.one(`SELECT (next_sync_at<=now())::text FROM connector_installs WHERE id=$1`, id) != "true" {
		t.Fatal("RequestSync must make the install due now")
	}
}

// ---- Google Calendar tests ----

func (p *pilot) due(item, start, due string) {
	p.exec(`UPDATE items SET start_date=NULLIF($2,'')::date,due_date=NULLIF($3,'')::date,version=version+1 WHERE id=$1`, item, start, due)
}

func TestGoogleOneWayLifecycle(t *testing.T) {
	p := newPilot(t)
	a, b, c := p.task("with range"), p.task("due only"), p.task("no dates")
	p.due(a, "2026-03-02", "2026-03-04")
	p.due(b, "", "2026-03-10")
	_ = c
	id := p.installG()
	if r := p.sync(id); r.Pushed != 2 || len(p.gcal.events) != 2 {
		t.Fatalf("create: %+v events=%d", r, len(p.gcal.events))
	}
	var ev map[string]any
	for _, e := range p.gcal.events {
		if e["summary"] == "with range" {
			ev = e
		}
	}
	if fmt.Sprint(ev["start"]) != "map[date:2026-03-02]" || fmt.Sprint(ev["end"]) != "map[date:2026-03-05]" {
		t.Fatalf("all-day range with exclusive end: %v", ev)
	}
	w := p.gcal.writes()
	if r := p.sync(id); r.Pushed != 0 || r.Unchanged != 2 || p.gcal.writes() != w {
		t.Fatalf("idempotent: %+v", r)
	}
	// An edit made in Google is never read back or written to the Item, and is left alone while the Item is unchanged.
	p.gcal.mu.Lock()
	ev["summary"] = "edited in Google"
	p.gcal.mu.Unlock()
	if r := p.sync(id); r.Pushed != 0 || p.gcal.writes() != w {
		t.Fatalf("Google edit must not be touched: %+v", r)
	}
	if p.one(`SELECT title FROM items WHERE id=$1`, a) != "with range" {
		t.Fatal("Google edit leaked into the Item")
	}
	// Both sides changed: the Item wins, the Google edit is replaced (one-way by design).
	p.exec(`UPDATE items SET title='renamed',version=version+1 WHERE id=$1`, a)
	if r := p.sync(id); r.Pushed != 1 || ev["summary"] != "renamed" {
		t.Fatalf("update: %+v %v", r, ev["summary"])
	}
	// Event deleted in Google, then the Item changes: it is created again, not an error.
	p.gcal.mu.Lock()
	for k := range p.gcal.events {
		delete(p.gcal.events, k)
	}
	p.gcal.mu.Unlock()
	p.exec(`UPDATE items SET title='renamed again',version=version+1 WHERE id=$1`, a)
	if r := p.sync(id); r.Pushed != 1 || len(p.gcal.events) != 1 {
		t.Fatalf("recreate: %+v events=%d", r, len(p.gcal.events))
	}
	// Dropping the due date, and deleting the Item, delete the events.
	p.due(a, "", "")
	p.exec(`UPDATE items SET deleted_at=now() WHERE id=$1`, b)
	p.sync(id)
	if len(p.gcal.events) != 0 || p.one(`SELECT count(*)::text FROM connector_links WHERE connector_id=$1`, id) != "0" {
		t.Fatalf("removal: events=%d", len(p.gcal.events))
	}
}

func TestGoogleOnlyShowsWhatTheCreatorCanSee(t *testing.T) {
	p := newPilot(t)
	// A regular user with no role in the Space sees nothing in it, so nothing leaves the system.
	user := p.one(`INSERT INTO users(email,user_type,first_name,last_name,password_hash,status) VALUES ('r@example.com','regular','R','R','x','active') RETURNING id::text`)
	it := p.task("secret plan")
	p.due(it, "", "2026-04-01")
	in, err := p.s.Install(context.Background(), user, "google_calendar", "Mine", map[string]any{"calendar_id": "primary", "space_id": p.space}, map[string]string{"token": "tok"})
	if err != nil {
		t.Fatal(err)
	}
	if r := p.sync(in.ID); len(p.gcal.events) != 0 || r.Pushed != 0 {
		t.Fatalf("invisible Item was published: %+v events=%d", r, len(p.gcal.events))
	}
	// The admin sees it.
	p.installG()
	p.exec(`UPDATE connector_installs SET next_sync_at=NULL`)
	p.s.RunDue(context.Background())
	if len(p.gcal.events) != 1 {
		t.Fatalf("admin's connector must publish it: %d", len(p.gcal.events))
	}
}

func TestGoogleErrorsRefreshAndRateLimit(t *testing.T) {
	p := newPilot(t)
	a, b := p.task("alpha"), p.task("bravo")
	p.due(a, "", "2026-05-01")
	p.due(b, "", "2026-05-02")
	id := p.installG()
	// One Item failing with a server error does not stop the other; the error is still reported.
	p.gcal.fail, p.gcal.failFor = 500, "alpha"
	r, err := p.s.Sync(context.Background(), id)
	if err == nil || r.Pushed != 1 || len(p.gcal.events) != 1 {
		t.Fatalf("partial failure: %+v %v", r, err)
	}
	p.gcal.fail, p.gcal.failFor = 0, ""
	if r := p.sync(id); r.Pushed != 1 || len(p.gcal.events) != 2 {
		t.Fatalf("retry of the failed item: %+v", r)
	}
	// 429 stops the run and waits for Retry-After.
	p.exec(`UPDATE items SET title='again',version=version+1 WHERE id=$1`, a)
	p.gcal.fail = 429
	var rl connectors.RateLimited
	if _, err := p.s.Sync(context.Background(), id); !errors.As(err, &rl) || time.Until(rl.After) < time.Minute {
		t.Fatalf("429: %v", err)
	}
	// 401 explains itself.
	p.gcal.fail = 0
	p.exec(`UPDATE connector_installs SET secrets_enc=NULL WHERE id=$1`, id)
	if _, err := p.s.Configure(context.Background(), id, nil, nil, map[string]string{"token": "stale"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.s.Sync(context.Background(), id); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("401: %v", err)
	}
	// A refresh token yields a fresh access token per run, without storing it.
	if _, err := p.s.Configure(context.Background(), id, nil, nil, map[string]string{"token": "", "refresh_token": "rt", "client_id": "c", "client_secret": "s"}); err != nil {
		t.Fatal(err)
	}
	if r := p.sync(id); r.Pushed != 1 || p.gcal.tokens == 0 {
		t.Fatalf("refresh flow: %+v tokens=%d", r, p.gcal.tokens)
	}
}

func TestValidation(t *testing.T) {
	p := newPilot(t)
	ctx := context.Background()
	for name, c := range map[string]struct {
		kind string
		cfg  map[string]any
		sec  map[string]string
	}{
		"bad repo":        {"github", map[string]any{"repo": "../etc"}, map[string]string{"token": "t"}},
		"no token":        {"github", map[string]any{"repo": "o/r"}, nil},
		"bad space":       {"github", map[string]any{"repo": "o/r", "space_id": "x"}, map[string]string{"token": "t"}},
		"no calendar":     {"google_calendar", map[string]any{}, map[string]string{"token": "t"}},
		"no google creds": {"google_calendar", map[string]any{"calendar_id": "c"}, nil},
	} {
		if _, err := p.s.Install(ctx, p.admin, c.kind, "x", c.cfg, c.sec); !errors.As(err, new(connectors.Invalid)) {
			t.Errorf("%s: want Invalid, got %v", name, err)
		}
	}
	// Google never reads: a Google install can be synced with no remote read at all.
	if _, err := (&gcal.Connector{}).Fetch(ctx, nil, nil); !errors.Is(err, connectors.ErrReadOnly) {
		t.Error("google_calendar must be read-only inbound")
	}
}

func TestWebhookSignature(t *testing.T) {
	body := []byte(`{"action":"opened"}`)
	good := "sha256=" + hmacHex("s3cret", body)
	if !github.VerifySignature("s3cret", body, good) {
		t.Fatal("valid signature rejected")
	}
	for _, bad := range []string{"", "sha256=", "sha256=zz", "sha1=" + hmacHex("s3cret", body), "sha256=" + hmacHex("other", body)} {
		if github.VerifySignature("s3cret", body, bad) {
			t.Errorf("accepted %q", bad)
		}
	}
	if github.VerifySignature("", body, "sha256="+hmacHex("", body)) {
		t.Error("an install without webhook_secret must reject everything")
	}
}

// ---- real-account tests: skipped unless the env is set ----

func TestRealGitHubAccount(t *testing.T) {
	token, repo := getenv("PMS_TEST_GITHUB_TOKEN"), getenv("PMS_TEST_GITHUB_REPO")
	if token == "" || repo == "" {
		t.Skip("set PMS_TEST_GITHUB_TOKEN and PMS_TEST_GITHUB_REPO to run against a real repository")
	}
	c := github.New()
	cfg := map[string]any{"repo": repo}
	sec := map[string]string{"token": token}
	if err := c.Validate(cfg, sec); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created, err := c.Push(ctx, cfg, sec, "", connectors.Record{Title: "pms connector test " + time.Now().Format(time.RFC3339), Body: "created by an automated test"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Push(ctx, cfg, sec, created.ExternalID, connectors.Record{Title: created.Record.Title, Done: true})
	all, err := c.Fetch(ctx, cfg, sec)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range all {
		if r.ExternalID == created.ExternalID {
			return
		}
	}
	t.Fatal("created issue not listed")
}

func TestRealGoogleAccount(t *testing.T) {
	token, cal := getenv("PMS_TEST_GOOGLE_TOKEN"), getenv("PMS_TEST_GOOGLE_CALENDAR")
	if token == "" || cal == "" {
		t.Skip("set PMS_TEST_GOOGLE_TOKEN and PMS_TEST_GOOGLE_CALENDAR to run against a real calendar")
	}
	c := gcal.New()
	cfg := map[string]any{"calendar_id": cal}
	sec := map[string]string{"token": token}
	ctx := context.Background()
	id, err := c.Upsert(ctx, cfg, sec, "", connectors.OutItem{Title: "pms connector test", Start: "2030-01-02", End: "2030-01-03"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Upsert(ctx, cfg, sec, id, connectors.OutItem{Title: "pms connector test (updated)", Start: "2030-01-02", End: "2030-01-03"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Remove(ctx, cfg, sec, id); err != nil {
		t.Fatal(err)
	}
}
