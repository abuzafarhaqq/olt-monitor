package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
)

const userColumns = `id::text, username, full_name, email, phone, role::text, disabled,
                     password_hash, last_login_at`

const userByUsernameQuery = `SELECT ` + userColumns + ` FROM users WHERE lower(username) = lower($1)`

const userByIDQuery = `SELECT ` + userColumns + ` FROM users WHERE id = $1`

// A session is only valid while it is unexpired, unrevoked and its user is
// enabled — all three conditions in one query, so no caller can forget one.
const userBySessionQuery = `
SELECT u.id::text, u.username, u.full_name, u.email, u.phone, u.role::text, u.disabled,
       u.password_hash, u.last_login_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND u.disabled = FALSE`

func scanUser(row pgx.Row) (store.User, error) {
	var user store.User
	err := row.Scan(
		&user.ID, &user.Username, &user.FullName, &user.Email, &user.Phone,
		&user.Role, &user.Disabled, &user.PasswordHash, &user.LastLoginAt,
	)
	return user, err
}

func (s *Store) UserByUsername(ctx context.Context, username string) (store.User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, userByUsernameQuery, username))
	if errors.Is(err, pgx.ErrNoRows) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("query user: %w", err)
	}
	return user, nil
}

func (s *Store) UserByID(ctx context.Context, id string) (store.User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, userByIDQuery, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("query user: %w", err)
	}
	return user, nil
}

func (s *Store) UserBySession(ctx context.Context, tokenHash []byte) (store.User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, userBySessionQuery, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("query session: %w", err)
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time, ip, userAgent string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at, ip, user_agent)
		VALUES ($1, $2, $3, NULLIF($4, '')::inet, NULLIF($5, ''))`,
		userID, tokenHash, expiresAt, ip, userAgent)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *Store) TouchLogin(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET last_login_at = now(), failed_attempts = 0 WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("touch login: %w", err)
	}
	return nil
}

// --- user management (used by tests and by oltctl, not by the read API) ---

type CreateUserParams struct {
	Username     string
	FullName     string
	Email        string
	Phone        string
	Role         string
	PasswordHash string
}

func (s *Store) CreateUser(ctx context.Context, params CreateUserParams) (store.User, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (username, full_name, email, phone, role, password_hash)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5::user_role, $6)
		RETURNING id::text`,
		params.Username, params.FullName, params.Email, params.Phone, params.Role, params.PasswordHash,
	).Scan(&id)
	if err != nil {
		return store.User{}, fmt.Errorf("create user: %w", err)
	}
	return s.UserByID(ctx, id)
}
