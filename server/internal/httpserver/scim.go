package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	stdmail "net/mail"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SCIM 2.0 provisioning (ENT-02, D-94). An identity provider holds a bearer token and creates, updates and deprovisions users and groups.
// Only the token's hash is stored. A SCIM call never acts as a person: its audit entries carry the label "scim:<token name>".
// shortcut: filters support only `<attr> eq "<value>"` (all that Okta, Entra ID and OneLogin send); add `and`/`co`/`sw` if a provider needs them.

const (
	scimSchemaUser   = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimSchemaGroup  = "urn:ietf:params:scim:schemas:core:2.0:Group"
	scimSchemaList   = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	scimSchemaError  = "urn:ietf:params:scim:api:messages:2.0:Error"
	scimSchemaPatch  = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	scimBase         = "/api/v1/scim/v2"
	scimMaxPage      = 200
	auditSCIMToken   = "scim.token_created"
	auditSCIMRevoked = "scim.token_revoked"
	auditSCIMUserNew = "scim.user_created"
	auditSCIMUserUpd = "scim.user_updated"
	auditSCIMUserOff = "scim.user_deprovisioned"
	auditSCIMUserOn  = "scim.user_reactivated"
	auditSCIMGroupNW = "scim.group_created"
	auditSCIMGroupUP = "scim.group_updated"
	auditSCIMGroupRM = "scim.group_deleted"
)

type scimToken struct {
	ID          string
	Name        string
	DefaultType policy.UserType
}

type scimErr struct {
	status  int
	scimTyp string
	detail  string
}

func (e scimErr) Error() string { return e.detail }

