// Package migrations embeds the SQL files so the binary can migrate a fresh
// database without shipping a directory alongside it.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
