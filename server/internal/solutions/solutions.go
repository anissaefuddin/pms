// Package solutions is the static registry of solutions this build ships (ARCH-05 in docs/architecture/ADR-CLIENT-SCOPE-MVP.md).
// It is code-managed metadata: no database, no installer, no plugin loading. Capabilities describe a solution;
// they never grant RBAC permissions, which stay in package policy.
package solutions

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// PMS is the only solution this release recognizes. A second one needs a code change here and in Validate.
const PMS = "pms"

// Definition is one solution as this build knows it.
type Definition struct {
	ID           string
	Name         string
	Required     bool
	Enabled      bool
	Capabilities []string
}

// Builtin returns a fresh copy of the registry. Callers may change what they receive without affecting later calls.
func Builtin() []Definition {
	return []Definition{{
		ID: PMS, Name: "Projects", Required: true, Enabled: true,
		Capabilities: []string{"work_management"},
	}}
}

// Validate rejects a registry that the build cannot run safely: nil or empty, blank or duplicate IDs, unknown IDs,
// and a PMS that is missing, disabled, or not required.
func Validate(defs []Definition) error {
	if len(defs) == 0 {
		return errors.New("solutions: registry is empty")
	}
	seen := map[string]bool{}
	for _, d := range defs {
		if strings.TrimSpace(d.ID) == "" {
			return errors.New("solutions: a solution has an empty ID")
		}
		if seen[d.ID] {
			return fmt.Errorf("solutions: solution %q is registered twice", d.ID)
		}
		seen[d.ID] = true
		if d.ID != PMS {
			return fmt.Errorf("solutions: unknown solution %q (this release recognizes only %q)", d.ID, PMS)
		}
	}
	pms := defs[slices.IndexFunc(defs, func(d Definition) bool { return d.ID == PMS })]
	switch {
	case !pms.Enabled:
		return fmt.Errorf("solutions: %q must be enabled", PMS)
	case !pms.Required:
		return fmt.Errorf("solutions: %q must be required", PMS)
	}
	return nil
}
