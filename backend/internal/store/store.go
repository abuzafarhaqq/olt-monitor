// Package store defines the read side of the platform.
//
// The interface exists so the HTTP layer never knows whether it is talking to
// Postgres or to the in-memory fixture used by tests. It also keeps the
// boundary honest: handlers cannot run arbitrary SQL.
package store

import (
	"context"
	"errors"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/domain"
)

// ErrNotFound is returned when a requested identifier does not exist. The HTTP
// layer maps it to 404 and nothing else — a missing ONU is not a server error.
var ErrNotFound = errors.New("not found")

type Store interface {
	Overview(ctx context.Context) (domain.Overview, error)

	Olts(ctx context.Context) ([]domain.Olt, error)
	Olt(ctx context.Context, id string) (domain.Olt, error)
	PonsByOlt(ctx context.Context, oltID string) ([]domain.Pon, error)

	OnusByPon(ctx context.Context, ponID string) ([]domain.Onu, error)
	Onu(ctx context.Context, id string) (domain.Onu, error)
	OpticalHistory(ctx context.Context, onuID string) ([]domain.OpticalPoint, error)

	Events(ctx context.Context) ([]domain.Event, error)

	Incidents(ctx context.Context) ([]domain.Incident, error)
	Incident(ctx context.Context, id string) (domain.Incident, error)

	CollectionJobs(ctx context.Context) ([]domain.CollectionJob, error)
}
