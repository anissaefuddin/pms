package views

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrMissing = errors.New("view or container not found")
	ErrDenied  = errors.New("editing this view is forbidden")
	ErrDefault = errors.New("choose another default view first")
)

type Invalid string

func (e Invalid) Error() string { return string(e) }

type Scope struct {
	SpaceID *uuid.UUID `json:"scope_space_id"`
	ItemID  *uuid.UUID `json:"scope_item_id"`
}

type View struct {
	ID uuid.UUID `json:"id"`
	Scope
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Visibility string         `json:"visibility"`
	Position   int            `json:"position"`
	Default    bool           `json:"is_default"`
	Config     map[string]any `json:"config"`
}

type CreateInput struct {
	Scope
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Visibility string         `json:"visibility"`
	Config     map[string]any `json:"config"`
}

type UpdateInput struct {
	Name       *string         `json:"name"`
	Visibility *string         `json:"visibility"`
	Position   *int32          `json:"position"`
	Default    *bool           `json:"is_default"`
	Config     *map[string]any `json:"config"`
}

const columns = "id,scope_space_id,scope_item_id,type::text,name,visibility::text,position,is_default,config"
const sameScope = "scope_space_id IS NOT DISTINCT FROM $1::uuid AND scope_item_id IS NOT DISTINCT FROM $2::uuid"

func scan(row pgx.Row) (View, error) {
	var v View
	err := row.Scan(&v.ID, &v.SpaceID, &v.ItemID, &v.Type, &v.Name, &v.Visibility, &v.Position, &v.Default, &v.Config)
	return v, err
}

func authorize(ctx context.Context, tx pgx.Tx, actor policy.Actor, scope Scope) (policy.Caps, error) {
	if (scope.SpaceID == nil) == (scope.ItemID == nil) {
		return 0, Invalid("exactly one view scope is required")
	}
	var caps policy.Caps
	if scope.SpaceID != nil {
		a, _, err := policy.SpaceAccessFor(ctx, tx, actor, scope.SpaceID.String())
		if err != nil {
			return 0, err
		}
		caps = a.Caps
	} else {
		c, err := policy.ItemCaps(ctx, tx, actor, scope.ItemID.String())
		if err != nil {
			return 0, err
		}
		caps = c
		if caps.Has(policy.View) {
			var isContainer bool
			if err := tx.QueryRow(ctx, "SELECT type IN ('project','folder') FROM items WHERE id=$1", *scope.ItemID).Scan(&isContainer); err != nil {
				return 0, err
			}
			if !isContainer {
				return 0, ErrMissing
			}
		}
	}
	if !caps.Has(policy.View) {
		return 0, ErrMissing
	}
	return caps, nil
}

func List(ctx context.Context, tx pgx.Tx, actor policy.Actor, scope Scope) ([]View, error) {
	if _, err := authorize(ctx, tx, actor, scope); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, "SELECT "+columns+" FROM views WHERE "+sameScope+
		" AND (visibility='public' OR owner_id=$3::uuid) ORDER BY visibility,position,id", scope.SpaceID, scope.ItemID, actor.ID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (View, error) { return scan(row) })
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 80 || strings.ContainsRune(name, 0) {
		return "", Invalid("name must be 1 to 80 characters")
	}
	return name, nil
}

func checkVisibility(visibility string, caps policy.Caps) error {
	switch visibility {
	case "public":
		if !caps.Has(policy.Edit) {
			return ErrDenied
		}
	case "personal":
	default:
		return Invalid("visibility must be public or personal")
	}
	return nil
}

func Create(ctx context.Context, tx pgx.Tx, actor policy.Actor, in CreateInput) (View, error) {
	caps, err := authorize(ctx, tx, actor, in.Scope)
	if err != nil {
		return View{}, err
	}
	if err := checkVisibility(in.Visibility, caps); err != nil {
		return View{}, err
	}
	switch in.Type {
	case "table", "board", "gantt", "calendar":
	case "timelog", "files", "chart":
		var project bool
		if in.ItemID == nil {
			return View{}, Invalid("Timelog, files and chart views require a project")
		}
		if err := tx.QueryRow(ctx, "SELECT type='project' FROM items WHERE id=$1", *in.ItemID).Scan(&project); err != nil {
			return View{}, err
		}
		if !project {
			return View{}, Invalid("Timelog, files and chart views require a project")
		}
	default:
		return View{}, Invalid("type must be table, board, gantt, calendar, timelog, files, or chart")
	}
	name, err := cleanName(in.Name)
	if err != nil {
		return View{}, err
	}
	if in.Config == nil {
		in.Config = map[string]any{}
	}
	return scan(tx.QueryRow(ctx, `
INSERT INTO views(scope_space_id,scope_item_id,type,name,visibility,owner_id,position,config)
SELECT $1,$2,$3::view_type,$4,$5::view_visibility,$6::uuid,COALESCE(max(position)+1,0),$7::jsonb
FROM views WHERE `+sameScope+` AND visibility=$5::view_visibility
AND ($5::view_visibility='public' OR owner_id=$6::uuid)
RETURNING `+columns, in.SpaceID, in.ItemID, in.Type, name, in.Visibility, actor.ID, in.Config))
}

