// Package clients is the native Customer Master domain service (B2-03, docs/architecture/ADR-CLIENT-SCOPE-MVP.md).
//
// Trust boundary: this package trusts its callers. It checks inputs and lifecycle rules, not who may call it. The HTTP
// boundary (B2-04/B2-06) decides authorization and must not expose these functions without it. Membership here is
// eligibility only: nothing in this package creates Spaces, shares, roles, or resource grants.
//
// Lock order for every mutation: the client row first, then the user row or the membership row. Grants take a shared lock
// on the client, so they serialize with archive (exclusive lock) and cannot create a membership after an archive commits.
package clients

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Audit actions. The HTTP layer writes them with the same names that audit_events stores.
const (
	ActionCreated       = "client.created"
	ActionUpdated       = "client.updated"
	ActionArchived      = "client.archived"
	ActionMemberGranted = "client.member_granted"
	ActionMemberRevoked = "client.member_revoked"
)

// Event is one real state change. It is recorded by a Recorder in the same transaction as the change, so a change
// cannot commit without its event. Events are not emitted for operations that change nothing.
type Event struct {
	Action     string
	CustomerID string
	UserID     string // membership events only
	Details    map[string]any
}

// Recorder writes an event inside the caller's transaction. An error aborts the mutation. It is required.
type Recorder func(ctx context.Context, tx pgx.Tx, ev Event) error

var (
	ErrNoRecorder     = errors.New("clients: an audit recorder is required")
	ErrNotFound       = errors.New("customer not found")
	ErrUserNotFound   = errors.New("user not found")
	ErrIneligibleUser = errors.New("user is not eligible for client membership")
	ErrDuplicateCode  = errors.New("customer code already exists")
	ErrArchived       = errors.New("customer is archived and read-only")
	ErrNotMember      = errors.New("user has never been a member of this customer")
)

// InvalidError is a bad input. Field names the input; Message is safe to show.
type InvalidError struct{ Field, Message string }

func (e InvalidError) Error() string { return e.Field + ": " + e.Message }

// Customer is one client. Code and ID never change after creation.
type Customer struct {
	ID         string
	Code       string
	Name       string
	ShortName  *string
	Status     string // active or archived
	CreatedAt  time.Time
	CreatedBy  string
	UpdatedAt  time.Time
	ArchivedAt *time.Time
	ArchivedBy *string
}

// Membership is one grant of eligibility. Revoked rows are kept as history.
type Membership struct {
	ID        string
	ClientID  string
	UserID    string
	UserName  string // only filled by ListMembers
	Status    string // active or revoked
	GrantedAt time.Time
	GrantedBy string
	RevokedAt *time.Time
	RevokedBy *string
}

// CreateInput is what a new customer needs. The code is normalized before it is stored.
type CreateInput struct {
	Code      string
	Name      string
	ShortName *string
}

// UpdateInput changes metadata only. There is no code or ID field: those are immutable by contract.
// A nil field is unchanged. A ShortName that is blank clears it.
type UpdateInput struct {
	Name      *string
	ShortName *string
}

// Filter selects customers by lifecycle state.
type Filter string

const (
	All      Filter = "all"
	Active   Filter = "active"
	Archived Filter = "archived"
)

const (
	defaultLimit = 50
	maxLimit     = 200
	maxMembers   = 500 // shortcut: no pagination for members yet; add a cursor when a client has more than this
)

// Service runs customer operations on the pool. Each operation uses its own transaction.
type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

const customerColumns = `id::text, code, name, short_name, status, created_at, created_by::text, updated_at, archived_at, archived_by::text`

func scanCustomer(row pgx.Row) (Customer, error) {
	var c Customer
	err := row.Scan(&c.ID, &c.Code, &c.Name, &c.ShortName, &c.Status, &c.CreatedAt, &c.CreatedBy, &c.UpdatedAt, &c.ArchivedAt, &c.ArchivedBy)
	return c, err
}

