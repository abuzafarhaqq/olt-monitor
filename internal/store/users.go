package store

import (
	"context"
	"time"
)

// User carries the password hash because the authentication path needs it.
// Nothing serialises this struct directly — the API returns a narrower shape —
// so a hash cannot leave the server by accident.
type User struct {
	ID           string
	Username     string
	FullName     string
	Email        string
	Phone        *string
	Role         string
	Disabled     bool
	PasswordHash string
	LastLoginAt  *time.Time
}

// AuthStore is the slice of storage the login path needs. Keeping it separate
// from Store means the in-memory fixture keeps working without users, and the
// HTTP layer cannot reach user tables it has no business touching.
type AuthStore interface {
	UserByUsername(ctx context.Context, username string) (User, error)
	UserBySession(ctx context.Context, tokenHash []byte) (User, error)
	CreateSession(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time, ip, userAgent string) error
	RevokeSession(ctx context.Context, tokenHash []byte) error
	TouchLogin(ctx context.Context, userID string) error
}
