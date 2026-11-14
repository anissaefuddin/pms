package connectors

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
)

// fake is an in-memory remote. Edits to it model someone changing the external system.
type fake struct {
	recs     map[string]Remote
	next     int
	pushes   int
	readOnly bool
}

func (f *fake) Kind() string { return "fake" }
func (f *fake) Validate(_ map[string]any, secrets map[string]string) error {
	if _, ok := secrets["token"]; !ok {
		return errors.New("token is required")
	}
	return nil
}
func (f *fake) Fetch(context.Context, map[string]any, map[string]string) ([]Remote, error) {
	out := []Remote{}
	for _, r := range f.recs {
		out = append(out, r)
	}
	return out, nil
}
func (f *fake) Push(_ context.Context, _ map[string]any, _ map[string]string, id string, rec Record) (Remote, error) {
	if f.readOnly {
		return Remote{}, ErrReadOnly
	}
	f.pushes++
	if id == "" {
		f.next++
		id = fmt.Sprintf("ext-%d", f.next)
	}
	r := Remote{ExternalID: id, Version: fmt.Sprintf("v%d", f.pushes+100), Record: rec}
	f.recs[id] = r
	return r, nil
}
func (f *fake) edit(id, title string) {
	r := f.recs[id]
	r.Version += "e"
	r.Record.Title = title
	f.recs[id] = r
}

type env struct {
	t    *testing.T
	s    *Service
	f    *fake
	user string
}