// NormalizeCode is the canonical form the database also enforces: trimmed, uppercase, never blank.
func NormalizeCode(code string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "" {
		return "", InvalidError{"code", "must not be blank"}
	}
	return c, nil
}

func normalizeName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" {
		return "", InvalidError{"name", "must not be blank"}
	}
	return n, nil
}

func normalizeShort(s *string) (*string, error) {
	if s == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil, nil // blank clears the optional field
	}
	return &v, nil
}

func pgErrorOf(err error) *pgconn.PgError {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg
	}
	return nil
}

// mapCustomerError turns verified constraint names into domain errors. Anything else stays generic: it is not shown to users.
func mapCustomerError(err error) error {
	pg := pgErrorOf(err)
	if pg == nil {
		return err
	}
	switch {
	case pg.Code == "23505" && pg.ConstraintName == "clients_code_key":
		return ErrDuplicateCode
	case pg.Code == "23514" && pg.ConstraintName == "clients_code_check":
		return InvalidError{"code", "must be trimmed and uppercase"}
	case pg.Code == "23514" && pg.ConstraintName == "clients_name_check":
		return InvalidError{"name", "must not be blank"}
	case pg.Code == "23514" && pg.ConstraintName == "clients_short_name_check":
		return InvalidError{"short_name", "must not be blank"}
	case pg.Code == "23503":
		return ErrUserNotFound // the only foreign keys on clients point at users (created_by, archived_by)
	}
	return fmt.Errorf("clients: database error (%s)", pg.Code)
}

