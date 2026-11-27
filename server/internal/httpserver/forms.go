package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/anissaefuddin/pms/server/internal/forms"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const formSelect = `SELECT f.version, jsonb_build_object('id',f.id,'space_id',f.space_id,'name',f.name,'description',f.description,
 'definition',f.definition,'response',f.response,'access',f.access,'public_token',f.public_token,'version',f.version,
 'status',CASE WHEN f.published_at IS NULL THEN 'draft' ELSE 'published' END,
 'created_by',jsonb_build_object('id',c.id,'name',concat_ws(' ',c.first_name,c.last_name)),
 'updated_by',jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)),
 'created_at',f.created_at,'updated_at',f.updated_at)
FROM request_forms f JOIN users c ON c.id=f.created_by JOIN users u ON u.id=f.updated_by `

func (s *Server) formRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/forms", s.requireUser(s.listAllForms))
	mux.HandleFunc("PUT /api/v1/forms/{formId}/access", s.requireUser(s.setFormAccess))
	mux.HandleFunc("POST /api/v1/forms/{formId}/publish", s.requireUser(s.publishForm))
	mux.HandleFunc("POST /api/v1/forms/{formId}/unpublish", s.requireUser(s.unpublishForm))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/forms", s.requireUser(s.listForms))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/forms", s.requireUser(s.writeForm))
	mux.HandleFunc("GET /api/v1/forms/{formId}", s.requireUser(s.writeForm))
	mux.HandleFunc("PUT /api/v1/forms/{formId}", s.requireUser(s.writeForm))
	mux.HandleFunc("DELETE /api/v1/forms/{formId}", s.requireUser(s.writeForm))
	mux.HandleFunc("GET /api/v1/forms/available", s.requireUser(s.listAvailableForms))
	mux.HandleFunc("GET /api/v1/me/requests", s.requireUser(s.listMyRequests))
	mux.HandleFunc("GET /api/v1/forms/{formId}/definition", s.requireUser(s.getFormDefinition))
	mux.HandleFunc("POST /api/v1/forms/{formId}/submissions", s.requireUser(s.submitForm))
	s.publicFormRoutes(mux)
}

func writeFormError(w http.ResponseWriter, err error) {
	if errors.Is(err, items.ErrAssignForbidden) {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	var invalid forms.Error
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &invalid):
		writeItemError(w, items.InvalidField{Field: invalid.Field, Message: invalid.Message})
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		writeProblem(w, http.StatusConflict, "name_taken", "a form with that name already exists in this Space")
	case errors.As(err, &pgErr) && pgErr.Code == "40001":
		writeItemError(w, items.ErrConflict)
	default:
		writeItemError(w, err)
	}
}

// formSpace requires the actor to manage forms in a Space: no access is 404, view-only is 403.
func formSpace(ctx context.Context, tx pgx.Tx, actor policy.Actor, spaceID uuid.UUID) (policy.SpaceAccess, error) {
	decision, access, err := policy.CanOnSpace(ctx, tx, actor, policy.Edit, spaceID.String())
	switch {
	case err != nil:
		return access, err
	case decision == policy.NotFound:
		return access, items.ErrNotFound
	case decision == policy.Forbidden:
		return access, items.ErrForbidden
	}
	return access, nil
}

func (s *Server) listForms(w http.ResponseWriter, r *http.Request) {
	spaceID, err := uuid.Parse(r.PathValue("spaceId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := formSpace(ctx, tx, actorOf(r), spaceID); err != nil {
		writeItemError(w, err)
		return
	}
	out, err := formSummaries(ctx, tx, "f.space_id=ANY($1::uuid[])", []uuid.UUID{spaceID})
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]json.RawMessage{"items": out})
}

// formSummaries lists forms matching a condition on $1, for the Manage forms views.
func formSummaries(ctx context.Context, tx pgx.Tx, where string, arg any) (json.RawMessage, error) {
	var out json.RawMessage
	err := tx.QueryRow(ctx, `
