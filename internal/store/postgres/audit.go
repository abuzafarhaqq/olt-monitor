package postgres

import (
	"context"
	"encoding/json"
	"fmt"
)

// RecordAudit writes one line to audit_log. detail is JSON-encoded when it is
// valid JSON, and stored as a JSON string otherwise — an audit row must never
// be lost because the caller passed something unexpected.
func (s *Store) RecordAudit(ctx context.Context, actor, action, target, detail string) error {
	var payload []byte
	if detail == "" {
		payload = nil
	} else if json.Valid([]byte(detail)) {
		payload = []byte(detail)
	} else {
		encoded, err := json.Marshal(detail)
		if err != nil {
			return fmt.Errorf("encode audit detail: %w", err)
		}
		payload = encoded
	}

	if _, err := s.pool.Exec(ctx,
		`INSERT INTO audit_log (actor, action, target, detail) VALUES ($1, $2, $3, $4)`,
		actor, action, target, payload); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}
