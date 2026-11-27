package clientscope

import "testing"

// PROMPT #026 Workstream C. Automation and template references that cannot be classified safely refuse the whole plan, and each record
// that blocked it leaves a durable quarantine reason. Nothing is applied in part. Removing the blocking records and applying a fresh,
// validated mapping is the only way forward.
func TestAutomationAndTemplateConflictsRefuseAndRecordReasons(t *testing.T) {
	f := newMigrationFixture(t)
	f.exec(`INSERT INTO automation_rules(id,space_id,container_id,name,owner_id) VALUES (gen_random_uuid(),$1,$2,'Cross owner rule',$3)`, f.s1, f.i2, f.admin)
	var ruleID string
	f.exec(`INSERT INTO saved_space_templates(created_by,source_space_id,body) VALUES ($1,$2,'{}'::jsonb)`, f.guest, f.s1)
	if err := f.pool.QueryRow(f.ctx, "SELECT id::text FROM automation_rules LIMIT 1").Scan(&ruleID); err != nil {
		t.Fatal(err)
	}
	m := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA}, SpaceRow{SpaceID: f.s2, Scope: "internal"})
	before := f.scopes()

	rep, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConflict(rep, "automation_container_owner_mismatch") || !hasConflict(rep, "template_provenance_ambiguous") {
		t.Fatalf("the automation and template references must be conflicts: %+v", rep)
	}
	if f.scopes() != before {
		t.Fatal("dry-run changed stored ownership")
	}
	if _, err := Apply(f.ctx, f.pool, m, rep.Digest); err == nil {
		t.Fatal("a plan with these conflicts must be refused as a whole")
	}
	if f.scopes() != before {
		t.Fatal("a refused plan changed stored ownership")
	}
	if n := f.count("SELECT count(*) FROM audit_events WHERE action='client_scope.quarantine' AND target_type='automation_rule' AND target_id=$1 AND details->>'reason'='automation_container_owner_mismatch'", ruleID); n != 1 {
		t.Fatalf("the automation rule needs one durable quarantine reason, got %d", n)
	}
	if n := f.count("SELECT count(*) FROM audit_events WHERE action='client_scope.quarantine' AND target_type='saved_template' AND details->>'reason'='template_provenance_ambiguous'"); n != 1 {
		t.Fatalf("the template needs one durable quarantine reason, got %d", n)
	}
}

// Release is explicit: once the blocking records are corrected, a fresh dry-run reports no conflict and a validated apply succeeds.
func TestResolvingTheBlockingRecordsReleasesTheMapping(t *testing.T) {
	f := newMigrationFixture(t)
	f.exec(`INSERT INTO automation_rules(id,space_id,container_id,name,owner_id) VALUES (gen_random_uuid(),$1,$2,'Cross owner rule',$3)`, f.s1, f.i2, f.admin)
	f.exec(`INSERT INTO saved_space_templates(created_by,source_space_id,body) VALUES ($1,$2,'{}'::jsonb)`, f.member, f.s1)
	m := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA}, SpaceRow{SpaceID: f.s2, Scope: "internal"})
	rep, err := DryRun(f.ctx, f.pool, m)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConflict(rep, "automation_container_owner_mismatch") {
		t.Fatalf("expected the automation conflict: %+v", rep)
	}
	// The operator fixes the records: the rule is removed and the template was created by a member (already so here).
	f.exec(`DELETE FROM automation_rules WHERE name='Cross owner rule'`)
	// The corrected mapping no longer routes items across owners, so it is validated by a fresh dry-run.
	fixed := f.mapping(SpaceRow{SpaceID: f.s1, Scope: "client", ClientID: f.clientA})
	fresh, err := DryRun(f.ctx, f.pool, fixed)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.Conflicts) != 0 {
		t.Fatalf("a corrected mapping must have no conflicts: %+v", fresh.Conflicts)
	}
	if _, err := Apply(f.ctx, f.pool, fixed, fresh.Digest); err != nil {
		t.Fatalf("the validated mapping must apply after the records are corrected: %v", err)
	}
}