func scimWrite(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func scimFail(w http.ResponseWriter, err error) {
	var e scimErr
	var pg *pgconn.PgError
	switch {
	case errors.As(err, &e):
	case errors.Is(err, pgx.ErrNoRows):
		e = scimErr{404, "", "resource not found"}
	case errors.As(err, &pg) && pg.Code == "23505":
		e = scimErr{409, "uniqueness", "a resource with this value already exists"}
	case errors.As(err, &pg) && (pg.Code == "23514" || pg.Code == "22P02" || pg.Code == "23503"):
		e = scimErr{400, "invalidValue", "a value is not valid"}
	default:
		slog.Error("scim operation failed", "error", err)
		e = scimErr{500, "", "internal error"}
	}
	body := map[string]any{"schemas": []string{scimSchemaError}, "status": strconv.Itoa(e.status), "detail": e.detail}
	if e.scimTyp != "" {
		body["scimType"] = e.scimTyp
	}
	scimWrite(w, e.status, body)
}

func (s *Server) scimRoutes(mux *http.ServeMux) {
	h := func(fn func(http.ResponseWriter, *http.Request, scimToken)) http.HandlerFunc { return s.scimAuth(fn) }
	mux.HandleFunc("GET "+scimBase+"/ServiceProviderConfig", h(s.scimConfig))
	mux.HandleFunc("GET "+scimBase+"/ResourceTypes", h(s.scimResourceTypes))
	mux.HandleFunc("GET "+scimBase+"/Schemas", h(s.scimSchemas))
	mux.HandleFunc("GET "+scimBase+"/Users", h(s.scimListUsers))
	mux.HandleFunc("POST "+scimBase+"/Users", h(s.scimCreateUser))
	mux.HandleFunc("GET "+scimBase+"/Users/{id}", h(s.scimGetUser))
	mux.HandleFunc("PUT "+scimBase+"/Users/{id}", h(s.scimPutUser))
	mux.HandleFunc("PATCH "+scimBase+"/Users/{id}", h(s.scimPatchUser))
	mux.HandleFunc("DELETE "+scimBase+"/Users/{id}", h(s.scimDeleteUser))
	mux.HandleFunc("GET "+scimBase+"/Groups", h(s.scimListGroups))
	mux.HandleFunc("POST "+scimBase+"/Groups", h(s.scimCreateGroup))
	mux.HandleFunc("GET "+scimBase+"/Groups/{id}", h(s.scimGetGroup))
	mux.HandleFunc("PUT "+scimBase+"/Groups/{id}", h(s.scimPutGroup))
	mux.HandleFunc("PATCH "+scimBase+"/Groups/{id}", h(s.scimPatchGroup))
	mux.HandleFunc("DELETE "+scimBase+"/Groups/{id}", h(s.scimDeleteGroup))
	mux.HandleFunc("GET /api/v1/scim-tokens", s.requireUser(requireAdmin(s.scimListTokens)))
	mux.HandleFunc("POST /api/v1/scim-tokens", s.requireUser(requireAdmin(s.scimNewToken)))
	mux.HandleFunc("DELETE /api/v1/scim-tokens/{tokenId}", s.requireUser(requireAdmin(s.scimRevokeToken)))
}

// scimAuth resolves the bearer token. A bad or revoked token and a rate-limited one answer alike for every path, so nothing leaks which exist.
func (s *Server) scimAuth(next func(http.ResponseWriter, *http.Request, scimToken)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || raw == "" || len(raw) > 200 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="scim"`)
			scimFail(w, scimErr{401, "", "a bearer token is required"})
			return
		}
		sum := sha256.Sum256([]byte(raw))
		var t scimToken
		err := s.pool.QueryRow(r.Context(), `UPDATE scim_tokens SET last_used_at=CASE WHEN last_used_at IS NULL OR last_used_at < now()-interval '1 minute' THEN now() ELSE last_used_at END
WHERE token_hash=$1 AND revoked_at IS NULL RETURNING id::text,name,default_user_type::text`, sum[:]).Scan(&t.ID, &t.Name, &t.DefaultType)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				slog.Error("scim token lookup", "error", err)
			}
			w.Header().Set("WWW-Authenticate", `Bearer realm="scim"`)
			scimFail(w, scimErr{401, "", "the token is not valid"})
			return
		}
		if wait := s.scimLimiter.take(t.ID); wait > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int((wait+time.Second-1)/time.Second)))
			scimFail(w, scimErr{429, "", "too many requests"})
			return
		}
		next(w, r, t)
	}
}

// scimTx locks the singleton account first, like adminTx, so seat counts and the last-admin rule hold under concurrent calls.
func (s *Server) scimTx(r *http.Request, work func(context.Context, pgx.Tx) error) error {
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT 1 FROM account WHERE id=1 FOR UPDATE"); err != nil {
		return err
	}
	if err = work(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func scimLabel(t scimToken) string { return "scim:" + t.Name }

func scimBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		scimFail(w, scimErr{400, "invalidSyntax", "the request body is not valid JSON"})
		return false
	}
	return true
}

func scimID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		scimFail(w, scimErr{404, "", "resource not found"})
		return "", false
	}
	return id.String(), true
}

// ---- discovery ----

func (s *Server) scimConfig(w http.ResponseWriter, _ *http.Request, _ scimToken) {
	scimWrite(w, 200, map[string]any{
		"schemas":               []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"patch":                 map[string]any{"supported": true},
		"bulk":                  map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":                map[string]any{"supported": true, "maxResults": scimMaxPage},
		"changePassword":        map[string]any{"supported": false},
		"sort":                  map[string]any{"supported": false},
		"etag":                  map[string]any{"supported": false},
		"authenticationSchemes": []any{map[string]any{"type": "oauthbearertoken", "name": "Bearer token", "description": "A SCIM token created by an administrator", "primary": true}},
	})
}

func (s *Server) scimResourceTypes(w http.ResponseWriter, _ *http.Request, _ scimToken) {
	scimWrite(w, 200, map[string]any{"schemas": []string{scimSchemaList}, "totalResults": 2, "startIndex": 1, "itemsPerPage": 2, "Resources": []any{
		map[string]any{"id": "User", "name": "User", "endpoint": "/Users", "schema": scimSchemaUser},
		map[string]any{"id": "Group", "name": "Group", "endpoint": "/Groups", "schema": scimSchemaGroup},
	}})
}

func (s *Server) scimSchemas(w http.ResponseWriter, _ *http.Request, _ scimToken) {
	scimWrite(w, 200, map[string]any{"schemas": []string{scimSchemaList}, "totalResults": 2, "startIndex": 1, "itemsPerPage": 2, "Resources": []any{
		map[string]any{"id": scimSchemaUser, "name": "User"}, map[string]any{"id": scimSchemaGroup, "name": "Group"},
	}})
}

// ---- shared helpers ----

var scimFilter = regexp.MustCompile(`(?i)^\s*([A-Za-z]+)\s+eq\s+"([^"]{0,320})"\s*$`)

// scimWhere turns `attr eq "value"` into a SQL condition on one of the allowed columns. The column comes from the map, never from the input.
func scimWhere(filter string, columns map[string]string) (string, any, error) {
	if strings.TrimSpace(filter) == "" {
		return "TRUE", nil, nil
	}
	m := scimFilter.FindStringSubmatch(filter)
	if m == nil {
		return "", nil, scimErr{400, "invalidFilter", "only the form attribute eq \"value\" is supported"}
	}
	column, ok := columns[strings.ToLower(m[1])]
	if !ok {
		return "", nil, scimErr{400, "invalidFilter", "the attribute " + m[1] + " cannot be filtered"}
	}
	return column + " = $1", m[2], nil
}

func scimPage(r *http.Request) (start, count int) {
	start, count = 1, 100
	if v, err := strconv.Atoi(r.URL.Query().Get("startIndex")); err == nil && v > 1 {
		start = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("count")); err == nil && v >= 0 {
		count = min(v, scimMaxPage)
	}
	return start, count
}

// scimBool reads a boolean the way real providers send it: Entra ID sends the strings "True" and "False".
func scimBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		b, err := strconv.ParseBool(strings.ToLower(x))
		return b, err == nil
	}
	return false, false
}

type scimOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

