package policy

import "testing"

func TestA1ContractSeparatesAdministrationFromContent(t *testing.T) {
	admin := Actor{ID: "a", Type: Admin}
	regular := Actor{ID: "r", Type: Regular}
	if !CanAdministerWorkspace(admin) || CanAdministerWorkspace(regular) {
		t.Fatal("workspace administration belongs to the admin type only")
	}
	// A Workspace admin with no caps on an item cannot read it: administration is not content access.
	if CanViewItem(0) {
		t.Fatal("no caps means no read, whatever the user type")
	}
	if !CanViewItem(setOf(View)) {
		t.Fatal("View caps allow a read")
	}
	if !CanViewOperationalReport(admin) || CanViewOperationalReport(regular) {
		t.Fatal("the operational report is for the admin type; raw rows still need item caps")
	}
	if CanApprove(false) || !CanApprove(true) {
		t.Fatal("approval needs an explicit assignment or grant, never a type alone")
	}
}
