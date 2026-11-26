package housekeeping_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/bootstrap"
	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/housekeeping"
	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/anissaefuddin/pms/server/internal/testdb"
)

func TestRunKeepsLiveRowsAndRemovesDeadOnes(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	boot, err := bootstrap.Run(ctx, pool, "Acme", "admin@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	// sessions: live, idle limit passed, absolute limit passed
	exec("INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) VALUES ('\\x01',$1,now()+interval '1 day',now()+interval '20 days')", boot.AdminID)
	exec("INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) VALUES ('\\x02',$1,now()-interval '1 minute',now()+interval '20 days')", boot.AdminID)
	exec("INSERT INTO sessions(token_hash,user_id,expires_at,absolute_expires_at) VALUES ('\\x03',$1,now()-interval '2 minutes',now()-interval '1 minute')", boot.AdminID)
	// tokens: bootstrap's own invite (live), an expired reset, a used reset, a live reset
	exec("INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) VALUES ('\\x11',$1,'reset',now()-interval '1 hour')", boot.AdminID)
	exec("INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at,used_at) VALUES ('\\x12',$1,'reset',now()+interval '1 hour',now())", boot.AdminID)
	exec("INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) VALUES ('\\x13',$1,'reset',now()+interval '1 hour')", boot.AdminID)
	// an invited user whose invitation expired keeps holding a seat
	exec("INSERT INTO users(email,user_type,status) VALUES ('late@example.com','regular','invited')")
	exec("INSERT INTO user_tokens(token_hash,user_id,purpose,expires_at) SELECT '\\x21',id,'invite',now()-interval '1 day' FROM users WHERE email='late@example.com'")

	// shares: one that still points at everything (bootstrap's own on General), one whose group is gone, one whose user is gone
	exec("INSERT INTO groups(id,name) VALUES ('00000000-0000-0000-0000-0000000000a1','Temp')")
	var spaceID string
	if err := pool.QueryRow(ctx, "SELECT id::text FROM spaces WHERE kind='shared' LIMIT 1").Scan(&spaceID); err != nil {
		t.Fatal(err)
	}
	exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,'group','00000000-0000-0000-0000-0000000000a1','read')", spaceID)
	exec("DELETE FROM groups WHERE id='00000000-0000-0000-0000-0000000000a1'")
	exec("INSERT INTO users(id,email,first_name,last_name,password_hash,status) VALUES ('00000000-0000-0000-0000-0000000000b1','gone@example.com','G','G','x','active')")
	exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,'user','00000000-0000-0000-0000-0000000000b1','read')", spaceID)
	exec("DELETE FROM users WHERE id='00000000-0000-0000-0000-0000000000b1'")

	if err := housekeeping.Run(ctx, pool); err != nil {
		t.Fatal(err)
	}
	count := func(table string) (n int) {
		t.Helper()
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	// kept: 1 session; tokens: bootstrap invite + live reset; users: admin + late
	if got := count("sessions"); got != 1 {
		t.Fatalf("sessions left: %d, want 1", got)
	}
	if got := count("user_tokens"); got != 2 {
		t.Fatalf("tokens left: %d, want 2", got)
	}
	if got := count("shares"); got != 1 {
		t.Fatalf("shares left: %d, want 1 (orphans of a deleted group and a deleted user must go)", got)
	}
	// the placeholder owner of public-form items (migration 00033) is always there and is not a person
	if got := count("users WHERE id<>'00000000-0000-4000-8000-0000000000e1'"); got != 2 {
		t.Fatalf("users left: %d, want 2 (invited users must keep their seat)", got)
	}
	if got := count("users WHERE id='00000000-0000-4000-8000-0000000000e1'"); got != 1 {
		t.Fatalf("housekeeping removed the placeholder requester: %d", got)
	}
}

func TestSweepFilesRemovesOnlyOldFilesWithoutARow(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, testdb.NewMigrated(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	boot, err := bootstrap.Run(ctx, pool, "Acme", "admin@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	store := storage.LocalStorage{Dir: t.TempDir()}
	put := func(age time.Duration) string {
		t.Helper()
		f, err := store.Put(bytes.NewReader([]byte("x")), 10)
		if err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-age)
		if err := os.Chtimes(filepath.Join(store.Dir, f.Key), old, old); err != nil {
			t.Fatal(err)
		}
		return f.Key
	}
	used, orphan, young, oldVersion := put(48*time.Hour), put(48*time.Hour), put(time.Hour), put(48*time.Hour)
	var item string
	if err := pool.QueryRow(ctx, "INSERT INTO items(type,title,created_by) VALUES ('task','t',$1) RETURNING id::text", boot.AdminID).Scan(&item); err != nil {
		t.Fatal(err)
	}
	// A soft-deleted row still owns its file: the Recycle bin can restore it.
	if _, err := pool.Exec(ctx, `INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256,deleted_at)
VALUES (gen_random_uuid(),$1,$2,'a.txt','text/plain',1,$3,decode(repeat('00',32),'hex'),now())`, item, boot.AdminID, used); err != nil {
		t.Fatal(err)
	}
	// An older version's file is referenced only by attachment_versions and must survive.
	if _, err := pool.Exec(ctx, `INSERT INTO attachment_versions(attachment_id,version,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
SELECT id,2,$1,'old.txt','text/plain',1,$2,decode(repeat('01',32),'hex') FROM attachments LIMIT 1`, boot.AdminID, oldVersion); err != nil {
		t.Fatal(err)
	}
	removed, err := housekeeping.SweepFiles(ctx, pool, store, 24*time.Hour)
	if err != nil || removed != 1 {
		t.Fatalf("removed %d, err %v; want exactly the old orphan", removed, err)
	}
	exists := func(key string) bool { _, err := os.Stat(filepath.Join(store.Dir, key)); return err == nil }
	if exists(orphan) || !exists(oldVersion) || !exists(used) || !exists(young) {
		t.Fatalf("orphan=%v used=%v young=%v", exists(orphan), exists(used), exists(young))
	}
	if n, err := housekeeping.SweepFiles(ctx, pool, storage.LocalStorage{Dir: filepath.Join(t.TempDir(), "missing")}, 0); err != nil || n != 0 {
		t.Fatalf("missing dir must be a no-op, got %d, %v", n, err)
	}
}
