package spaces

import (
	"testing"

	"github.com/anissaefuddin/pms/server/internal/policy"
)

func TestBuildMembersMergesDirectAndGroupGrants(t *testing.T) {
	grants := []shareGrant{
		{principalType: "user", principalID: "ann", name: "Ann", role: policy.Editor, caps: policy.RoleCaps(policy.Editor)},
		{principalType: "group", principalID: "g1", name: "Ops", role: policy.Read, caps: policy.RoleCaps(policy.Read)},
	}
	links := []groupLink{{groupID: "g1", userID: "ann"}, {groupID: "g1", userID: "bob"}}
	users := map[string]userInfo{"ann": {name: "Ann", userType: policy.Regular}, "bob": {name: "Bob", userType: policy.Regular}}
	got := buildMembers(grants, links, users)
	if len(got) != 2 {
		t.Fatalf("want 2 members, got %d", len(got))
	}
	ann := got[0]
	if ann.Role != string(policy.Editor) {
		t.Fatalf("Ann's union is the editor role, got %q", ann.Role)
	}
	if len(ann.Sources) != 2 {
		t.Fatalf("Ann keeps both reasons, got %d", len(ann.Sources))
	}
	if got[1].Role != string(policy.Read) || len(got[1].Sources) != 1 || got[1].Sources[0].Kind != "group" {
		t.Fatalf("Bob has only the group reason: %+v", got[1])
	}
}

func TestBuildMembersCapsByUserType(t *testing.T) {
	grants := []shareGrant{{principalType: "user", principalID: "v", name: "V", role: policy.Full, caps: policy.RoleCaps(policy.Full)}}
	users := map[string]userInfo{"v": {name: "V", userType: policy.Viewer}}
	got := buildMembers(grants, nil, users)
	if len(got) != 1 || !got[0].CappedByUserType {
		t.Fatalf("a viewer's full role is capped and flagged: %+v", got)
	}
	if got[0].Role == string(policy.Full) {
		t.Fatalf("the capped access is no longer the full role")
	}
}

func TestBuildMembersSkipsUnknownUsers(t *testing.T) {
	grants := []shareGrant{{principalType: "user", principalID: "gone", role: policy.Read, caps: policy.RoleCaps(policy.Read)}}
	if got := buildMembers(grants, nil, map[string]userInfo{}); len(got) != 0 {
		t.Fatalf("a deactivated user shows no access: %+v", got)
	}
}

func TestEffectiveRoleNamesCustomWhenNoBuiltInMatches(t *testing.T) {
	if got := effectiveRole(policy.RoleCaps(policy.Limited) | policy.RoleCaps(policy.Read)); got != string(policy.Limited) {
		t.Fatalf("a union equal to a built-in role names that role, got %q", got)
	}
	odd, ok := policy.CapsFromNames([]string{"view", "delete"}) // Read plus delete: no built-in role has exactly this
	if !ok {
		t.Fatal("capability names must be known")
	}
	if got := effectiveRole(odd); got != "custom" {
		t.Fatalf("a set matching no built-in role is custom, got %q", got)
	}
}