func setup(t *testing.T) env {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.New(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	f := &fake{recs: map[string]Remote{}}
	reg := &Registry{}
	reg.Register(f)
	var user string
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,user_type,first_name,last_name,password_hash,status) VALUES ('a@example.com','admin','A','A','x','active') RETURNING id::text`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	return env{t, &Service{Pool: pool, Keys: twofactor.Keys{Current: key}, Registry: reg}, f, user}
}

func (e env) item(title string) string {
	var id string
	if err := e.s.Pool.QueryRow(context.Background(), `INSERT INTO items(type,title,created_by) VALUES('task',$1,$2) RETURNING id::text`, title, e.user).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}
func (e env) title(id string) string {
	var s string
	if err := e.s.Pool.QueryRow(context.Background(), `SELECT title FROM items WHERE id=$1`, id).Scan(&s); err != nil {
		e.t.Fatal(err)
	}
	return s
}
func (e env) rename(id, title string) {
	if _, err := e.s.Pool.Exec(context.Background(), `UPDATE items SET title=$2,version=version+1 WHERE id=$1`, id, title); err != nil {
		e.t.Fatal(err)
	}
}
func (e env) sync(id string) SyncResult {
	r, err := e.s.Sync(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

func TestLifecycleAndSecrets(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.s.Install(ctx, e.user, "nope", "x", nil, nil); !errors.As(err, new(Invalid)) {
		t.Fatalf("unknown kind: %v", err)
	}
	if _, err := e.s.Install(ctx, e.user, "fake", "x", nil, map[string]string{}); !errors.As(err, new(Invalid)) {
		t.Fatalf("missing secret must fail validation: %v", err)
	}
	in, err := e.s.Install(ctx, e.user, "fake", "Pilot", map[string]any{"repo": "a/b"}, map[string]string{"token": "s3cret-value"})
	if err != nil || !in.Enabled || len(in.SecretNames) != 1 {
		t.Fatalf("install: %+v %v", in, err)
	}
	var stored string
	_ = e.s.Pool.QueryRow(ctx, `SELECT secrets_enc FROM connector_installs WHERE id=$1`, in.ID).Scan(&stored)
	if !strings.HasPrefix(stored, "v1:") || strings.Contains(stored, "s3cret") {
		t.Fatalf("secret not encrypted: %q", stored)
	}
	// Secrets are bound to the install: the blob copied to another row does not open.
	if _, _, err := e.s.Keys.Decrypt(stored, "other-id"); err == nil {
		t.Fatal("blob decrypted under a different install id")
	}
	name := "Renamed"
	in, err = e.s.Configure(ctx, in.ID, &name, map[string]any{"repo": "c/d"}, map[string]string{"extra": "1"})
	if err != nil || in.Name != "Renamed" || len(in.SecretNames) != 2 || in.Config["repo"] != "c/d" {
		t.Fatalf("configure: %+v %v", in, err)
	}
	if _, err := e.s.Configure(ctx, in.ID, nil, nil, map[string]string{"token": ""}); !errors.As(err, new(Invalid)) {
		t.Fatalf("removing the required secret must fail: %v", err)
	}
	if in, _ = e.s.SetEnabled(ctx, in.ID, false); in.Enabled {
		t.Fatal("still enabled")
	}
	if _, err := e.s.Sync(ctx, in.ID); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled connector synced: %v", err)
	}
	if in, _ = e.s.SetEnabled(ctx, in.ID, true); !in.Enabled {
		t.Fatal("not re-enabled")
	}
	// No key configured: install with secrets is refused, not stored in clear.
	e.s.Keys = twofactor.Keys{}
	if _, err := e.s.Install(ctx, e.user, "fake", "y", nil, map[string]string{"token": "t"}); !errors.Is(err, ErrNoKey) {
		t.Fatalf("no key: %v", err)
	}
}

func TestSyncBothWaysNoSilentOverwriteAndIdempotent(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	in, _ := e.s.Install(ctx, e.user, "fake", "P", nil, map[string]string{"token": "t"})
	item := e.item("Plan")
	ext, err := e.s.Link(ctx, in.ID, item, "")
	if err != nil || e.f.recs[ext].Record.Title != "Plan" {
		t.Fatalf("link: %v", err)
	}
	if r := e.sync(in.ID); r.Unchanged != 1 || r.Pushed+r.Pulled+r.Conflicts != 0 {
		t.Fatalf("fresh link must be quiet: %+v", r)
	}
	// Item only -> remote.
	e.rename(item, "Plan v2")
	if r := e.sync(in.ID); r.Pushed != 1 || e.f.recs[ext].Record.Title != "Plan v2" {
		t.Fatalf("push: %+v", r)
	}
	pushes := e.f.pushes
	// Idempotent: again with no change writes nothing anywhere.
	if r := e.sync(in.ID); r.Unchanged != 1 || e.f.pushes != pushes {
		t.Fatalf("rerun wrote: %+v", r)
	}
	// Remote only -> Item.
	e.f.edit(ext, "Plan v3")
	if r := e.sync(in.ID); r.Pulled != 1 || e.title(item) != "Plan v3" {
		t.Fatalf("pull: %+v", r)
	}
	// Both changed: neither side is overwritten, a conflict is recorded; reruns do not duplicate it.
	e.rename(item, "Item side")
	e.f.edit(ext, "Remote side")
	for i := 0; i < 3; i++ {
		r := e.sync(in.ID)
		if (i == 0 && r.Conflicts != 1) || (i > 0 && r.Pending != 1) {
			t.Fatalf("run %d: %+v", i, r)
		}
	}
	if e.title(item) != "Item side" || e.f.recs[ext].Record.Title != "Remote side" {
		t.Fatal("a side was overwritten while in conflict")
	}
	open, _ := e.s.Conflicts(ctx, in.ID, false)
	if len(open) != 1 || open[0].ItemTitle != "Item side" || open[0].RemoteTitle != "Remote side" || *open[0].BaseTitle != "Plan v3" {
		t.Fatalf("conflict record: %+v", open)
	}
	// Resolve: remote wins -> Item takes it; the Item-side text stays in the record.
	if err := e.s.Resolve(ctx, open[0].ID, "remote", e.user); err != nil || e.title(item) != "Remote side" {
		t.Fatalf("resolve remote: %v", err)
	}
	if err := e.s.Resolve(ctx, open[0].ID, "item", e.user); !errors.Is(err, ErrAlreadyClosed) {
		t.Fatalf("second resolve: %v", err)
	}
	if all, _ := e.s.Conflicts(ctx, in.ID, true); all[0].ItemTitle != "Item side" || all[0].Resolution == nil {
		t.Fatalf("history lost: %+v", all)
	}
	if r := e.sync(in.ID); r.Unchanged != 1 {
		t.Fatalf("after resolve: %+v", r)
	}
	// Resolve the other way.
	e.rename(item, "A")
	e.f.edit(ext, "B")
	e.sync(in.ID)
	open, _ = e.s.Conflicts(ctx, in.ID, false)
	if err := e.s.Resolve(ctx, open[0].ID, "item", e.user); err != nil || e.f.recs[ext].Record.Title != "A" {
		t.Fatalf("resolve item: %v", err)
	}
}

func TestLinkExistingRemoteDifferentTitleConflicts(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	in, _ := e.s.Install(ctx, e.user, "fake", "P", nil, map[string]string{"token": "t"})
	e.f.recs["r1"] = Remote{ExternalID: "r1", Version: "1", Record: Record{Title: "Remote"}}
	item := e.item("Local")
	if _, err := e.s.Link(ctx, in.ID, item, "r1"); err != nil {
		t.Fatal(err)
	}
	if r := e.sync(in.ID); r.Conflicts != 1 || e.title(item) != "Local" || e.f.recs["r1"].Record.Title != "Remote" {
		t.Fatalf("first sync must not pick a side: %+v", r)
	}
	// Equal titles are adopted without a conflict.
	e.f.recs["r2"] = Remote{ExternalID: "r2", Version: "1", Record: Record{Title: "Same"}}
	if _, err := e.s.Link(ctx, in.ID, e.item("Same"), "r2"); err != nil {
		t.Fatal(err)
	}
	if r := e.sync(in.ID); r.Conflicts != 0 || r.Pending != 1 || r.Unchanged != 1 {
		t.Fatalf("equal adopt: %+v", r)
	}
}

func TestReadOnlyRemoteSkipsPush(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	in, _ := e.s.Install(ctx, e.user, "fake", "P", nil, map[string]string{"token": "t"})
	item := e.item("T")
	ext, _ := e.s.Link(ctx, in.ID, item, "")
	e.f.readOnly = true
	e.rename(item, "T2")
	if r := e.sync(in.ID); r.Skipped != 1 || e.f.recs[ext].Record.Title != "T" {
		t.Fatalf("%+v", r)
	}
}
