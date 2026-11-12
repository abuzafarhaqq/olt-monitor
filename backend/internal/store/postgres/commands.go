package postgres

import (
	"context"
	"fmt"
	"time"
)

// DiscoveredCommand is one row of the crawled CLI tree.
type DiscoveredCommand struct {
	Path  string `json:"path"`
	Help  string `json:"help"`
	Kind  string `json:"kind"`
	Depth int    `json:"depth"`
}

// SaveCommands replaces a device's discovered tree.
//
// Replace, not merge: a firmware upgrade removes commands, and a merge would
// keep offering ones that no longer exist.
func (s *Store) SaveCommands(ctx context.Context, oltID, adapter string, commands []DiscoveredCommand, at time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM olt_commands WHERE olt_id = $1`, oltID); err != nil {
		return fmt.Errorf("clear commands: %w", err)
	}

	for _, command := range commands {
		if _, err := tx.Exec(ctx, `
			INSERT INTO olt_commands (olt_id, path, help, kind, depth, adapter, discovered_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (olt_id, path) DO UPDATE
			   SET help = EXCLUDED.help, kind = EXCLUDED.kind,
			       depth = EXCLUDED.depth, adapter = EXCLUDED.adapter,
			       discovered_at = EXCLUDED.discovered_at`,
			oltID, command.Path, command.Help, command.Kind, command.Depth, adapter, at); err != nil {
			return fmt.Errorf("insert command %q: %w", command.Path, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ListCommands returns a device's discovered commands, read paths first.
func (s *Store) ListCommands(ctx context.Context, oltID string) ([]DiscoveredCommand, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT path, help, kind, depth
		  FROM olt_commands
		 WHERE olt_id = $1
		 ORDER BY (kind = 'read') DESC, path`, oltID)
	if err != nil {
		return nil, fmt.Errorf("list commands: %w", err)
	}
	defer rows.Close()

	out := make([]DiscoveredCommand, 0, 64)
	for rows.Next() {
		var command DiscoveredCommand
		if err := rows.Scan(&command.Path, &command.Help, &command.Kind, &command.Depth); err != nil {
			return nil, fmt.Errorf("scan command: %w", err)
		}
		out = append(out, command)
	}
	return out, rows.Err()
}