func scimPatchBody(w http.ResponseWriter, r *http.Request) ([]scimOp, bool) {
	var in struct {
		Schemas    []string `json:"schemas"`
		Operations []scimOp `json:"Operations"`
	}
	if !scimBody(w, r, &in) {
		return nil, false
	}
	if !slices.Contains(in.Schemas, scimSchemaPatch) || len(in.Operations) == 0 || len(in.Operations) > 100 {
		scimFail(w, scimErr{400, "invalidSyntax", "a PatchOp with 1 to 100 operations is required"})
		return nil, false
	}
	for _, op := range in.Operations {
		switch strings.ToLower(op.Op) {
		case "add", "replace", "remove":
		default:
			scimFail(w, scimErr{400, "invalidSyntax", "unknown operation " + op.Op})
			return nil, false
		}
	}
	return in.Operations, true
}

// ---- users ----

type scimUserIn struct {
	UserName    string `json:"userName"`
	ExternalID  string `json:"externalId"`
	DisplayName string `json:"displayName"`
	Title       string `json:"title"`
	Active      *bool  `json:"-"`
	RawActive   any    `json:"active"`
	Name        struct {
		Given  string `json:"givenName"`
		Family string `json:"familyName"`
	} `json:"name"`
	Emails []struct {
		Value   string `json:"value"`
		Primary bool   `json:"primary"`
	} `json:"emails"`
}

// email is the sign-in name: userName first (all providers send it), the primary work address when userName is not an address.
func (u *scimUserIn) email() (string, error) {
	candidates := []string{u.UserName}
	for _, e := range u.Emails {
		candidates = append(candidates, e.Value)
	}
	for _, c := range candidates {
		c = strings.ToLower(strings.TrimSpace(c))
		if a, err := stdmail.ParseAddress(c); err == nil && a.Address == c && len(c) <= 254 {
			return c, nil
		}
	}
	return "", scimErr{400, "invalidValue", "userName must be an email address"}
}

func (u *scimUserIn) clean() error {
	if u.RawActive != nil {
		b, ok := scimBool(u.RawActive)
		if !ok {
			return scimErr{400, "invalidValue", "active must be true or false"}
		}
		u.Active = &b
	}
	u.Name.Given, u.Name.Family = strings.TrimSpace(u.Name.Given), strings.TrimSpace(u.Name.Family)
	if u.Name.Given == "" && u.Name.Family == "" && strings.TrimSpace(u.DisplayName) != "" {
		u.Name.Given, u.Name.Family, _ = strings.Cut(strings.TrimSpace(u.DisplayName), " ")
		u.Name.Family = strings.TrimSpace(u.Name.Family)
	}
	if u.Name.Family == "" { // the account needs both names; a provider with a single name field gets a dot rather than a refusal
		u.Name.Family = "."
	}
	if u.Name.Given == "" {
		u.Name.Given = "."
	}
	if utf8.RuneCountInString(u.Name.Given) > 100 || utf8.RuneCountInString(u.Name.Family) > 100 || utf8.RuneCountInString(u.Title) > 200 || len(u.ExternalID) > 255 {
		return scimErr{400, "invalidValue", "a value is too long"}
	}
	return nil
}

const scimUserSelect = `SELECT jsonb_build_object('schemas',jsonb_build_array('` + scimSchemaUser + `'),'id',u.id,'externalId',u.scim_external_id,'userName',u.email,
 'name',jsonb_build_object('givenName',u.first_name,'familyName',u.last_name),'displayName',btrim(u.first_name||' '||u.last_name),
 'title',NULLIF(u.job_title,''),'emails',jsonb_build_array(jsonb_build_object('value',u.email,'primary',true)),'active',u.status='active',
 'meta',jsonb_build_object('resourceType','User','created',u.created_at,'lastModified',u.updated_at,'location','` + scimBase + `/Users/'||u.id))
FROM users u WHERE u.id<>'` + externalRequesterID + `'`

func scimUser(ctx context.Context, db adminQueryRow, id string) (json.RawMessage, error) {
	var out json.RawMessage
	err := db.QueryRow(ctx, scimUserSelect+" AND u.id=$1::uuid", id).Scan(&out)
	return out, err
}

func (s *Server) scimListUsers(w http.ResponseWriter, r *http.Request, _ scimToken) {
	cond, arg, err := scimWhere(r.URL.Query().Get("filter"), map[string]string{"username": "u.email", "externalid": "u.scim_external_id", "id": "u.id::text"})
	if err != nil {
		scimFail(w, err)
		return
	}
	s.scimList(w, r, scimUserSelect, "FROM users u WHERE u.id<>'"+externalRequesterID+"'", "u", cond, arg)
}

