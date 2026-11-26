// Package seed creates opt-in starter configuration for an instance (PMS_SEED, comma separated), so a deploy does not
// depend on someone clicking it in. Each seed runs once: its name is recorded in seed_runs in the same transaction as
// the data, so a field an admin later renames, archives or re-grants is not created again.
package seed

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type field struct {
	input items.CustomFieldCreate
	// restricted gives the seed's group edit access and hides the field from everyone else (CF-D03). Workspace admins
	// still manage it and can add people to the group.
	restricted bool
}

type definition struct {
	group  string // created or reused for restricted fields
	fields []field
}

var zero = 0

var seeds = map[string]definition{
	// MVP-03/MVP-04 (docs/roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md §3.1): contract reference and the project's Pydio folder.
	// The contract value is visible only to the Contract Managers group. Not project_budget: that flag feeds budget reports,
	// a contract is not a budget. Pydio Folder is a text field; the item panel shows an http(s) value as a link.
	// A seed runs once per database, so a field added here later does not reach instances that already ran it.
	"project-master": {group: "Contract Managers", fields: []field{
		{input: items.CustomFieldCreate{Name: "Contract/PO Number", Type: "text"}},
		{input: items.CustomFieldCreate{Name: "Pydio Folder", Type: "text"}},
		{input: items.CustomFieldCreate{Name: "Contract Value", Type: "currency", Config: items.CustomFieldConfig{Currency: "IDR", Decimals: &zero}}, restricted: true},
	}},
}

// Parse splits PMS_SEED and rejects an unknown name, so a typo fails the start instead of silently seeding nothing.
func Parse(value string) ([]string, error) {
	var names []string
	for _, n := range strings.Split(value, ",") {
		n = strings.TrimSpace(n)
		if n == "" || slices.Contains(names, n) {
			continue
		}
		if _, ok := seeds[n]; !ok {
			return nil, fmt.Errorf("PMS_SEED: unknown seed %q", n)
		}
		names = append(names, n)
	}
	return names, nil
}

// Apply runs the named seeds that have not run on this database yet.
func Apply(ctx context.Context, pool *pgxpool.Pool, names []string) error {
	for _, name := range names {
		def, ok := seeds[name]
		if !ok {
			return fmt.Errorf("unknown seed %q", name)
		}
		applied, err := apply(ctx, pool, name, def)
		if err != nil {
			return fmt.Errorf("seed %s: %w", name, err)
		}
		if applied {
			slog.Info("seed applied", "seed", name)
		}
	}
	return nil
}

func apply(ctx context.Context, pool *pgxpool.Pool, name string, def definition) (bool, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	// The row is the lock: a second instance starting at the same time waits here, then sees the seed as done.
	tag, err := tx.Exec(ctx, "INSERT INTO seed_runs(name) VALUES ($1) ON CONFLICT (name) DO NOTHING", name)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err
	}
	system := policy.Actor{Type: policy.Admin}
	var group string
	for _, f := range def.fields {
		// A Workspace field an admin already made by hand is reused rather than duplicated.
		var id string
		var hasGrants bool
		err := tx.QueryRow(ctx, `SELECT id::text, EXISTS(SELECT 1 FROM custom_field_grants g WHERE g.field_id=f.id) FROM custom_fields f
WHERE f.space_id IS NULL AND lower(f.name)=lower($1) ORDER BY f.archived_at NULLS FIRST, f.position LIMIT 1`, f.input.Name).Scan(&id, &hasGrants)
		if err == pgx.ErrNoRows {
			created, cerr := items.CreateCustomFieldTx(ctx, tx, system, "", f.input)
			if cerr != nil {
				return false, fmt.Errorf("field %q: %w", f.input.Name, cerr)
			}
			id, err = created.ID, nil
		}
		if err != nil {
			return false, err
		}
		if !f.restricted || hasGrants { // existing grants are the admin's decision
			continue
		}
		if group == "" {
			if err := tx.QueryRow(ctx, `INSERT INTO groups(name) VALUES ($1)
ON CONFLICT ((lower(name))) DO UPDATE SET name=groups.name RETURNING id::text`, def.group).Scan(&group); err != nil {
				return false, err
			}
		}
		if _, err := tx.Exec(ctx, "INSERT INTO custom_field_grants(field_id,subject_kind,subject_id,level) VALUES ($1,'group',$2,'edit')", id, group); err != nil {
			return false, err
		}
		// Same action the grants API records (D-90), so the audit log shows who restricted the field: the seed.
		if _, err := tx.Exec(ctx, `INSERT INTO audit_events(actor_name,action,target_type,target_id,details)
VALUES ('seed','custom_field.grants_changed','custom_field',$1,jsonb_build_object('seed',$2::text,'group',$3::text,'level','edit'))`, id, name, def.group); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}
