// Package migrations embeds *.sql files applied by internal/store.Migrator.
// This package lives next to the SQL files so the embed path has no ".." elements.
package migrations

import "embed"

// FS holds all *.sql migration files in this directory, read by embed at compile time.
//
//go:embed *.sql
var FS embed.FS