// scimList runs the select twice-in-one: a total and one page, both under the same filter.
func (s *Server) scimList(w http.ResponseWriter, r *http.Request, selectSQL, countFrom, alias, cond string, arg any) {
	start, count := scimPage(r)
	args := []any{}
	if arg != nil {
		args = append(args, arg)
	}
	limitAt := len(args) + 1
	pageSQL := selectSQL + " AND " + cond + " ORDER BY " + alias + ".created_at," + alias + ".id OFFSET $" + strconv.Itoa(limitAt) + " LIMIT $" + strconv.Itoa(limitAt+1)
	pool := s.pool
	var total int
	if err := pool.QueryRow(r.Context(), "SELECT count(*) "+countFrom+" AND "+cond, args...).Scan(&total); err != nil {
		scimFail(w, err)
		return
	}
	rows, err := pool.Query(r.Context(), pageSQL, append(args, start-1, count)...)
	if err != nil {
		scimFail(w, err)
		return
	}
	defer rows.Close()
	list := []json.RawMessage{}
	for rows.Next() {
		var one json.RawMessage
		if err := rows.Scan(&one); err != nil {
			scimFail(w, err)
			return
		}
		list = append(list, one)
	}
	if rows.Err() != nil {
		scimFail(w, rows.Err())
		return
	}
	scimWrite(w, 200, map[string]any{"schemas": []string{scimSchemaList}, "totalResults": total, "startIndex": start, "itemsPerPage": len(list), "Resources": list})
}

func (s *Server) scimGetUser(w http.ResponseWriter, r *http.Request, _ scimToken) {
	id, ok := scimID(w, r)
	if !ok {
		return
	}
	u, err := scimUser(r.Context(), s.pool, id)
	if err != nil {
		scimFail(w, err)
		return
	}
	scimWrite(w, 200, u)
}

func (s *Server) scimCreateUser(w http.ResponseWriter, r *http.Request, t scimToken) {
	var in scimUserIn
	if !scimBody(w, r, &in) {
		return
	}
	if err := in.clean(); err != nil {
		scimFail(w, err)
		return
	}
	email, err := in.email()
	if err != nil {
		scimFail(w, err)
		return
	}
	active := in.Active == nil || *in.Active
	var id string
	err = s.scimTx(r, func(ctx context.Context, tx pgx.Tx) error {
		if active {
			if err := s.ensureSeat(ctx, tx); err != nil {
				return scimErr{409, "", "no seats are available"}
			}
		}
		status := "active"
		if !active {
			status = "deactivated"
		}
		if err := tx.QueryRow(ctx, `INSERT INTO users(email,first_name,last_name,user_type,status,job_title,scim_external_id,scim_provisioned)
VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),true) RETURNING id::text`, email, in.Name.Given, in.Name.Family, t.DefaultType, status, in.Title, in.ExternalID).Scan(&id); err != nil {
			return err
		}
		return s.auditIn(ctx, tx, r, "", scimLabel(t), auditEvent{Action: auditSCIMUserNew, TargetType: "user", TargetID: id, Details: map[string]any{"email": email, "user_type": t.DefaultType, "active": active}})
	})
	if err != nil {
		scimFail(w, err)
		return
	}
	u, err := scimUser(r.Context(), s.pool, id)
	if err != nil {
		scimFail(w, err)
		return
	}
	w.Header().Set("Location", scimBase+"/Users/"+id)
	scimWrite(w, 201, u)
}

// userChange is what a PUT or PATCH wants to alter; nil means leave it.
type userChange struct {
	email, given, family, title, external *string
	active                                *bool
}

func (s *Server) scimApplyUser(ctx context.Context, tx pgx.Tx, r *http.Request, t scimToken, id string, c userChange) error {
	var status, userType string
	var password bool
	if err := tx.QueryRow(ctx, "SELECT status::text,user_type::text,password_hash IS NOT NULL FROM users WHERE id=$1 AND id<>$2 FOR UPDATE", id, externalRequesterID).Scan(&status, &userType, &password); err != nil {
		return err
	}
	if userType == string(policy.Admin) {
		return scimErr{403, "mutability", "an administrator cannot be changed through provisioning"}
	}
	if c.email != nil || c.given != nil || c.family != nil || c.title != nil || c.external != nil {
		// Providers resend the whole record on every sync, so only a real change is written and audited.
		tag, err := tx.Exec(ctx, `UPDATE users SET email=COALESCE($2,email),first_name=COALESCE($3,first_name),last_name=COALESCE($4,last_name),job_title=COALESCE($5,job_title),
scim_external_id=CASE WHEN $6::boolean THEN NULLIF($7,'') ELSE scim_external_id END
WHERE id=$1 AND (email IS DISTINCT FROM COALESCE($2,email) OR first_name IS DISTINCT FROM COALESCE($3,first_name) OR last_name IS DISTINCT FROM COALESCE($4,last_name)
 OR job_title IS DISTINCT FROM COALESCE($5,job_title) OR ($6::boolean AND scim_external_id IS DISTINCT FROM NULLIF($7,'')))`,
			id, c.email, c.given, c.family, c.title, c.external != nil, deref(c.external))
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			if err := s.auditIn(ctx, tx, r, "", scimLabel(t), auditEvent{Action: auditSCIMUserUpd, TargetType: "user", TargetID: id}); err != nil {
				return err
			}
		}
	}
	if c.active == nil {
		return nil
	}
	switch {
	case !*c.active && status != "deactivated":
		if err := scimDeactivate(ctx, tx, id); err != nil {
			return err
		}
		return s.auditIn(ctx, tx, r, "", scimLabel(t), auditEvent{Action: auditSCIMUserOff, TargetType: "user", TargetID: id})
	case *c.active && status != "active":
		if err := s.ensureSeat2(ctx, tx, status == "invited"); err != nil {
			return err
		}
		// A person with no password signs in through single sign-on; scim_provisioned is what lets such an account be active.
		if _, err := tx.Exec(ctx, "UPDATE users SET status='active',scim_provisioned=scim_provisioned OR password_hash IS NULL WHERE id=$1", id); err != nil {
			return err
		}
		return s.auditIn(ctx, tx, r, "", scimLabel(t), auditEvent{Action: auditSCIMUserOn, TargetType: "user", TargetID: id})
	}
	return nil
}

