package clientscope

import (
	"testing"
)

// Persistent evidence: each real ownership change writes one audit row in its own transaction, reruns write none, a refused plan leaves a
// durable refusal record, and an audit write that fails rolls the ownership change back.
func TestApplyWritesDurableAuditOnlyForRealChanges(t *testing.T) {
	f := newMigrationFixture(t)
	m := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA})
	rep, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(f.ctx, f.pool, m, rep.Digest); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if n := f.count("SELECT count(*) FROM audit_events WHERE action='client_scope.classify' AND details->>'mapping_digest'=$1", m.Digest()); n != 2 {
		t.Fatalf("one audit row per changed Space and item, got %d", n)
	}
	again, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(f.ctx, f.pool, m, again.Digest); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if n := f.count("SELECT count(*) FROM audit_events WHERE action='client_scope.classify'"); n != 2 {
		t.Fatalf("a rerun must not duplicate mutation audit, got %d", n)
	}
}

func TestAuditFailureRollsBackOwnershipChange(t *testing.T) {
	f := newMigrationFixture(t)
	f.exec(`CREATE OR REPLACE FUNCTION clientscope_fail_classify() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN IF NEW.action = 'client_scope.classify' THEN RAISE EXCEPTION 'audit unavailable'; END IF; RETURN NEW; END $$`)
	f.exec(`CREATE TRIGGER clientscope_fail_classify BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION clientscope_fail_classify()`)
	t.Cleanup(func() {
		_, _ = f.pool.Exec(f.ctx, `DROP TRIGGER IF EXISTS clientscope_fail_classify ON audit_events`)
	})
	m := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA})
	rep, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	before := f.scopes()
	if _, err := Apply(f.ctx, f.pool, m, rep.Digest); err == nil {
		t.Fatal("apply must fail when its audit write fails")
	}
	if f.scopes() != before {
		t.Fatal("an unaudited ownership change was left behind")
	}
}

func TestRefusedPlanLeavesDurableRefusalRecord(t *testing.T) {
	f := newMigrationFixture(t)
	conflict := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA}, SpaceRow{SpaceID: f.s2, Scope: "internal"})
	rep, err := DryRun(f.ctx, f.pool, conflict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(f.ctx, f.pool, conflict, rep.Digest); err == nil {
		t.Fatal("a conflicting plan must be refused")
	}
	if n := f.count("SELECT count(*) FROM audit_events WHERE action='client_scope.refused' AND details->>'mapping_digest'=$1", conflict.Digest()); n != 1 {
		t.Fatalf("the refusal must be recorded once, got %d", n)
	}
	if n := f.count("SELECT count(*) FROM audit_events WHERE action='client_scope.classify'"); n != 0 {
		t.Fatalf("a refused plan classified %d rows", n)
	}
}

// A Request Form whose destination Space belongs to another owner is a conflict. The same form with a same-owner destination is not.
func TestFormDestinationOwnerIsAConflict(t *testing.T) {
	f := newMigrationFixture(t)
	f.exec(`INSERT INTO request_forms(id,space_id,name,definition,response,created_by,updated_by)
VALUES (gen_random_uuid(),$1,'Intake','{}',jsonb_build_object('location',jsonb_build_object('space_id',$2::text)),$3,$3)`, f.s1, f.s2, f.admin)
	mixed := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA}, SpaceRow{SpaceID: f.s2, Scope: "internal"})
	rep, err := DryRun(f.ctx, f.pool, mixed)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConflict(rep, "form_destination_owner_mismatch") {
		t.Fatalf("a form that routes across owners must be a conflict: %+v", rep)
	}
	f.exec(`UPDATE request_forms SET response=jsonb_build_object('location',jsonb_build_object('space_id',$1::text))`, f.s1)
	same := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA})
	ok, err := DryRun(f.ctx, f.pool, same)
	if err != nil {
		t.Fatal(err)
	}
	if hasConflict(ok, "form_destination_owner_mismatch") {
		t.Fatal("a same-owner destination must not be a conflict")
	}
}

// Existing identifiers, hierarchy and valid shares survive apply: only scope_state and client_id change.
func TestApplyPreservesIdentifiersHierarchyAndShares(t *testing.T) {
	f := newMigrationFixture(t)
	f.exec(`INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2,'read')`, f.i1, f.member)
	idsBefore := f.count("SELECT count(*) FROM items WHERE id IN ($1,$2,$3)", f.i1, f.i2, f.i3)
	sharesBefore := f.count("SELECT count(*) FROM shares WHERE target_id=$1", f.i1)
	locsBefore := f.count("SELECT count(*) FROM item_locations WHERE item_id IN ($1,$2,$3)", f.i1, f.i2, f.i3)
	m := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA})
	rep, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(f.ctx, f.pool, m, rep.Digest); err != nil {
		t.Fatal(err)
	}
	if f.count("SELECT count(*) FROM items WHERE id IN ($1,$2,$3)", f.i1, f.i2, f.i3) != idsBefore ||
		f.count("SELECT count(*) FROM item_locations WHERE item_id IN ($1,$2,$3)", f.i1, f.i2, f.i3) != locsBefore {
		t.Fatal("apply must keep item identifiers and placements")
	}
	if f.count("SELECT count(*) FROM shares WHERE target_id=$1 AND principal_id=$2", f.i1, f.member) != sharesBefore {
		t.Fatal("apply must keep a valid share")
	}
}
