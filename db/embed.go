// Package db owns the schema. The SQL lives here rather than inside the Go
// module's original home so that the database is a first-class directory in the
// monorepo: migrations, the compose file and the seed docs sit together.
package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var embedded embed.FS

// FS is rooted at the migrations directory, so callers see "0001_init.sql"
// rather than "migrations/0001_init.sql" — the runner records bare file names
// in schema_migrations.
var FS = mustSub()

func mustSub() fs.FS {
	sub, err := fs.Sub(embedded, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}
