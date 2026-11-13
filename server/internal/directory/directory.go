// Package directory lists users and groups so people can be picked in sharing dialogs.
// Full user and group management (invite, deactivate, edit) arrives with S1.6.1.
package directory

import (
	"context"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	UserType  string  `json:"user_type"`
	Status    string  `json:"status"`
	JobTitle  *string `json:"job_title"`
	Dept      *string `json:"department"`
}

type Group struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	MemberIDs []string `json:"member_ids"`
}

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func canBrowse(actor policy.Actor) bool { return policy.CanBrowseDirectory(actor.Type) }

// Users returns up to limit users matching q (name or email), plus the next offset or -1 when there are no more.
// Admins see every status, everybody else only active users. shortcut: offset paging; fine for a few thousand users.
func (s *Service) Users(ctx context.Context, actor policy.Actor, q, status, userType string, limit, offset int) ([]User, int, error) {
	if !canBrowse(actor) {
		return []User{}, -1, nil
	}
	if len(q) > 200 {
		q = q[:200]
	}
	pattern := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(q)) + "%"
	rows, err := s.pool.Query(ctx, `SELECT id::text, email::text, first_name, last_name, user_type::text, status::text, NULLIF(job_title,''), NULLIF(department,'')
		FROM users
		WHERE ($1 OR status = 'active') AND id <> '00000000-0000-4000-8000-0000000000e1' -- the placeholder owner of public-form items
		  AND (first_name || ' ' || last_name ILIKE $2 OR email::text ILIKE $2)
		  AND ($5 = '' OR status::text = $5) AND ($6 = '' OR user_type::text = $6)
		ORDER BY lower(last_name), lower(first_name), id
		LIMIT $3 OFFSET $4`, actor.Type == policy.Admin, pattern, limit+1, offset, status, userType)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.UserType, &u.Status, &u.JobTitle, &u.Dept); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	next := -1
	if len(out) > limit {
		out, next = out[:limit], offset+limit
	}
	return out, next, nil
}

func (s *Service) Groups(ctx context.Context, actor policy.Actor) ([]Group, error) {
	if !canBrowse(actor) {
		return []Group{}, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT g.id::text, g.name, COALESCE(array_agg(m.user_id::text ORDER BY m.user_id) FILTER (WHERE m.user_id IS NOT NULL), '{}')
		FROM groups g LEFT JOIN group_members m ON m.group_id = g.id GROUP BY g.id ORDER BY lower(g.name), g.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.MemberIDs); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