// Create stores a new active customer. Duplicate codes are rejected by the unique constraint, which is the final authority.
func (s *Service) Create(ctx context.Context, actor string, in CreateInput, rec Recorder) (Customer, error) {
	if rec == nil {
		return Customer{}, ErrNoRecorder
	}
	code, err := NormalizeCode(in.Code)
	if err != nil {
		return Customer{}, err
	}
	name, err := normalizeName(in.Name)
	if err != nil {
		return Customer{}, err
	}
	short, err := normalizeShort(in.ShortName)
	if err != nil {
		return Customer{}, err
	}
	if _, err := uuid.Parse(actor); err != nil {
		return Customer{}, ErrUserNotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Customer{}, err
	}
	defer tx.Rollback(ctx)
	c, err := scanCustomer(tx.QueryRow(ctx,
		`INSERT INTO clients(code,name,short_name,created_by) VALUES ($1,$2,$3,$4::uuid) RETURNING `+customerColumns,
		code, name, short, actor))
	if err != nil {
		return Customer{}, mapCustomerError(err)
	}
	if err := rec(ctx, tx, Event{Action: ActionCreated, CustomerID: c.ID, Details: map[string]any{"code": c.Code}}); err != nil {
		return Customer{}, fmt.Errorf("clients: record audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Customer{}, mapCustomerError(err)
	}
	return c, nil
}

// Get returns one customer by UUID, whatever its state. Callers that show history need archived customers too.
func (s *Service) Get(ctx context.Context, id string) (Customer, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Customer{}, ErrNotFound
	}
	c, err := scanCustomer(s.pool.QueryRow(ctx, `SELECT `+customerColumns+` FROM clients WHERE id=$1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrNotFound
	}
	return c, err
}

// List returns customers in a stable order (code, then id). A zero limit means the default; above the cap is rejected.
func (s *Service) List(ctx context.Context, filter Filter, limit int) ([]Customer, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maxLimit {
		return nil, InvalidError{"limit", fmt.Sprintf("must be between 1 and %d", maxLimit)}
	}
	var where string
	switch filter {
	case All:
	case Active:
		where = `WHERE status='active'`
	case Archived:
		where = `WHERE status='archived'`
	default:
		return nil, InvalidError{"filter", "must be all, active, or archived"}
	}
	rows, err := s.pool.Query(ctx, `SELECT `+customerColumns+` FROM clients `+where+` ORDER BY code, id LIMIT $1`, limit)
	if err != nil {
		return nil, mapCustomerError(err)
	}
	defer rows.Close()
	out := []Customer{}
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, mapCustomerError(err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Update changes name and short name. An archived customer is read-only, so it is rejected, not silently restored.
func (s *Service) Update(ctx context.Context, actor, id string, in UpdateInput, rec Recorder) (Customer, error) {
	if rec == nil {
		return Customer{}, ErrNoRecorder
	}
	if _, err := uuid.Parse(id); err != nil {
		return Customer{}, ErrNotFound
	}
	var name *string
	if in.Name != nil {
		n, err := normalizeName(*in.Name)
		if err != nil {
			return Customer{}, err
		}
		name = &n
	}
	short, err := normalizeShort(in.ShortName)
	if err != nil {
		return Customer{}, err
	}
	clearShort := in.ShortName != nil && short == nil

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Customer{}, err
	}
	defer tx.Rollback(ctx)
	cur, err := scanCustomer(tx.QueryRow(ctx, `SELECT `+customerColumns+` FROM clients WHERE id=$1::uuid FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrNotFound
	} else if err != nil {
		return Customer{}, mapCustomerError(err)
	}
	if cur.Status == "archived" {
		return Customer{}, ErrArchived
	}
	var changed []string
	if name != nil && *name != cur.Name {
		changed = append(changed, "name")
	}
	if in.ShortName != nil && !sameOptional(short, cur.ShortName) {
		changed = append(changed, "short_name")
	}
	if len(changed) == 0 {
		return cur, nil // nothing differs: no write, no event
	}
	c, err := scanCustomer(tx.QueryRow(ctx,
		`UPDATE clients SET name=COALESCE($2,name), short_name=CASE WHEN $4::boolean THEN NULL WHEN $3::text IS NOT NULL THEN $3 ELSE short_name END, updated_at=now()
		 WHERE id=$1::uuid RETURNING `+customerColumns,
		id, name, short, clearShort))
	if err != nil {
		return Customer{}, mapCustomerError(err)
	}
	if err := rec(ctx, tx, Event{Action: ActionUpdated, CustomerID: id, Details: map[string]any{"fields": changed}}); err != nil {
		return Customer{}, fmt.Errorf("clients: record audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Customer{}, mapCustomerError(err)
	}
	return c, nil
}

func sameOptional(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// Archive makes the customer read-only and keeps every membership row. Archiving twice keeps the first archive metadata.
func (s *Service) Archive(ctx context.Context, actor, id string, rec Recorder) (Customer, error) {
	if rec == nil {
		return Customer{}, ErrNoRecorder
	}
	if _, err := uuid.Parse(id); err != nil {
		return Customer{}, ErrNotFound
	}
	if _, err := uuid.Parse(actor); err != nil {
		return Customer{}, ErrUserNotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Customer{}, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM clients WHERE id=$1::uuid FOR UPDATE`, id).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrNotFound
	} else if err != nil {
		return Customer{}, mapCustomerError(err)
	}
	if status == "archived" {
		cur, err := scanCustomer(tx.QueryRow(ctx, `SELECT `+customerColumns+` FROM clients WHERE id=$1::uuid`, id))
		if err != nil {
			return Customer{}, mapCustomerError(err)
		}
		return cur, nil // unchanged: the original archive metadata stays, no event
	}
	c, err := scanCustomer(tx.QueryRow(ctx,
		`UPDATE clients SET status='archived', archived_at=now(), archived_by=$2::uuid, updated_at=now()
		 WHERE id=$1::uuid RETURNING `+customerColumns, id, actor))
	if err != nil {
		return Customer{}, mapCustomerError(err)
	}
	if err := rec(ctx, tx, Event{Action: ActionArchived, CustomerID: id}); err != nil {
		return Customer{}, fmt.Errorf("clients: record audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Customer{}, mapCustomerError(err)
	}
	return c, nil
}
