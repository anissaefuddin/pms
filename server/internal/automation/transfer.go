package automation

import (
	"context"
	"encoding/json"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Format names the export file so an import can refuse anything else.
const Format = "pms.automation.v1"

// Export is a rule's latest definition as a file: no ids, no owner, no run history. Importing it creates a draft in the target Space.
type Export struct {
	Format     string          `json:"format"`
	Name       string          `json:"name"`
	Definition json.RawMessage `json:"definition"`
}

// ExportRule returns the latest definition of a rule for people who can edit its Space.
func ExportRule(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID string) (Export, error) {
	rule, err := Get(ctx, pool, actor, ruleID)
	if err != nil {
		return Export{}, err
	}
	if len(rule.Versions) == 0 {
		return Export{}, items.Invalid("the rule has no version to export")
	}
	return Export{Format: Format, Name: rule.Name, Definition: rule.Versions[0].Definition}, nil
}

// ImportRule creates a disabled draft in a Space from an exported file. The definition is validated like any other draft, and its
// references must exist in the target Space.
func ImportRule(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID string, in Export) (Rule, error) {
	if in.Format != Format {
		return Rule{}, items.InvalidField{Field: "format", Message: "this file is not a rule export"}
	}
	if len(in.Definition) == 0 || !json.Valid(in.Definition) {
		return Rule{}, items.InvalidField{Field: "definition", Message: "the definition is missing"}
	}
	name := in.Name
	if name == "" {
		name = "Imported rule"
	}
	return Create(ctx, pool, actor, spaceID, name, nil, in.Definition)
}
