package solutions

import (
	"reflect"
	"strings"
	"testing"
)

// SOL-01a, SOL-01b: exactly one active PMS, with the contract from the MVP plan.
func TestBuiltinContainsOnlyTheActivePMSContract(t *testing.T) {
	want := []Definition{{ID: "pms", Name: "Projects", Required: true, Enabled: true, Capabilities: []string{"work_management"}}}
	if got := Builtin(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Builtin() = %+v, want %+v", got, want)
	}
	if err := Validate(Builtin()); err != nil {
		t.Fatalf("valid builtin registry rejected: %v", err)
	}
}

// SOL-01c: a caller that changes what it received cannot change what the next call returns, including capabilities.
func TestBuiltinReturnsIndependentCopies(t *testing.T) {
	first := Builtin()
	first[0].Name = "changed"
	first[0].Enabled = false
	first[0].Capabilities[0] = "rbac:everything"
	first = append(first, Definition{ID: "extra"})
	second := Builtin()
	if second[0].Name != "Projects" || !second[0].Enabled || second[0].Capabilities[0] != "work_management" || len(second) != 1 {
		t.Fatalf("builtin state leaked between calls: %+v", second)
	}
}

// SOL-01d, SOL-02a..SOL-02d: every rejected shape, table-driven.
func TestValidateRejectsInvalidRegistries(t *testing.T) {
	pms := func(mut func(*Definition)) []Definition {
		d := Definition{ID: "pms", Name: "Projects", Required: true, Enabled: true, Capabilities: []string{"work_management"}}
		mut(&d)
		return []Definition{d}
	}
	cases := []struct {
		name string
		defs []Definition
		want string
	}{
		{"nil registry", nil, "empty"},
		{"empty registry", []Definition{}, "empty"},
		{"empty ID", []Definition{{ID: "", Required: true, Enabled: true}}, "empty ID"},
		{"whitespace ID", []Definition{{ID: "  \t", Required: true, Enabled: true}}, "empty ID"},
		{"duplicate ID", []Definition{pms(func(*Definition) {})[0], pms(func(*Definition) {})[0]}, "twice"},
		{"missing PMS (only unknown)", []Definition{{ID: "aims", Required: true, Enabled: true}}, "unknown solution"},
		{"disabled PMS", pms(func(d *Definition) { d.Enabled = false }), "enabled"},
		{"PMS not required", pms(func(d *Definition) { d.Required = false }), "required"},
		{"unknown solution next to PMS", append(pms(func(*Definition) {}), Definition{ID: "hazop", Required: false, Enabled: true}), "unknown solution"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.defs)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}
