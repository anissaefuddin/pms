package policy

import (
	"fmt"
	"slices"
	"testing"
)

// The built-in roles and ceilings are written out literally here on purpose: custom roles touched the code that resolves
// them, and this snapshot fails if a built-in ever changes meaning (ENT-03 regression).
func TestBuiltInRolesAndCeilingsAreUnchanged(t *testing.T) {
	roles := map[Role][]Capability{
		Read:    {View},
		Limited: {View, Comment, ChangeStatus, Attach},
		Editor:  {View, Comment, ChangeStatus, Attach, Edit, Create, UseBlueprint, CreateBlueprint, ManageBlueprint},
		Full:    {View, Comment, ChangeStatus, Attach, Edit, Create, Delete, Share, UseBlueprint, CreateBlueprint, ManageBlueprint},
	}
	for role, want := range roles {
		if got := RoleCaps(role).List(); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", role, got, want)
		}
	}
	ceilings := map[UserType][]Capability{
		Admin:        {View, Comment, ChangeStatus, Attach, Edit, Create, Delete, Share, UseBlueprint, CreateBlueprint, ManageBlueprint},
		Regular:      {View, Comment, ChangeStatus, Attach, Edit, Create, Delete, Share, UseBlueprint, CreateBlueprint, ManageBlueprint},
		External:     {View, Comment, ChangeStatus, Attach, Edit, Create, UseBlueprint, CreateBlueprint, ManageBlueprint},
		Collaborator: {View, Comment, ChangeStatus, Attach},
		Viewer:       {View, Comment},
	}
	for kind, want := range ceilings {
		if got := Ceiling(kind).List(); !slices.Equal(got, want) {
			t.Errorf("%s ceiling = %v, want %v", kind, got, want)
		}
	}
	if want := []Capability{View, Comment, ChangeStatus, Attach, Edit, Create, Delete, Share, UseBlueprint, CreateBlueprint, ManageBlueprint}; !slices.Equal(AllCapabilities, want) {
		t.Fatalf("capability order is part of the stored mask and must only be appended to: %v", AllCapabilities)
	}
	if ValidRole("mask:1") || ValidRole("custom:x") {
		t.Fatal("custom roles are not built-in roles")
	}
}

// Least privilege: whatever set a custom role holds, a user gets exactly that set cut by the ceiling of the user type, never more.
func TestCustomRoleMasksNeverExceedTheCeiling(t *testing.T) {
	for mask := 0; mask < 1<<len(AllCapabilities); mask++ {
		role := Role(fmt.Sprintf("mask:%d", mask))
		if RoleCaps(role) != Caps(mask) {
			t.Fatalf("mask %d resolved to %d", mask, RoleCaps(role))
		}
		for _, kind := range UserTypes {
			got := Effective(kind, role)
			if got != Caps(mask)&Ceiling(kind) {
				t.Fatalf("%s with mask %d = %v, want %v", kind, mask, got.List(), (Caps(mask) & Ceiling(kind)).List())
			}
			if !Ceiling(kind).Contains(got) {
				t.Fatalf("%s exceeds its ceiling with mask %d", kind, mask)
			}
			// Together with a built-in role the result is the union, still cut by the ceiling.
			if both := Effective(kind, role, Limited); both != (Caps(mask)|RoleCaps(Limited))&Ceiling(kind) {
				t.Fatalf("%s union with limited for mask %d = %v", kind, mask, both.List())
			}
		}
	}
}

func TestMalformedCustomMasksGrantNothing(t *testing.T) {
	for _, raw := range []string{"mask:", "mask:x", "mask:-1", "mask:2048", "mask:1000000", "mask:1.5", "mask: 1", "MASK:1", "custom:1"} {
		if got := RoleCaps(Role(raw)); got != 0 {
			t.Errorf("%q granted %v", raw, got.List())
		}
	}
}

func TestCapsFromNames(t *testing.T) {
	if got, ok := CapsFromNames([]string{"view", "edit"}); !ok || got != setOf(View, Edit) {
		t.Fatalf("got %v %v", got.List(), ok)
	}
	for _, bad := range [][]string{{"view", "view"}, {"view", "fly"}, {""}, {"View"}} {
		if _, ok := CapsFromNames(bad); ok {
			t.Errorf("%v must be refused", bad)
		}
	}
	if got, ok := CapsFromNames(nil); !ok || got != 0 {
		t.Error("an empty list is the empty set")
	}
}
