package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
)

const listUsersQuery = `
SELECT id::text, username, full_name, email, phone, role::text, disabled,
       password_hash, last_login_at
FROM users
ORDER BY lower(username)`

func (s *Store) ListUsers(ctx context.Context) ([]store.User, error) {
	rows, err := s.pool.Query(ctx, listUsersQuery)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	out := make([]store.User, 0, 8)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, user)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUser(ctx context.Context, p store.UserWrite) (store.User, error) {
	_, err := s.pool.Exec(ctx, `
		UPDATE users
		   SET full_name = $2, email = $3, phone = NULLIF($4, ''),
		       role = $5::user_role, disabled = $6, updated_at = now()
		 WHERE id = $1`,
		p.ID, p.FullName, p.Email, p.Phone, p.Role, p.Disabled)
	if err != nil {
		if isUniqueViolation(err) {
			return store.User{}, store.ErrConflict
		}
		return store.User{}, fmt.Errorf("update user: %w", err)
	}
	return s.UserByID(ctx, p.ID)
}

func (s *Store) SetPasswordHash(ctx context.Context, id, hash string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, hash)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	// A password change invalidates every session minted with the old one.
	if _, err := s.pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, id); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) CountEnabledAdmins(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*)::int FROM users WHERE role = 'administrator' AND disabled = FALSE`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return count, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// NormaliseRole keeps the API and the permission table in step: an unknown role
// string would silently grant nothing instead of failing.
func NormaliseRole(role string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "administrator":
		return "administrator", true
	case "network-engineer":
		return "network-engineer", true
	case "noc-operator":
		return "noc-operator", true
	case "viewer":
		return "viewer", true
	default:
		return "", false
	}
}

// NormaliseRoleOrAdmin exists for the bootstrap path: the first account is an
// administrator by definition, and the normaliser keeps the string canonical.
func NormaliseRoleOrAdmin() string {
	role, _ := NormaliseRole("administrator")
	return role
}
