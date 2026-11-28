package clients

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Option is the picker metadata for one customer. It is all a lookup returns: no membership, audit or resource data.
type Option struct {
	ID        string
	Code      string
	Name      string
	ShortName *string
}

const maxSearchRunes = 100

// eligibleOptionsSQL is membership-based eligibility, evaluated in the database before search and LIMIT.
// A customer qualifies only when it is active, the actor has an ACTIVE membership row (revoked and history rows do not
// count), and the actor is an active internal user. The EXISTS form returns each customer once, even after revoke and regrant.
// Search is optional ($2 NULL means no search) and matches name, code or short name.
const eligibleOptionsSQL = `
SELECT c.id::text, c.code, c.name, c.short_name
FROM clients c
WHERE c.status = 'active'
  AND EXISTS (
    SELECT 1 FROM client_memberships m JOIN users u ON u.id = m.user_id
    WHERE m.client_id = c.id AND m.user_id = $1::uuid AND m.status = 'active'
      AND u.status = 'active' AND u.user_type IN ('admin', 'regular'))
  AND ($2::text IS NULL OR c.name ILIKE $2 ESCAPE '\' OR c.code ILIKE $2 ESCAPE '\' OR c.short_name ILIKE $2 ESCAPE '\')
ORDER BY c.code, c.id
LIMIT $3`

// activeOptionsSQL is the administrative configuration lookup: every active customer. It grants no membership.
const activeOptionsSQL = `
SELECT c.id::text, c.code, c.name, c.short_name
FROM clients c
WHERE c.status = 'active'
  AND ($1::text IS NULL OR c.name ILIKE $1 ESCAPE '\' OR c.code ILIKE $1 ESCAPE '\' OR c.short_name ILIKE $1 ESCAPE '\')
ORDER BY c.code, c.id
LIMIT $2`

// ListEligible returns the active customers the actor may select. Eligibility is not content access: it grants nothing.
// An actor that is not a UUID has no eligibility, so the result is empty.
func (s *Service) ListEligible(ctx context.Context, actorID, search string, limit int) ([]Option, error) {
	if _, err := uuid.Parse(actorID); err != nil {
		return []Option{}, nil
	}
	pattern, limit, err := searchArgs(search, limit)
	if err != nil {
		return nil, err
	}
	return s.options(ctx, eligibleOptionsSQL, actorID, pattern, limit)
}

// ListActiveForAdmin is the administrative lookup. Callers must already be Workspace Admin; it grants no membership.
func (s *Service) ListActiveForAdmin(ctx context.Context, search string, limit int) ([]Option, error) {
	pattern, limit, err := searchArgs(search, limit)
	if err != nil {
		return nil, err
	}
	return s.options(ctx, activeOptionsSQL, pattern, limit)
}

// searchArgs validates the limit and search text, and returns the ILIKE pattern (nil when there is no search).
func searchArgs(search string, limit int) (*string, int, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maxLimit {
		return nil, 0, InvalidError{"limit", "must be between 1 and 200"}
	}
	if utf8.RuneCountInString(search) > maxSearchRunes {
		return nil, 0, InvalidError{"q", "must be at most 100 characters"}
	}
	q := strings.TrimSpace(search)
	if q == "" {
		return nil, limit, nil
	}
	p := "%" + likeEscaper.Replace(q) + "%"
	return &p, limit, nil
}

func (s *Service) options(ctx context.Context, sql string, args ...any) ([]Option, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, mapCustomerError(err)
	}
	defer rows.Close()
	out := []Option{}
	for rows.Next() {
		var o Option
		if err := rows.Scan(&o.ID, &o.Code, &o.Name, &o.ShortName); err != nil {
			return nil, mapCustomerError(err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// likeEscaper keeps user text literal inside ILIKE: %, _ and the escape character itself match only themselves.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
