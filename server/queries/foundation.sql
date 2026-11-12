-- name: AccountName :one
SELECT name FROM account WHERE id = 1;

-- name: CountSeatsInUse :one
-- Seats are held by invited and active users (DATABASE.md §3.1).
SELECT count(*) FROM users WHERE status IN ('invited', 'active');

-- name: GetUserByEmail :one
SELECT id, email, first_name, last_name, user_type, status, password_hash
FROM users
WHERE email = $1;

-- name: ListStatusesOfDefaultWorkflow :many
SELECT s.id, s.name, s.grp, s.color, s.position
FROM statuses s
JOIN workflows w ON w.id = s.workflow_id
WHERE w.is_default
ORDER BY s.position;
