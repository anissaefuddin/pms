package policy

import (
	"reflect"
	"testing"
)

// expectedRoleCaps and expectedCeilings are typed in by hand from the tables in docs/ARCHITECTURE.md §6.1, on purpose
// not built from the code under test: if either side drifts from the document, this test fails.
var expectedRoleCaps = map[Role][]Capability{
	Read:    {View},
	Limited: {View, Comment, ChangeStatus, Attach},
	Editor:  {View, Comment, ChangeStatus, Attach, Edit, Create, UseBlueprint, CreateBlueprint, ManageBlueprint},
	Full:    {View, Comment, ChangeStatus, Attach, Edit, Create, Delete, Share, UseBlueprint, CreateBlueprint, ManageBlueprint},
}

var expectedCeilings = map[UserType][]Capability{
	Admin:        {View, Comment, ChangeStatus, Attach, Edit, Create, Delete, Share, UseBlueprint, CreateBlueprint, ManageBlueprint},
	Regular:      {View, Comment, ChangeStatus, Attach, Edit, Create, Delete, Share, UseBlueprint, CreateBlueprint, ManageBlueprint},
	External:     {View, Comment, ChangeStatus, Attach, Edit, Create, UseBlueprint, CreateBlueprint, ManageBlueprint},
	Collaborator: {View, Comment, ChangeStatus, Attach},
	Viewer:       {View, Comment},
}

func TestDefinitionMatchesDocument(t *testing.T) {
	for role, want := range expectedRoleCaps {
		if got := RoleCaps(role).List(); !reflect.DeepEqual(got, want) {
			t.Errorf("role %s: got %v, want %v", role, got, want)
		}
	}
	for ut, want := range expectedCeilings {
		if got := Ceiling(ut).List(); !reflect.DeepEqual(got, want) {
			t.Errorf("user type %s: got %v, want %v", ut, got, want)
		}
	}
	def := Definition()
	if len(def.Roles) != 4 || len(def.UserTypes) != 5 {
		t.Fatalf("definition has %d roles and %d user types, want 4 and 5", len(def.Roles), len(def.UserTypes))
	}
}

func TestUnknownRoleAndTypeGetNothing(t *testing.T) {
	if !RoleCaps("owner").Empty() || !Ceiling("superuser").Empty() || ValidRole("owner") {
		t.Fatal("an unknown role or user type must grant nothing")
	}
	if !Effective("superuser", Full).Empty() {
		t.Fatal("an unknown user type must get no capabilities even with a full share")
	}
}

func TestEffectiveIsUnionCutByCeiling(t *testing.T) {
	cases := []struct {
		name  string
		ut    UserType
		roles []Role
		want  []Capability
	}{
		{"no share, no access", Regular, nil, []Capability{}},
		{"read + editor union", Regular, []Role{Read, Editor}, expectedRoleCaps[Editor]},
		{"limited + full union", Regular, []Role{Limited, Full}, expectedRoleCaps[Full]},
		{"viewer is capped even with full", Viewer, []Role{Full}, []Capability{View, Comment}},
		{"collaborator is capped at limited", Collaborator, []Role{Full}, expectedRoleCaps[Limited]},
		{"external may edit but not delete or share", External, []Role{Full}, expectedRoleCaps[Editor]},
		{"viewer with read sees but cannot comment", Viewer, []Role{Read}, []Capability{View}},
		{"admin keeps everything", Admin, []Role{Read}, expectedRoleCaps[Read]},
	}
	for _, tc := range cases {
		if got := Effective(tc.ut, tc.roles...).List(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDecide(t *testing.T) {
	if Decide(none, View) != NotFound {
		t.Error("no capability at all must be NotFound, never Forbidden")
	}
	if Decide(RoleCaps(Read), Edit) != Forbidden || Decide(RoleCaps(Read), View) != Allowed {
		t.Error("visible but not allowed must be Forbidden; allowed must be Allowed")
	}
}

func TestSpaceCapsRules(t *testing.T) {
	owner := Actor{ID: "owner", Type: Regular}
	if got := spaceCaps(owner, "personal", "owner", false, false, nil); got != Ceiling(Regular) {
		t.Errorf("owner of a personal space: %v", got.List())
	}
	if !spaceCaps(Actor{ID: "x", Type: Admin}, "personal", "owner", false, false, nil).Empty() {
		t.Error("an admin must not see someone else's personal space")
	}
	// D-17 A1: a Workspace admin has no implicit capability in a shared space; an explicit Full share is what grants it.
	if !spaceCaps(Actor{ID: "x", Type: Admin}, "shared", "", false, false, nil).Empty() {
		t.Error("a workspace admin without a share must not see a shared space's content")
	}
	if got := spaceCaps(Actor{ID: "x", Type: Admin}, "shared", "", false, false, []string{"full"}); got != Effective(Admin, Full) {
		t.Error("an admin with an explicit full share gets the full role, cut to the admin ceiling")
	}
	if !spaceCaps(Actor{ID: "x", Type: Regular}, "shared", "", false, false, nil).Empty() {
		t.Error("a regular user without a share must not see a shared space")
	}
	if got := spaceCaps(Actor{ID: "e", Type: External}, "personal", "e", false, false, nil); got != Ceiling(External) {
		t.Errorf("an external owner of a personal space is still cut by the ceiling: %v", got.List())
	}
}