SELECT COALESCE(jsonb_agg(jsonb_build_object('id',f.id,'space_id',f.space_id,'space_name',sp.name,'name',f.name,'description',f.description,
 'access',f.access,'public_token',f.public_token,'status',CASE WHEN f.published_at IS NULL THEN 'draft' ELSE 'published' END,
 'question_count',(SELECT count(*) FROM jsonb_array_elements(f.definition->'pages') p, jsonb_array_elements(p->'elements') e WHERE e->>'kind'<>'section'),
 'updated_by',jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)),'updated_at',f.updated_at)
 ORDER BY lower(sp.name),lower(f.name),f.id),'[]'::jsonb)
FROM request_forms f JOIN users u ON u.id=f.updated_by JOIN spaces sp ON sp.id=f.space_id AND sp.deleted_at IS NULL WHERE `+where, arg).Scan(&out)
	return out, err
}

func (s *Server) listAllForms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	visible, err := policy.VisibleSpaces(ctx, tx, actorOf(r), "")
	if err != nil {
		writeItemError(w, err)
		return
	}
	manage := []uuid.UUID{}
	for _, access := range visible {
		if id, err := uuid.Parse(access.ID); err == nil && access.Kind == "shared" && access.Caps.Has(policy.Edit) {
			manage = append(manage, id)
		}
	}
	out, err := formSummaries(ctx, tx, "f.space_id=ANY($1::uuid[])", manage)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]json.RawMessage{"items": out})
}

// setFormAccess changes who may fill the form. Publishing a form to everyone is sharing, so it needs the Space's share capability.
func (s *Server) setFormAccess(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("formId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	var in struct {
		Access string `json:"access"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Access != "private" && in.Access != "internal" {
		writeItemError(w, items.InvalidField{Field: "access", Message: "access must be private or internal"})
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var spaceID uuid.UUID
	if err = tx.QueryRow(ctx, "SELECT space_id FROM request_forms WHERE id=$1", id).Scan(&spaceID); errors.Is(err, pgx.ErrNoRows) {
		err = items.ErrNotFound
	}
	if err == nil {
		_, err = formSpace(ctx, tx, actorOf(r), spaceID)
	}
	if err == nil {
		var decision policy.Decision
		if decision, _, err = policy.CanOnSpace(ctx, tx, actorOf(r), policy.Share, spaceID.String()); err == nil && decision != policy.Allowed {
			err = items.ErrForbidden
		}
	}
	if err == nil {
		_, err = tx.Exec(ctx, "UPDATE request_forms SET access=$2 WHERE id=$1", id, in.Access)
	}
	var out items.Detail
	if err == nil {
		err = tx.QueryRow(ctx, formSelect+"WHERE f.id=$1", id).Scan(&out.Version, &out.Body)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeFormError(w, err)
		return
	}
	writeItem(w, http.StatusOK, out, nil)
}

func (s *Server) publishForm(w http.ResponseWriter, r *http.Request) { s.setFormPublished(w, r, true) }
func (s *Server) unpublishForm(w http.ResponseWriter, r *http.Request) {
	s.setFormPublished(w, r, false)
}

