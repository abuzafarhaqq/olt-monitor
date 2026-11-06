package store

import (
	"context"
	"time"

	"github.com/abuzafarhaqq/olt-monitor-backend/internal/domain"
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

// AuditStore records who did what. Separate from Store so a read-only fixture
// does not have to pretend to keep an audit trail.
type AuditStore interface {
	RecordAudit(ctx context.Context, actor, action, target, detail string) error
}

// OltWrite is the editable shape of a device. Defined here, not in postgres, so
// the HTTP layer can write devices without importing a storage driver.
type OltWrite struct {
	ID                string
	Name              string
	Vendor            string
	Model             string
	Technology        string
	ManagementAddress string
	ManagementPort    int
	Connection        string
	Enabled           bool
}

// DeviceStore is the write side of the inventory.
type DeviceStore interface {
	Store
	CreateOlt(ctx context.Context, p OltWrite) (domain.Olt, error)
	UpdateOlt(ctx context.Context, p OltWrite) (domain.Olt, error)
}
