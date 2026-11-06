package postgres

import (
	"testing"

	"github.com/auxten/postgresql-parser/pkg/sql/parser"
)

// The SQL in this package cannot be executed without a database, so the next
// best check runs here: parse every runtime statement with a PostgreSQL
// grammar. This is what catches a WHERE appended after an ORDER BY.
//
// It proves syntax, not semantics. A misspelled column still parses and still
// fails at runtime — that is what the TEST_DATABASE_URL integration tests are
// for. schemaql.0001_init.sql is not covered: this parser is the CockroachDB
// grammar and does not implement CREATE TYPE ... AS ENUM or macaddr, neither of
// which is a reason to change a correct Postgres schema.
func TestQueriesParse(t *testing.T) {
	queries := map[string]string{
		"oltsQuery":           oltsQuery,
		"ponsQuery":           ponsQuery,
		"onusByPonQuery":      onusByPonQuery,
		"onuQuery":            onuQuery,
		"eventsQuery":         eventsQuery,
		"incidentsQuery":      incidentsQuery,
		"incidentQuery":       incidentQuery,
		"jobsQuery":           jobsQuery,
		"opticalHistoryQuery": opticalHistoryQuery,
		"allOnusQuery":        `SELECT ` + onuColumns + onuFrom + ` ORDER BY u.olt_id, u.pon_id, u.onu_number`,
		"incidentNotesQuery":  `SELECT id, author, created_at, body FROM incident_notes WHERE incident_id = $1 ORDER BY created_at`,
		"incidentEvidence":    `SELECT event_id FROM incident_evidence WHERE incident_id = $1 ORDER BY event_id`,
		"schemaMigrations":    `SELECT name FROM schema_migrations`,
	}

	for name, query := range queries {
		if _, err := parser.Parse(query); err != nil {
			t.Errorf("%s does not parse: %v\nquery: %s", name, err, query)
		}
	}
}
