package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/spaces"
	"github.com/anissaefuddin/pms/server/internal/spacetemplates"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxTemplateItems = 500

var uuidInText = regexp.MustCompile(`[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}`)

// savedTemplate loads a saved template the actor may use: their own, or one whose source Space they can see. Titles in it come from
// that Space, so nobody else may read them.
func (s *Server) savedTemplate(ctx context.Context, actor policy.Actor, id string) (spacetemplates.Template, bool, error) {
	list, err := s.savedTemplates(ctx, actor, id)
	if err != nil || len(list) == 0 {
		return spacetemplates.Template{}, false, err
	}
	return list[0], true, nil
}

func (s *Server) savedTemplates(ctx context.Context, actor policy.Actor, id string) ([]spacetemplates.Template, error) {
	var only *uuid.UUID
	if id != "" {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, nil
		}
		only = &parsed
	}
	visible, err := policy.VisibleSpaces(ctx, s.pool, actor, "")
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, space := range visible {
		if space.Caps.Has(policy.View) {
			ids = append(ids, space.ID)
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text, body, created_by=$1::uuid FROM saved_space_templates
WHERE ($2::uuid IS NULL OR id=$2) AND (created_by=$1::uuid OR source_space_id=ANY(ARRAY(SELECT unnest($3::text[])::uuid)))
ORDER BY body->>'name', id LIMIT 200`, actor.ID, only, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (spacetemplates.Template, error) {
		var t spacetemplates.Template
		var body []byte
		if err := row.Scan(&t.ID, &body, &t.Mine); err != nil {
			return t, err
		}
		id, mine := t.ID, t.Mine
		err := json.Unmarshal(body, &t)
		t.ID, t.Mine, t.Saved = id, mine, true
		return t, err
	})
}

// saveSpaceTemplate stores a Space's structure as a template. It needs Full access, because the copy can be used later without the
// Space's own sharing, and a user type that may create Spaces.
func (s *Server) saveSpaceTemplate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SpaceID     string `json:"space_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 120 || utf8.RuneCountInString(in.Description) > 500 || strings.ContainsRune(in.Name+in.Description, 0) {
		writeError(w, http.StatusUnprocessableEntity, "name must be 1 to 120 characters and description at most 500")
		return
	}
	ctx, actor := r.Context(), actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	t, err := templateOfSpace(ctx, tx, actor, in.SpaceID)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	t.Name, t.Description = in.Name, in.Description
	body, err := json.Marshal(t)
	if err == nil {
		err = tx.QueryRow(ctx, "INSERT INTO saved_space_templates(created_by,source_space_id,body) VALUES ($1::uuid,$2::uuid,$3) RETURNING id::text", actor.ID, in.SpaceID, body).Scan(&t.ID)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	t.Saved, t.Mine = true, true
	writeJSON(w, http.StatusCreated, t.Summary())
}

// deleteSpaceTemplate removes a saved template. Only its author may; Spaces made from it are not touched.
func (s *Server) deleteSpaceTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("templateId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	tag, err := s.pool.Exec(r.Context(), "DELETE FROM saved_space_templates WHERE id=$1 AND created_by=$2::uuid", id, actorOf(r).ID)
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func templateOfSpace(ctx context.Context, tx pgx.Tx, actor policy.Actor, spaceID string) (spacetemplates.Template, error) {
	t := spacetemplates.Template{Format: spacetemplates.Format, Version: 1, Category: "Saved", Forms: []spacetemplates.Form{}, Projects: []spacetemplates.Project{}}
	if !spaces.IsID(spaceID) {
		return t, spaces.ErrNotFound
	}
	if actor.Type != policy.Admin && actor.Type != policy.Regular {
		return t, spaces.ErrForbidden
	}
	decision, access, err := policy.CanOnSpace(ctx, tx, actor, policy.Share, spaceID)
	if err != nil {
		return t, err
	}
	if decision == policy.NotFound {
		return t, spaces.ErrNotFound
	}
	if decision != policy.Allowed || access.Kind == "personal" {
		return t, spaces.ErrForbidden
	}
	if t.CustomFields, err = templateFields(ctx, tx, spaceID); err != nil {
		return t, err
	}
	if t.Items, err = templateItems(ctx, tx, actor, spaceID); err != nil {
		return t, err
	}
	rows, err := tx.Query(ctx, `SELECT type::text, name, config FROM views WHERE scope_space_id=$1::uuid AND scope_item_id IS NULL AND visibility='public'
AND (type::text,name) NOT IN (('table','Table'),('board','Board'),('gantt','Gantt'),('calendar','Calendar'),('files','Files')) ORDER BY position`, spaceID)
	if err != nil {
		return t, err
	}
	if t.Views, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (spacetemplates.View, error) {
		var v spacetemplates.View
		var config []byte
		if err := row.Scan(&v.Type, &v.Name, &config); err != nil {
			return v, err
		}
		// A setting that names a record (a field, a person) belongs to the old Space, so that view starts with default settings.
		if !uuidInText.Match(config) {
			return v, json.Unmarshal(config, &v.Config)
		}
		return v, nil
	}); err != nil {
		return t, err
	}
	rows, err = tx.Query(ctx, "SELECT base_type, workflow_id::text FROM space_default_workflows WHERE space_id=$1::uuid", spaceID)
	if err != nil {
		return t, err
	}
	t.DefaultWorkflows = map[string]string{}
	var base, workflow string
	_, err = pgx.ForEachRow(rows, []any{&base, &workflow}, func() error { t.DefaultWorkflows[base] = workflow; return nil })
	return t, err
}

// templateFields copies the Space's own field definitions. A restricted field is left out, because the copy would not carry its
// restriction; so is a formula that reads one, and a relation to an item type that exists only in this Space.
func templateFields(ctx context.Context, tx pgx.Tx, spaceID string) ([]spacetemplates.Field, error) {
	rows, err := tx.Query(ctx, `WITH restricted AS (
  SELECT name FROM custom_fields f WHERE f.space_id=$1::uuid AND EXISTS(SELECT 1 FROM custom_field_grants g WHERE g.field_id=f.id))
SELECT f.name, f.type, f.options, f.config FROM custom_fields f
WHERE f.space_id=$1::uuid AND f.archived_at IS NULL AND NOT EXISTS(SELECT 1 FROM custom_field_grants g WHERE g.field_id=f.id)
  AND NOT (f.type='formula' AND EXISTS(SELECT 1 FROM restricted r WHERE position(lower(r.name) IN lower(f.config->>'expression'))>0))
  AND NOT (f.type='relation' AND EXISTS(SELECT 1 FROM item_type_defs d WHERE d.key=f.config->>'target_type_key' AND d.space_id IS NOT NULL))
ORDER BY f.type='formula', f.position`, spaceID)
	if err != nil {
		return nil, err
	}
	fields, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (spacetemplates.Field, error) {
		var f spacetemplates.Field
		var options []items.CustomFieldOption
		var config items.CustomFieldConfig
		if err := row.Scan(&f.Name, &f.Type, &options, &config); err != nil {
			return f, err
		}
		f.Options = []spacetemplates.Option{}
		for _, o := range options {
			f.Options = append(f.Options, spacetemplates.Option{Label: o.Label, Color: o.Color})
		}
		var err error
		f.Config, err = json.Marshal(config)
		return f, err
	})
	if fields == nil {
		fields = []spacetemplates.Field{}
	}
	return fields, err
}