func read(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID) (View, policy.Caps, error) {
	v, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM views WHERE id=$1 AND (visibility='public' OR owner_id=$2::uuid)", id, actor.ID))
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, 0, ErrMissing
	}
	if err != nil {
		return View{}, 0, err
	}
	caps, err := authorize(ctx, tx, actor, v.Scope)
	return v, caps, err
}

func Duplicate(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, name, visibility string) (View, error) {
	v, _, err := read(ctx, tx, actor, id)
	if err != nil {
		return View{}, err
	}
	return Create(ctx, tx, actor, CreateInput{Scope: v.Scope, Type: v.Type, Name: name, Visibility: visibility, Config: v.Config})
}

func Update(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, in UpdateInput) (View, error) {
	v, caps, err := read(ctx, tx, actor, id)
	if err != nil {
		return View{}, err
	}
	if err = checkVisibility(v.Visibility, caps); err != nil {
		return View{}, err
	}
	if in.Name != nil {
		v.Name, err = cleanName(*in.Name)
		if err != nil {
			return View{}, err
		}
	}
	visibility := v.Visibility
	if in.Visibility != nil {
		visibility = *in.Visibility
		if err = checkVisibility(visibility, caps); err != nil {
			return View{}, err
		}
	}
	if (v.Default && visibility == "personal") || (v.Default && in.Default != nil && !*in.Default) {
		return View{}, ErrDefault
	}
	if in.Default != nil && *in.Default {
		if visibility != "public" {
			return View{}, Invalid("only public views may be default")
		}
		if _, err = tx.Exec(ctx, "UPDATE views SET is_default=false WHERE "+sameScope+" AND is_default AND id<>$3", v.SpaceID, v.ItemID, id); err != nil {
			return View{}, err
		}
		v.Default = true
	}
	if in.Config != nil {
		v.Config = *in.Config
	}
	if in.Position != nil && *in.Position < 0 {
		return View{}, Invalid("position must be non-negative")
	}
	position := v.Position
	if visibility != v.Visibility {
		if err := tx.QueryRow(ctx, "SELECT COALESCE(max(position)+1,0) FROM views WHERE "+sameScope+
			" AND visibility=$3::view_visibility AND ($3='public' OR owner_id=$4::uuid)", v.SpaceID, v.ItemID, visibility, actor.ID).Scan(&position); err != nil {
			return View{}, err
		}
	} else if in.Position != nil && int(*in.Position) != v.Position {
		position = int(*in.Position)
		var last int
		if err := tx.QueryRow(ctx, "SELECT COALESCE(max(position),0) FROM views WHERE "+sameScope+
			" AND visibility=$3::view_visibility AND ($3='public' OR owner_id=$4::uuid)",
			v.SpaceID, v.ItemID, visibility, actor.ID).Scan(&last); err != nil {
			return View{}, err
		}
		position = min(position, last)
		_, err = tx.Exec(ctx, `
UPDATE views SET position=CASE WHEN $5::integer < $6::integer THEN position+1 ELSE position-1 END
WHERE `+sameScope+` AND visibility=$3::view_visibility AND ($3='public' OR owner_id=$4::uuid)
AND id<>$7 AND position BETWEEN least($5::integer,$6::integer) AND greatest($5::integer,$6::integer)`,
			v.SpaceID, v.ItemID, visibility, actor.ID, position, v.Position, id)
		if err != nil {
			return View{}, err
		}
	}
	return scan(tx.QueryRow(ctx, `
UPDATE views SET name=$2,visibility=$3::view_visibility,
 owner_id=CASE WHEN $3='personal' THEN $4::uuid ELSE owner_id END,position=$5,is_default=$6,config=$7::jsonb
WHERE id=$1 RETURNING `+columns, id, v.Name, visibility, actor.ID, position, v.Default, v.Config))
}

func Delete(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID) error {
	v, caps, err := read(ctx, tx, actor, id)
	if err != nil {
		return err
	}
	if err = checkVisibility(v.Visibility, caps); err != nil {
		return err
	}
	if v.Default {
		return ErrDefault
	}
	_, err = tx.Exec(ctx, "DELETE FROM views WHERE id=$1", id)
	return err
}
