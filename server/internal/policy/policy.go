// Package policy is the single place that decides who may do what (ARCHITECTURE §6, SEC-02).
//
// The capability matrix below is plain data so the HTTP layer, the tests and the future USR-02 matrix screen all read the
// same definition. Rules: effective capabilities = union of the share roles that apply to the actor, intersected with
// the ceiling of the actor's user type. "No access" is reported as not found, never as forbidden.
package policy

import (
	"math/bits"
	"strconv"
	"strings"
)

type Capability string

const (
	View         Capability = "view"
	Comment      Capability = "comment"
	ChangeStatus Capability = "change_status"
	Attach       Capability = "attach"
	Edit         Capability = "edit"
	Create       Capability = "create"
	Delete       Capability = "delete"
	Share        Capability = "share"
	// Blueprint rights, appended (the bit order is stored in custom roles): use creates items from a blueprint,
	// create saves a new blueprint, manage deletes one (and will edit it).
	UseBlueprint    Capability = "use_blueprint"
	CreateBlueprint Capability = "create_blueprint"
	ManageBlueprint Capability = "manage_blueprint"
)

// AllCapabilities is the display and API order.
var AllCapabilities = []Capability{View, Comment, ChangeStatus, Attach, Edit, Create, Delete, Share, UseBlueprint, CreateBlueprint, ManageBlueprint}

// Caps is a set of capabilities, one bit per entry of AllCapabilities.
type Caps uint16

func bit(c Capability) Caps {
	for i, known := range AllCapabilities {
		if known == c {
			return 1 << i
		}
	}
	return 0
}

func setOf(list ...Capability) (s Caps) {
	for _, c := range list {
		s |= bit(c)
	}
	return s
}

const none Caps = 0

func (s Caps) Has(c Capability) bool { return bit(c) != 0 && s&bit(c) == bit(c) }
func (s Caps) Empty() bool           { return s == 0 }
func (s Caps) Count() int            { return bits.OnesCount16(uint16(s)) }

// List returns the capabilities in AllCapabilities order (never nil, so it marshals as [] not null).
func (s Caps) List() []Capability {
	out := make([]Capability, 0, s.Count())
	for _, c := range AllCapabilities {
		if s.Has(c) {
			out = append(out, c)
		}
	}
	return out
}

// Contains reports whether every capability of other is also in s.
func (s Caps) Contains(other Caps) bool { return s&other == other }

type Role string

const (
	Read    Role = "read"
	Limited Role = "limited"
	Editor  Role = "editor"
	Full    Role = "full"
)

// Roles in ascending order of power.
var Roles = []Role{Read, Limited, Editor, Full}

type UserType string

const (
	Admin        UserType = "admin"
	Regular      UserType = "regular"
	External     UserType = "external"
	Collaborator UserType = "collaborator"
	Viewer       UserType = "viewer"
)

var UserTypes = []UserType{Admin, Regular, External, Collaborator, Viewer}

var roleCaps = map[Role]Caps{
	Read:    setOf(View),
	Limited: setOf(View, Comment, ChangeStatus, Attach),
	// Editor keeps what it could do before blueprints had their own rights: save and delete used Edit, use used Create.
	Editor: setOf(View, Comment, ChangeStatus, Attach, Edit, Create, UseBlueprint, CreateBlueprint, ManageBlueprint),
	Full:   setOf(AllCapabilities...),
}

var ceilings = map[UserType]Caps{
	Admin:        setOf(AllCapabilities...),
	Regular:      setOf(AllCapabilities...),
	External:     roleCaps[Editor],
	Collaborator: roleCaps[Limited],
	Viewer:       setOf(View, Comment),
}

// RoleCaps returns what a share role grants; an unknown role grants nothing.
// RoleCaps returns what a share role grants. A role of the form "mask:N" comes from a custom role (SQL turns such a share into
// its capability mask); it still passes through Effective, so it can never exceed the user type's ceiling.
func RoleCaps(r Role) Caps {
	if raw, ok := strings.CutPrefix(string(r), "mask:"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 1<<len(AllCapabilities)-1 {
			return none
		}
		return Caps(n)
	}
	return roleCaps[r]
}

// Mask is the stored form of a capability set (one bit per AllCapabilities entry, append-only order).
func (s Caps) Mask() int { return int(s) }

// CapsFromNames builds a set from capability names; ok is false for an unknown or repeated name.
func CapsFromNames(names []string) (Caps, bool) {
	var out Caps
	for _, n := range names {
		b := bit(Capability(n))
		if b == 0 || out&b != 0 {
			return 0, false
		}
		out |= b
	}
	return out, true
}

// Ceiling returns the most a user type may ever do; an unknown type gets nothing.
func Ceiling(t UserType) Caps { return ceilings[t] }

// ValidRole reports whether s names a share role.
func ValidRole(s string) bool { _, ok := roleCaps[Role(s)]; return ok }

// Effective combines the roles that apply to an actor (union) and applies the ceiling of the actor's user type.
func Effective(t UserType, roles ...Role) Caps {
	var union Caps
	for _, r := range roles {
		union |= RoleCaps(r)
	}
	return union & ceilings[t]
}

// Matrix is the definition as data, for the USR-02 screen and for tests that must follow it.
type Matrix struct {
	Roles     map[Role][]Capability     `json:"roles"`
	UserTypes map[UserType][]Capability `json:"user_types"`
}

func Definition() Matrix {
	m := Matrix{Roles: map[Role][]Capability{}, UserTypes: map[UserType][]Capability{}}
	for _, r := range Roles {
		m.Roles[r] = roleCaps[r].List()
	}
	for _, t := range UserTypes {
		m.UserTypes[t] = ceilings[t].List()
	}
	return m
}

// Decision is the answer to "may this actor do this?".
type Decision int

const (
	// NotFound means the actor cannot see the target at all; callers answer 404 so existence does not leak.
	NotFound Decision = iota
	// Forbidden means the target is visible but the action is not allowed (403).
	Forbidden
	Allowed
)

func (d Decision) String() string { return [...]string{"not_found", "forbidden", "allowed"}[d] }

// Decide turns an actor's effective capabilities on a target into a decision for one capability.
func Decide(effective Caps, need Capability) Decision {
	switch {
	case effective.Empty():
		return NotFound
	case effective.Has(need):
		return Allowed
	default:
		return Forbidden
	}
}

// CanBrowseDirectory says whether a user type may look people up (names, emails, groups). External, collaborator and
// viewer users may not, so a partner organisation cannot read the staff directory.
// shortcut: a coarse rule; refine to "only people who share a Space with me" when external-user scenarios are specified.
func CanBrowseDirectory(t UserType) bool { return t == Admin || t == Regular }