// setFormPublished moves a form between draft and published. Publishing needs at least one question, so a requester
// never lands on an empty form. The version goes up, so an open builder sees the change as a new version.
func (s *Server) setFormPublished(w http.ResponseWriter, r *http.Request, publish bool) {
	id, err := uuid.Parse(r.PathValue("formId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var spaceID uuid.UUID
	var definition []byte
	if err = tx.QueryRow(ctx, "SELECT space_id,definition FROM request_forms WHERE id=$1", id).Scan(&spaceID, &definition); errors.Is(err, pgx.ErrNoRows) {
		err = items.ErrNotFound
	}
	if err == nil {
		_, err = formSpace(ctx, tx, actorOf(r), spaceID)
	}
	if err == nil && publish {
		var def forms.Definition
		if err = json.Unmarshal(definition, &def); err == nil && !hasQuestion(def) {
			err = items.InvalidField{Field: "definition", Message: "add at least one question before publishing"}
		}
	}
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE request_forms SET published_at=CASE WHEN $2 THEN COALESCE(published_at, now()) ELSE NULL END,
version=version+1,updated_by=$3::uuid WHERE id=$1`, id, publish, actorOf(r).ID)
	}
	var out items.Detail
	if err == nil {
		err = tx.QueryRow(ctx, formSelect+"WHERE f.id=$1", id).Scan(&out.Version, &out.Body)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeFormError(w, err)
		return
	}
	writeItem(w, http.StatusOK, out, nil)
}

func hasQuestion(def forms.Definition) bool {
	for _, page := range def.Pages {
		for _, el := range page.Elements {
			if el.Kind != forms.Section {
				return true
			}
		}
	}
	return false
}

func (s *Server) writeForm(w http.ResponseWriter, r *http.Request) {
	creating, reading, deleting := r.Method == http.MethodPost, r.Method == http.MethodGet, r.Method == http.MethodDelete
	rawID := r.PathValue("formId")
	if creating {
		rawID = r.PathValue("spaceId")
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	version := 0
	if r.Method == http.MethodPut {
		var ok bool
		if version, ok = ifMatchVersion(w, r); !ok {
			return
		}
	}
	var in struct {
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Definition  forms.Definition `json:"definition"`
		Response    forms.Response   `json:"response"`
	}
	if (creating || r.Method == http.MethodPut) && !decode(w, r, &in) {
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	actor := actorOf(r)
	formID, spaceID := id, id
	if !creating {
		var current int
		err = tx.QueryRow(ctx, "SELECT space_id,version FROM request_forms WHERE id=$1", id).Scan(&spaceID, &current)
		if errors.Is(err, pgx.ErrNoRows) {
			err = items.ErrNotFound
		}
		if err == nil {
			_, err = formSpace(ctx, tx, actor, spaceID)
		}
		if err == nil && r.Method == http.MethodPut && current != version {
			err = items.ErrConflict
		}
	} else {
		formID, err = uuid.NewV7()
		if err == nil {
			var access policy.SpaceAccess
			if access, err = formSpace(ctx, tx, actor, spaceID); err == nil && access.Kind != "shared" {
				err = items.InvalidField{Field: "space_id", Message: "forms live in shared Spaces"}
			}
		}
	}
	if err == nil && deleting {
		_, err = tx.Exec(ctx, "DELETE FROM request_forms WHERE id=$1", formID)
	} else if err == nil && !reading {
		err = saveForm(ctx, tx, actor, formID, spaceID, creating, in.Name, in.Description, in.Definition, in.Response)
	}
	var out items.Detail
	if err == nil && !deleting {
		err = tx.QueryRow(ctx, formSelect+"WHERE f.id=$1", formID).Scan(&out.Version, &out.Body)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeFormError(w, err)
		return
	}
	if deleting {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status := http.StatusOK
	if creating {
		status = http.StatusCreated
	}
	writeItem(w, status, out, nil)
}

func saveForm(ctx context.Context, tx pgx.Tx, actor policy.Actor, id, spaceID uuid.UUID, creating bool, name, description string, def forms.Definition, resp forms.Response) error {
	name, description, err := forms.ValidateName(name, description)
	if err != nil {
		return err
	}
	if err := forms.ValidateDefinition(def); err != nil {
		return err
	}
	if err := forms.ValidateResponse(def, resp); err != nil {
		return err
	}
	if resp.AssigneeIDs == nil {
		resp.AssigneeIDs = []uuid.UUID{}
	}
	if err := checkDestination(ctx, tx, actor, spaceID, resp); err != nil {
		return err
	}
	if !creating {
		if err := requireFormAssignAuthority(ctx, tx, actor, id, def, resp); err != nil {
			return err
		}
	}
	definition, _ := json.Marshal(def)
	response, _ := json.Marshal(resp)
	if creating {
		_, err = tx.Exec(ctx, `INSERT INTO request_forms(id,space_id,name,description,definition,response,created_by,updated_by)
VALUES($1,$2,$3,$4,$5,$6,$7::uuid,$7::uuid)`, id, spaceID, name, description, definition, response, actor.ID)
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE request_forms SET name=$2,description=$3,definition=$4,response=$5,updated_by=$6::uuid,version=version+1 WHERE id=$1`,
		id, name, description, definition, response, actor.ID)
	return err
}

// checkDestination makes sure the form author may create items where submissions will land, and that the
// status, assignees, and approvers exist. Submitters have no rights there, so the author vouches for the destination.
func checkDestination(ctx context.Context, tx pgx.Tx, actor policy.Actor, formSpace uuid.UUID, resp forms.Response) error {
	const spaceField, parentField = "response.location.space_id", "response.location.parent_item_id"
	space := resp.Location.SpaceID.String()
	decision, access, err := policy.CanOnSpace(ctx, tx, actor, policy.Create, space)
	if err != nil {
		return err
	}
	if decision != policy.Allowed || access.Kind != "shared" {
		return forms.Error{Field: spaceField, Message: "choose a shared Space where you can create items"}
	}
	// B3-12: the destination must have a verified owner, the same owner as the form, and an active Customer if it is client-owned.
	var scope string
	var client, customerStatus *string
	if err := tx.QueryRow(ctx, `SELECT s.scope_state, s.client_id::text, c.status FROM spaces s LEFT JOIN clients c ON c.id=s.client_id WHERE s.id=$1`,
		space).Scan(&scope, &client, &customerStatus); err != nil {
		return err
	}
	if scope != "internal" && scope != "client" {
		return forms.Error{Field: spaceField, Message: "choose a Space with a verified owner"}
	}
	if customerStatus != nil && *customerStatus != "active" {
		return forms.Error{Field: spaceField, Message: "the customer is archived and receives no new work"}
	}
	var sameOwner bool
	if err := tx.QueryRow(ctx, `SELECT a.scope_state=b.scope_state AND a.client_id IS NOT DISTINCT FROM b.client_id
FROM spaces a JOIN spaces b ON b.id=$2 WHERE a.id=$1`, space, formSpace).Scan(&sameOwner); err != nil {
		return err
	}
	if !sameOwner {
		return forms.Error{Field: spaceField, Message: "the destination must belong to the same owner as the form"}
	}
	var workflow uuid.UUID
	if parent := resp.Location.ParentItemID; parent != nil {
		var kind string
		var here bool
		err := tx.QueryRow(ctx, `
SELECT i.type::text, COALESCE(i.default_workflow_id,s.workflow_id,(SELECT id FROM workflows WHERE is_default)),
  EXISTS(SELECT 1 FROM item_locations l WHERE l.item_id=i.id AND l.space_id=$2 AND l.deleted_root_id IS NULL)
FROM items i LEFT JOIN statuses s ON s.id=i.status_id WHERE i.id=$1 AND i.deleted_at IS NULL`, *parent, resp.Location.SpaceID).Scan(&kind, &workflow, &here)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !here) {
			return forms.Error{Field: parentField, Message: "choose a project or folder in that Space"}
		}
		if err != nil {
			return err
		}
		if kind == "task" && resp.ItemType != "task" {
			return forms.Error{Field: parentField, Message: "a task can contain only tasks"}
		}
		caps, err := policy.ItemCaps(ctx, tx, actor, parent.String())
		if err != nil {
			return err
		}
		if !caps.Has(policy.Create) {
			return forms.Error{Field: parentField, Message: "you cannot create items inside that project or folder"}
		}
	} else if err := tx.QueryRow(ctx, "SELECT id FROM workflows WHERE is_default").Scan(&workflow); err != nil {
		return err
	}
	if resp.StatusID != nil {
		var ok bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM statuses WHERE id=$1 AND workflow_id=$2)", *resp.StatusID, workflow).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return forms.Error{Field: "response.status_id", Message: "the status must belong to the workflow of the destination"}
		}
	}
	if err := checkRouting(ctx, tx, actor, resp); err != nil {
		return err
	}
	people := append([]uuid.UUID{}, resp.AssigneeIDs...)
	field := "response.assignee_ids"
	if resp.Approval != nil {
		people, field = append(people, resp.Approval.ApproverIDs...), "response.approval.approver_ids"
	}
	var active int
	if err := tx.QueryRow(ctx, "SELECT count(DISTINCT id) FROM users WHERE status='active' AND id=ANY($1::uuid[])", people).Scan(&active); err != nil {
		return err
	}
	if distinct := len(uniqueIDs(people)); active != distinct {
		return forms.Error{Field: field, Message: "assignees and approvers must be active users"}
	}
	// A client destination takes only active members of its Customer as assignees and approvers (B3-06, B3-12).
	if client != nil {
		var members int
		if err := tx.QueryRow(ctx, `SELECT count(DISTINCT u.id) FROM users u JOIN client_memberships m ON m.user_id=u.id AND m.client_id=$1::uuid AND m.status='active'
WHERE u.status='active' AND u.user_type IN ('admin','regular') AND u.id=ANY($2::uuid[])`, *client, people).Scan(&members); err != nil {
			return err
		}
		if members != len(uniqueIDs(people)) {
			return forms.Error{Field: field, Message: "assignees and approvers must be active members of the destination's customer"}
		}
	}
	if resp.Approval == nil {
		return nil
	}
	// Approvers get a request on the created item, so they must be able to open it there.
	rows, err := tx.Query(ctx, "SELECT id::text,user_type::text FROM users WHERE id=ANY($1::uuid[])", resp.Approval.ApproverIDs)
	if err != nil {
		return err
	}
	approvers, err := policy.CollectActors(rows)
	if err != nil {
		return err
	}
	for _, approver := range approvers {
		var sees bool
		if parent := resp.Location.ParentItemID; parent != nil {
			caps, err := policy.ItemCaps(ctx, tx, approver, parent.String())
			if err != nil {
				return err
			}
			sees = caps.Has(policy.View)
		} else {
			found, ok, err := policy.SpaceAccessFor(ctx, tx, approver, space)
			if err != nil {
				return err
			}
			sees = ok && found.Caps.Has(policy.View)
		}
		if !sees {
			return forms.Error{Field: "response.approval.approver_ids", Message: "approvers must be able to see the destination"}
		}
	}
	return nil
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	out := []uuid.UUID{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// checkRouting (FORM-09): every item type a form can create is active and of the class the form creates, and every
// mapped custom field lives in the destination Space and can take that kind of answer.
func checkRouting(ctx context.Context, tx pgx.Tx, actor policy.Actor, resp forms.Response) error {
	keys := []string{}
	if resp.TypeKey != nil {
		keys = append(keys, *resp.TypeKey)
	}
	for _, route := range resp.Routes {
		keys = append(keys, route.TypeKey)
	}
	for _, key := range keys {
		var base string
		var active bool
		var typeSpace *string
		err := tx.QueryRow(ctx, "SELECT base::text, active, space_id::text FROM item_type_defs WHERE key=$1", key).Scan(&base, &active, &typeSpace)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!active || base != resp.ItemType)) {
			return forms.Error{Field: "response.type_key", Message: "item type " + key + " must be active and of the class " + resp.ItemType}
		}
		// A Space type belongs to one Space: a form whose items land elsewhere would fail on every submission.
		if err == nil && typeSpace != nil && *typeSpace != resp.Location.SpaceID.String() {
			return forms.Error{Field: "response.type_key", Message: "item type " + key + " belongs to another Space"}
		}
		if err != nil {
			return err
		}
	}
	for i, route := range resp.Routes {
		if route.BlueprintID == nil {
			continue
		}
		var ok bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM blueprints WHERE id=$1 AND space_id=$2)", *route.BlueprintID, resp.Location.SpaceID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return forms.Error{Field: fmt.Sprintf("response.routes[%d].blueprint_id", i), Message: "choose a blueprint of the destination Space"}
		}
		// The author must be allowed to use blueprints there: the submission will apply it with the author's vouching.
		if decision, _, err := policy.CanOnSpace(ctx, tx, actor, policy.UseBlueprint, resp.Location.SpaceID.String()); err != nil {
			return err
		} else if decision != policy.Allowed {
			return forms.Error{Field: fmt.Sprintf("response.routes[%d].blueprint_id", i), Message: "you need the use-blueprint right in the destination Space"}
		}
	}
	for _, id := range resp.FieldMap {
		var kind string
		err := tx.QueryRow(ctx, "SELECT type FROM custom_fields WHERE id=$1 AND (space_id=$2 OR space_id IS NULL) AND archived_at IS NULL", id, resp.Location.SpaceID).Scan(&kind)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !slices.Contains(mappableFieldTypes, kind)) {
			return forms.Error{Field: "response.field_map", Message: "map answers to text, number, date or select fields of the destination Space"}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

var mappableFieldTypes = []string{"text", "number", "date", "single_select", "multi_select"}