// ensureSeat2: a pending invitation already holds a seat, so activating it needs none.
func (s *Server) ensureSeat2(ctx context.Context, tx pgx.Tx, holdsSeat bool) error {
	if holdsSeat {
		return nil
	}
	if err := s.ensureSeat(ctx, tx); err != nil {
		return scimErr{409, "", "no seats are available"}
	}
	return nil
}

// scimDeactivate is the single deprovision path: the seat frees (status), and nothing the person held keeps working (sessions, invite links).
func scimDeactivate(ctx context.Context, tx pgx.Tx, id string) error {
	if _, err := tx.Exec(ctx, "UPDATE users SET status='deactivated',invitation_generation=invitation_generation+1 WHERE id=$1", id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM user_tokens WHERE user_id=$1 AND used_at IS NULL", id); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "DELETE FROM sessions WHERE user_id=$1", id)
	return err
}

func (s *Server) scimPutUser(w http.ResponseWriter, r *http.Request, t scimToken) {
	id, ok := scimID(w, r)
	if !ok {
		return
	}
	var in scimUserIn
	if !scimBody(w, r, &in) {
		return
	}
	if err := in.clean(); err != nil {
		scimFail(w, err)
		return
	}
	email, err := in.email()
	if err != nil {
		scimFail(w, err)
		return
	}
	active := in.Active == nil || *in.Active // a PUT replaces the whole resource, and a missing active means the default: active
	c := userChange{email: &email, given: &in.Name.Given, family: &in.Name.Family, title: &in.Title, external: &in.ExternalID, active: &active}
	s.scimUserWrite(w, r, t, id, c)
}

func (s *Server) scimUserWrite(w http.ResponseWriter, r *http.Request, t scimToken, id string, c userChange) {
	err := s.scimTx(r, func(ctx context.Context, tx pgx.Tx) error { return s.scimApplyUser(ctx, tx, r, t, id, c) })
	if err != nil {
		scimFail(w, err)
		return
	}
	u, err := scimUser(r.Context(), s.pool, id)
	if err != nil {
		scimFail(w, err)
		return
	}
	scimWrite(w, 200, u)
}

func (s *Server) scimPatchUser(w http.ResponseWriter, r *http.Request, t scimToken) {
	id, ok := scimID(w, r)
	if !ok {
		return
	}
	ops, ok := scimPatchBody(w, r)
	if !ok {
		return
	}
	var c userChange
	set := func(attr string, raw json.RawMessage, remove bool) error {
		var str string
		if !remove {
			var v any
			if json.Unmarshal(raw, &v) != nil {
				return scimErr{400, "invalidValue", "a value is not valid"}
			}
			if attr == "active" {
				b, ok := scimBool(v)
				if !ok {
					return scimErr{400, "invalidValue", "active must be true or false"}
				}
				c.active = &b
				return nil
			}
			var isStr bool
			if str, isStr = v.(string); !isStr {
				return nil // an attribute this server does not store
			}
		}
		str = strings.TrimSpace(str)
		switch attr {
		case "username":
			e := strings.ToLower(str)
			if a, err := stdmail.ParseAddress(e); err != nil || a.Address != e || len(e) > 254 {
				return scimErr{400, "invalidValue", "userName must be an email address"}
			}
			c.email = &e
		case "externalid":
			c.external = &str
		case "name.givenname":
			c.given = &str
		case "name.familyname":
			c.family = &str
		case "title":
			c.title = &str
		}
		return nil
	}
	for _, op := range ops {
		remove := strings.EqualFold(op.Op, "remove")
		path := strings.ToLower(strings.TrimSpace(op.Path))
		var err error
		if path == "" { // the value is an object of attributes
			var obj map[string]json.RawMessage
			if json.Unmarshal(op.Value, &obj) != nil {
				scimFail(w, scimErr{400, "invalidSyntax", "an operation without a path needs an object value"})
				return
			}
			for k, v := range obj {
				if strings.EqualFold(k, "name") {
					var n map[string]json.RawMessage
					if json.Unmarshal(v, &n) == nil {
						for nk, nv := range n {
							if err = set("name."+strings.ToLower(nk), nv, false); err != nil {
								break
							}
						}
					}
					continue
				}
				if err = set(strings.ToLower(k), v, false); err != nil {
					break
				}
			}
		} else {
			err = set(path, op.Value, remove)
		}
		if err != nil {
			scimFail(w, err)
			return
		}
	}
	if c.given != nil && *c.given == "" {
		c.given = nil // the account needs a name; removing it keeps the old one
	}
	if c.family != nil && *c.family == "" {
		c.family = nil
	}
	if c.email != nil && *c.email == "" {
		c.email = nil
	}
	s.scimUserWrite(w, r, t, id, c)
}

func (s *Server) scimDeleteUser(w http.ResponseWriter, r *http.Request, t scimToken) {
	id, ok := scimID(w, r)
	if !ok {
		return
	}
	off := false
	// DELETE deprovisions: the row and its history stay (comments, audit), only access and the seat go. Calling it again is harmless.
	err := s.scimTx(r, func(ctx context.Context, tx pgx.Tx) error {
		return s.scimApplyUser(ctx, tx, r, t, id, userChange{active: &off})
	})
	if err != nil {
		scimFail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- groups ----

const scimGroupSelect = `SELECT jsonb_build_object('schemas',jsonb_build_array('` + scimSchemaGroup + `'),'id',g.id,'externalId',g.scim_external_id,'displayName',g.name,
 'members',COALESCE((SELECT jsonb_agg(jsonb_build_object('value',m.user_id,'display',u.email) ORDER BY u.email) FROM group_members m JOIN users u ON u.id=m.user_id WHERE m.group_id=g.id),'[]'::jsonb),
 'meta',jsonb_build_object('resourceType','Group','created',g.created_at,'lastModified',g.updated_at,'location','` + scimBase + `/Groups/'||g.id))
FROM groups g WHERE TRUE`

func scimGroup(ctx context.Context, db adminQueryRow, id string) (json.RawMessage, error) {
	var out json.RawMessage
	err := db.QueryRow(ctx, scimGroupSelect+" AND g.id=$1::uuid", id).Scan(&out)
	return out, err
}

func (s *Server) scimListGroups(w http.ResponseWriter, r *http.Request, _ scimToken) {
	cond, arg, err := scimWhere(r.URL.Query().Get("filter"), map[string]string{"displayname": "lower(g.name)", "externalid": "g.scim_external_id", "id": "g.id::text"})
	if err != nil {
		scimFail(w, err)
		return
	}
	if v, ok := arg.(string); ok && strings.Contains(cond, "lower(") {
		arg = strings.ToLower(v)
	}
	s.scimList(w, r, scimGroupSelect, "FROM groups g WHERE TRUE", "g", cond, arg)
}

func (s *Server) scimGetGroup(w http.ResponseWriter, r *http.Request, _ scimToken) {
	id, ok := scimID(w, r)
	if !ok {
		return
	}
	g, err := scimGroup(r.Context(), s.pool, id)
	if err != nil {
		scimFail(w, err)
		return
	}
	scimWrite(w, 200, g)
}

type scimGroupIn struct {
	DisplayName string `json:"displayName"`
	ExternalID  string `json:"externalId"`
	Members     []struct {
		Value string `json:"value"`
	} `json:"members"`
}

func (g *scimGroupIn) members() ([]string, error) {
	if len(g.Members) > 5000 {
		return nil, scimErr{400, "invalidValue", "too many members"}
	}
	out := make([]string, 0, len(g.Members))
	for _, m := range g.Members {
		id, err := uuid.Parse(m.Value)
		if err != nil {
			return nil, scimErr{400, "invalidValue", "a member must be a user id"}
		}
		out = append(out, id.String())
	}
	return out, nil
}

func scimName(n string) (string, error) {
	n = strings.TrimSpace(n)
	if n == "" || utf8.RuneCountInString(n) > 200 {
		return "", scimErr{400, "invalidValue", "displayName is required and at most 200 characters"}
	}
	return n, nil
}

// setMembers makes the group's members exactly this list. A member must be a real user, never the form placeholder.
func setMembers(ctx context.Context, tx pgx.Tx, group string, members []string) error {
	var known int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE id=ANY($1::uuid[]) AND id<>$2", members, externalRequesterID).Scan(&known); err != nil {
		return err
	}
	if known != len(uniq(members)) {
		return scimErr{400, "invalidValue", "a member is not a known user"}
	}
	if _, err := tx.Exec(ctx, "DELETE FROM group_members WHERE group_id=$1", group); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "INSERT INTO group_members(group_id,user_id) SELECT $1::uuid,m FROM unnest($2::uuid[]) m ON CONFLICT DO NOTHING", group, members)
	return err
}

func uniq(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}

func (s *Server) scimCreateGroup(w http.ResponseWriter, r *http.Request, t scimToken) {
	var in scimGroupIn
	if !scimBody(w, r, &in) {
		return
	}
	name, err := scimName(in.DisplayName)
	if err == nil && len(in.ExternalID) > 255 {
		err = scimErr{400, "invalidValue", "externalId is too long"}
	}
	var members []string
	if err == nil {
		members, err = in.members()
	}
	if err != nil {
		scimFail(w, err)
		return
	}
	var id string
	err = s.scimTx(r, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, "INSERT INTO groups(name,scim_external_id) VALUES($1,NULLIF($2,'')) RETURNING id::text", name, in.ExternalID).Scan(&id); err != nil {
			return err
		}
		if err := setMembers(ctx, tx, id, members); err != nil {
			return err
		}
		return s.auditIn(ctx, tx, r, "", scimLabel(t), auditEvent{Action: auditSCIMGroupNW, TargetType: "group", TargetID: id, Details: map[string]any{"name": name, "members": len(uniq(members))}})
	})
	if err != nil {
		scimFail(w, err)
		return
	}
	g, err := scimGroup(r.Context(), s.pool, id)
	if err != nil {
		scimFail(w, err)
		return
	}
	w.Header().Set("Location", scimBase+"/Groups/"+id)
	scimWrite(w, 201, g)
}

