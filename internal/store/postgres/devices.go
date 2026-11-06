package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/abuzafarhaqq/olt-monitor-backend/internal/domain"
	"github.com/abuzafarhaqq/olt-monitor-backend/internal/store"
)

// CreateOlt inserts the device. It does not create olt_state: a device has no
// state until something has collected from it, and pretending otherwise is how
// a brand-new OLT shows up as healthy.
func (s *Store) CreateOlt(ctx context.Context, p store.OltWrite) (domain.Olt, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO olts (id, name, vendor, model, technology, management_address,
		                  management_port, connection, collection_profile, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'standard-5m', $9)`,
		p.ID, p.Name, p.Vendor, p.Model, p.Technology, p.ManagementAddress,
		p.ManagementPort, p.Connection, p.Enabled)
	if err != nil {
		return domain.Olt{}, fmt.Errorf("create olt: %w", err)
	}
	return s.Olt(ctx, p.ID)
}

// UpdateOlt changes the fields an operator may edit. Technology is included
// because a mis-typed technology silently dirties every threshold lookup.
func (s *Store) UpdateOlt(ctx context.Context, p store.OltWrite) (domain.Olt, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE olts
		   SET name = $2, vendor = $3, model = $4, technology = $5,
		       management_address = $6, management_port = $7, connection = $8,
		       enabled = $9
		 WHERE id = $1`,
		p.ID, p.Name, p.Vendor, p.Model, p.Technology,
		p.ManagementAddress, p.ManagementPort, p.Connection, p.Enabled)
	if err != nil {
		return domain.Olt{}, fmt.Errorf("update olt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Olt{}, store.ErrNotFound
	}
	return s.Olt(ctx, p.ID)
}

// --- credentials ---------------------------------------------------------
//
// The database only ever sees ciphertext. Sealing and opening happen in
// internal/secrets, which holds the key.

type CredentialMeta struct {
	Username   string
	KeyVersion int
	UpdatedAt  time.Time
	UpdatedBy  *string
}

func (s *Store) UpsertCredential(ctx context.Context, oltID, username string, sealed []byte, keyVersion int, updatedBy string) error {
	var by *string
	if updatedBy != "" {
		by = &updatedBy
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO olt_credentials (olt_id, username, secret_enc, key_version, updated_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (olt_id) DO UPDATE
		   SET username = EXCLUDED.username,
		       secret_enc = EXCLUDED.secret_enc,
		       key_version = EXCLUDED.key_version,
		       updated_at = now(),
		       updated_by = EXCLUDED.updated_by`,
		oltID, username, sealed, keyVersion, by)
	if err != nil {
		return fmt.Errorf("upsert credential: %w", err)
	}
	return nil
}

func (s *Store) CredentialMeta(ctx context.Context, oltID string) (CredentialMeta, error) {
	var (
		meta CredentialMeta
		by   *string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT username, key_version, updated_at, updated_by::text
		  FROM olt_credentials WHERE olt_id = $1`, oltID).
		Scan(&meta.Username, &meta.KeyVersion, &meta.UpdatedAt, &by)
	if errors.Is(err, pgx.ErrNoRows) {
		return CredentialMeta{}, store.ErrNotFound
	}
	if err != nil {
		return CredentialMeta{}, fmt.Errorf("credential meta: %w", err)
	}
	meta.UpdatedBy = by
	return meta, nil
}

func (s *Store) SealedCredential(ctx context.Context, oltID string) (username string, sealed []byte, keyVersion int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT username, secret_enc, key_version
		  FROM olt_credentials WHERE olt_id = $1`, oltID).
		Scan(&username, &sealed, &keyVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, 0, store.ErrNotFound
	}
	if err != nil {
		return "", nil, 0, fmt.Errorf("sealed credential: %w", err)
	}
	return username, sealed, keyVersion, nil
}

func (s *Store) DeleteCredential(ctx context.Context, oltID string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM olt_credentials WHERE olt_id = $1`, oltID); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	return nil
}
