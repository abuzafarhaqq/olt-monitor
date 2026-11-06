// Package secrets is the only place a device password exists in the clear.
//
// Nothing above it — no handler, no template, no log line — may hold one: the
// API returns metadata, and the collector calls Resolve immediately before
// dialling and drops the value when the session ends.
package secrets

import (
	"context"
	"fmt"
	"time"

	"github.com/abuzafarhaqq/olt-monitor-backend/internal/crypto"
	"github.com/abuzafarhaqq/olt-monitor-backend/internal/store/postgres"
)

// Backend is the narrow slice of storage this package needs.
type Backend interface {
	UpsertCredential(ctx context.Context, oltID, username string, sealed []byte, keyVersion int, updatedBy string) error
	CredentialMeta(ctx context.Context, oltID string) (postgres.CredentialMeta, error)
	SealedCredential(ctx context.Context, oltID string) (string, []byte, int, error)
	DeleteCredential(ctx context.Context, oltID string) error
}

type Service struct {
	ring  *crypto.Keyring
	store Backend
}

func New(ring *crypto.Keyring, backend Backend) *Service {
	return &Service{ring: ring, store: backend}
}

type Metadata struct {
	Username   string    `json:"username"`
	KeyVersion int       `json:"keyVersion"`
	UpdatedAt  time.Time `json:"updatedAt"`
	UpdatedBy  *string   `json:"updatedBy"`
}

// Set encrypts immediately. The plaintext is never persisted, logged or
// returned by this call.
func (s *Service) Set(ctx context.Context, oltID, username, password, updatedBy string) error {
	sealed, err := s.ring.Seal([]byte(password))
	if err != nil {
		return fmt.Errorf("seal credential: %w", err)
	}
	return s.store.UpsertCredential(ctx, oltID, username, sealed, s.ring.Version, updatedBy)
}

func (s *Service) Meta(ctx context.Context, oltID string) (Metadata, error) {
	meta, err := s.store.CredentialMeta(ctx, oltID)
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{
		Username:   meta.Username,
		KeyVersion: meta.KeyVersion,
		UpdatedAt:  meta.UpdatedAt,
		UpdatedBy:  meta.UpdatedBy,
	}, nil
}

func (s *Service) Delete(ctx context.Context, oltID string) error {
	return s.store.DeleteCredential(ctx, oltID)
}

// Resolve is for the collector only. Callers must not keep the returned string
// beyond the connection it feeds.
func (s *Service) Resolve(ctx context.Context, oltID string) (username, password string, err error) {
	username, sealed, version, err := s.store.SealedCredential(ctx, oltID)
	if err != nil {
		return "", "", err
	}
	if version != s.ring.Version {
		// A row sealed with an older key needs that key present during rotation;
		// refusing loudly beats returning garbage that fails at the device.
		return "", "", fmt.Errorf("credential sealed with key version %d, current is %d", version, s.ring.Version)
	}

	plaintext, err := s.ring.Open(sealed)
	if err != nil {
		return "", "", err
	}
	return username, string(plaintext), nil
}