func (s *Server) scimPutGroup(w http.ResponseWriter, r *http.Request, t scimToken) {
	id, ok := scimID(w, r)
	if !ok {
		return
	}
	var in scimGroupIn
	if !scimBody(w, r, &in) {
		return
	}
	name, err := scimName(in.DisplayName)
	var members []string
	if err == nil {
		members, err = in.members()
	}
	if err != nil {
		scimFail(w, err)
		return
	}
	s.scimGroupWrite(w, r, t, id, &name, &in.ExternalID, func(ctx context.Context, tx pgx.Tx) error { return setMembers(ctx, tx, id, members) })
}

func (s *Server) scimGroupWrite(w http.ResponseWriter, r *http.Request, t scimToken, id string, name, external *string, members func(context.Context, pgx.Tx) error) {
	err := s.scimTx(r, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE groups SET name=COALESCE($2,name),scim_external_id=CASE WHEN $3::boolean THEN NULLIF($4,'') ELSE scim_external_id END WHERE id=$1`, id, name, external != nil, deref(external))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		if members != nil {
			if err := members(ctx, tx); err != nil {
				return err
			}
		}
		return s.auditIn(ctx, tx, r, "", scimLabel(t), auditEvent{Action: auditSCIMGroupUP, TargetType: "group", TargetID: id})
	})
	if err != nil {
		scimFail(w, err)
		return
	}
	g, err := scimGroup(r.Context(), s.pool, id)
	if err != nil {
		scimFail(w, err)
		return
	}
	scimWrite(w, 200, g)
}

var scimMemberPath = regexp.MustCompile(`(?i)^members\[value\s+eq\s+"([^"]+)"\]$`)

func (s *Server) scimPatchGroup(w http.ResponseWriter, r *http.Request, t scimToken) {
	id, ok := scimID(w, r)
	if !ok {
		return
	}
	ops, ok := scimPatchBody(w, r)
	if !ok {
		return
	}
	var name, external *string
	// Each op becomes one edit of the member set, applied in order inside the transaction.
	var edits []func(map[string]bool)
	var replaced bool
	parseMembers := func(raw json.RawMessage) ([]string, error) {
		var in scimGroupIn
		if err := json.Unmarshal([]byte(`{"members":`+string(raw)+`}`), &in); err != nil {
			return nil, scimErr{400, "invalidValue", "members must be a list"}
		}
		return in.members()
	}
	for _, op := range ops {
		kind := strings.ToLower(op.Op)
		path := strings.TrimSpace(op.Path)
		switch {
		case strings.EqualFold(path, "members") && kind == "remove" && len(op.Value) == 0:
			edits = append(edits, func(m map[string]bool) { clear(m) })
			replaced = true
		case strings.EqualFold(path, "members"):
			list, err := parseMembers(op.Value)
			if err != nil {
				scimFail(w, err)
				return
			}
			edits = append(edits, func(m map[string]bool) {
				if kind == "replace" {
					clear(m)
				}
				for _, u := range list {
					if kind == "remove" {
						delete(m, u)
					} else {
						m[u] = true
					}
				}
			})
			replaced = true
		case scimMemberPath.MatchString(path) && kind == "remove":
			user, err := uuid.Parse(scimMemberPath.FindStringSubmatch(path)[1])
			if err != nil {
				scimFail(w, scimErr{400, "invalidValue", "a member must be a user id"})
				return
			}
			edits = append(edits, func(m map[string]bool) { delete(m, user.String()) })
			replaced = true
		case strings.EqualFold(path, "displayName") || strings.EqualFold(path, "externalId") || path == "":
			var vals map[string]any
			if path == "" {
				if json.Unmarshal(op.Value, &vals) != nil {
					scimFail(w, scimErr{400, "invalidSyntax", "an operation without a path needs an object value"})
					return
				}
			} else {
				var v any
				_ = json.Unmarshal(op.Value, &v)
				vals = map[string]any{path: v}
			}
			for k, v := range vals {
				str, _ := v.(string)
				switch strings.ToLower(k) {
				case "displayname":
					n, err := scimName(str)
					if err != nil {
						scimFail(w, err)
						return
					}
					name = &n
				case "externalid":
					if len(str) > 255 {
						scimFail(w, scimErr{400, "invalidValue", "externalId is too long"})
						return
					}
					external = &str
				case "members":
					list, err := parseMembers(mustJSON(v))
					if err != nil {
						scimFail(w, err)
						return
					}
					edits = append(edits, func(m map[string]bool) {
						clear(m)
						for _, u := range list {
							m[u] = true
						}
					})
					replaced = true
				}
			}
		default:
			scimFail(w, scimErr{400, "noTarget", "the path " + path + " is not supported"})
			return
		}
	}
	var members func(context.Context, pgx.Tx) error
	if replaced {
		members = func(ctx context.Context, tx pgx.Tx) error {
			current := map[string]bool{}
			rows, err := tx.Query(ctx, "SELECT user_id::text FROM group_members WHERE group_id=$1", id)
			if err != nil {
				return err
			}
			for rows.Next() {
				var u string
				if err := rows.Scan(&u); err != nil {
					rows.Close()
					return err
				}
				current[u] = true
			}
			rows.Close()
			for _, edit := range edits {
				edit(current)
			}
			list := make([]string, 0, len(current))
			for u := range current {
				list = append(list, u)
			}
			return setMembers(ctx, tx, id, list)
		}
	}
	s.scimGroupWrite(w, r, t, id, name, external, members)
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func (s *Server) scimDeleteGroup(w http.ResponseWriter, r *http.Request, t scimToken) {
	id, ok := scimID(w, r)
	if !ok {
		return
	}
	err := s.scimTx(r, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "DELETE FROM shares WHERE principal_type='group' AND principal_id=$1", id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, "DELETE FROM groups WHERE id=$1", id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return s.auditIn(ctx, tx, r, "", scimLabel(t), auditEvent{Action: auditSCIMGroupRM, TargetType: "group", TargetID: id})
	})
	if err != nil {
		scimFail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- tokens (administrators, by session) ----

type scimTokenView struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Prefix      string          `json:"prefix"`
	DefaultType policy.UserType `json:"default_user_type"`
	CreatedAt   time.Time       `json:"created_at"`
	LastUsedAt  *time.Time      `json:"last_used_at"`
	RevokedAt   *time.Time      `json:"revoked_at"`
	Token       string          `json:"token,omitempty"`
}

func (s *Server) scimListTokens(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), "SELECT id::text,name,prefix,default_user_type::text,created_at,last_used_at,revoked_at FROM scim_tokens ORDER BY created_at DESC")
	if err != nil {
		adminWriteError(w, err)
		return
	}
	defer rows.Close()
	list := []scimTokenView{}
	for rows.Next() {
		var v scimTokenView
		if err := rows.Scan(&v.ID, &v.Name, &v.Prefix, &v.DefaultType, &v.CreatedAt, &v.LastUsedAt, &v.RevokedAt); err != nil {
			adminWriteError(w, err)
			return
		}
		list = append(list, v)
	}
	writeJSON(w, 200, map[string]any{"items": list, "base_url": s.cfg.PublicURL + scimBase})
}