// templateItems copies the item tree: kind, title, importance, duration, milestone flag and default view. An item placed twice in
// the Space is copied at its first place.
func templateItems(ctx context.Context, tx pgx.Tx, actor policy.Actor, spaceID string) ([]spacetemplates.Node, error) {
	rows, err := tx.Query(ctx, `
WITH RECURSIVE t(id, parent, path) AS (
  SELECT l.item_id, NULL::uuid, ARRAY[(l.rank COLLATE "C") || l.item_id::text] FROM item_locations l
  WHERE l.space_id=$1::uuid AND l.parent_item_id IS NULL AND l.deleted_root_id IS NULL
  UNION ALL
  SELECT l.item_id, t.id, t.path || ((l.rank COLLATE "C") || l.item_id::text)
  FROM t JOIN item_locations l ON l.parent_item_id=t.id AND l.space_id=$1::uuid AND l.deleted_root_id IS NULL
  WHERE cardinality(t.path) <= 30
)
SELECT i.id::text, COALESCE(t.parent::text,''), i.type::text, i.title, i.importance::text, i.duration_days, i.is_milestone, COALESCE(i.default_view_type::text,''), COALESCE(i.default_workflow_id::text,'')
FROM t JOIN items i ON i.id=t.id AND i.deleted_at IS NULL ORDER BY t.path`, spaceID)
	if err != nil {
		return nil, err
	}
	type source struct {
		id, parent string
		node       spacetemplates.Node
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (source, error) {
		var s source
		var importance *string
		err := row.Scan(&s.id, &s.parent, &s.node.Type, &s.node.Title, &importance, &s.node.DurationDays, &s.node.Milestone, &s.node.DefaultView, &s.node.WorkflowID)
		if importance != nil && *importance != "normal" {
			s.node.Importance = *importance
		}
		if s.node.Type == "task" {
			s.node.DefaultView = ""
		}
		return s, err
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(list))
	for i, s := range list {
		ids[i] = s.id
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		return nil, err
	}
	children, seen := map[string][]source{}, map[string]bool{}
	for _, s := range list {
		if seen[s.id] || !caps[s.id].Has(policy.View) {
			continue
		}
		seen[s.id] = true
		children[s.parent] = append(children[s.parent], s)
	}
	if len(seen) > maxTemplateItems {
		return nil, spaces.ValidationError{Message: "a template can hold at most 500 items"}
	}
	var build func(parent string) []spacetemplates.Node
	build = func(parent string) []spacetemplates.Node {
		out := []spacetemplates.Node{}
		for _, s := range children[parent] {
			s.node.Children = build(s.id)
			out = append(out, s.node)
		}
		return out
	}
	return build(""), nil
}