func (s *Server) scimNewToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string          `json:"name"`
		DefaultType policy.UserType `json:"default_user_type"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.DefaultType == "" {
		in.DefaultType = policy.Regular
	}
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 80 || in.DefaultType == policy.Admin || !slices.Contains(policy.UserTypes, in.DefaultType) {
		writeError(w, 422, "a name (up to 80 characters) and a non-administrator default user type are required")
		return
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		adminWriteError(w, err)
		return
	}
	token := "pms_scim_" + base64.RawURLEncoding.EncodeToString(random)
	sum := sha256.Sum256([]byte(token))
	v := scimTokenView{Name: in.Name, Prefix: token[:13], DefaultType: in.DefaultType, Token: token}
	err := s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, "INSERT INTO scim_tokens(name,token_hash,prefix,default_user_type,created_by) VALUES($1,$2,$3,$4,$5::uuid) RETURNING id::text,created_at",
			v.Name, sum[:], v.Prefix, v.DefaultType, actorOf(r).ID).Scan(&v.ID, &v.CreatedAt); err != nil {
			return err
		}
		return s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditSCIMToken, TargetType: "scim_token", TargetID: v.ID, Details: map[string]any{"name": v.Name, "default_user_type": v.DefaultType}})
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) scimRevokeToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("tokenId"))
	if err != nil {
		writeError(w, 404, "not found")
		return
	}
	err = s.adminTx(r, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "UPDATE scim_tokens SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL", id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditSCIMRevoked, TargetType: "scim_token", TargetID: id.String()})
	})
	if err != nil {
		adminWriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
