package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

// DB is the minimal ClickHouse interface used by Migrator.
// clickhouse.Conn satisfies this interface.
type DB interface {
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
}

// Migrator applies SQL migrations embedded in an fs.FS idempotently.
// Migrations are tracked by filename and SHA-256 checksum in meta.schema_migrations.
// A run is refused if any previously applied file has changed checksum.
type Migrator struct {
	db         DB
	migrations fs.FS
}

// NewMigrator constructs a Migrator. migrations must be the embedded FS from
// infra/clickhouse/migrations (package migrations).
func NewMigrator(db DB, migrations fs.FS) *Migrator {
	return &Migrator{db: db, migrations: migrations}
}

// appliedMigration is scanned from meta.schema_migrations.
type appliedMigration struct {
	Filename string `ch:"filename"`
	Checksum string `ch:"checksum"`
}

// Run applies all pending migrations in lexicographic order.
// It is safe to call multiple times; already-applied migrations are skipped.
// Returns an error if any previously-applied file has a changed checksum.
func (m *Migrator) Run(ctx context.Context) error {
	if err := m.bootstrap(ctx); err != nil {
		return fmt.Errorf("migrate bootstrap: %w", err)
	}

	files, err := listMigrationFiles(m.migrations)
	if err != nil {
		return fmt.Errorf("migrate list files: %w", err)
	}

	applied, err := m.loadApplied(ctx)
	if err != nil {
		return fmt.Errorf("migrate load applied: %w", err)
	}

	for _, filename := range files {
		content, err := fs.ReadFile(m.migrations, filename)
		if err != nil {
			return fmt.Errorf("migrate read %s: %w", filename, err)
		}

		checksum := fileChecksum(content)

		if prev, ok := applied[filename]; ok {
			if prev != checksum {
				return fmt.Errorf("migrate: applied file %q checksum mismatch (applied=%s current=%s): refusing to run", filename, prev, checksum)
			}
			continue // already applied, checksum matches — skip
		}

		if err := m.applyFile(ctx, filename, content); err != nil {
			return fmt.Errorf("migrate apply %s: %w", filename, err)
		}

		if err := m.record(ctx, filename, checksum); err != nil {
			return fmt.Errorf("migrate record %s: %w", filename, err)
		}
	}

	return nil
}

// bootstrap ensures the meta database and schema_migrations table exist.
// These are NOT tracked as migrations themselves to avoid the chicken-and-egg problem.
func (m *Migrator) bootstrap(ctx context.Context) error {
	for _, q := range []string{
		`CREATE DATABASE IF NOT EXISTS meta`,
		`CREATE TABLE IF NOT EXISTS meta.schema_migrations
		 (
		     filename   String,
		     checksum   FixedString(64),
		     applied_at DateTime64(3, 'UTC') DEFAULT now64(3)
		 )
		 ENGINE = ReplacingMergeTree(applied_at)
		 ORDER BY filename`,
	} {
		if err := m.db.Exec(ctx, q); err != nil {
			return fmt.Errorf("bootstrap %q: %w", shortQuery(q), err)
		}
	}
	return nil
}

// loadApplied returns a map of filename → checksum for all applied migrations.
func (m *Migrator) loadApplied(ctx context.Context) (map[string]string, error) {
	var rows []appliedMigration
	if err := m.db.Select(ctx, &rows,
		`SELECT filename, checksum FROM meta.schema_migrations FINAL ORDER BY filename`,
	); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Filename] = r.Checksum
	}
	return out, nil
}

// applyFile executes every statement in a migration file.
func (m *Migrator) applyFile(ctx context.Context, filename string, content []byte) error {
	stmts := splitStatements(string(content))
	for i, stmt := range stmts {
		if err := m.db.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("%s statement %d %q: %w", filename, i+1, shortQuery(stmt), err)
		}
	}
	return nil
}

// record inserts an applied migration entry into meta.schema_migrations.
func (m *Migrator) record(ctx context.Context, filename, checksum string) error {
	return m.db.Exec(ctx,
		`INSERT INTO meta.schema_migrations (filename, checksum, applied_at) VALUES (?, ?, ?)`,
		filename, checksum, time.Now().UTC(),
	)
}

// listMigrationFiles returns *.sql filenames from fsys, sorted lexicographically.
func listMigrationFiles(fsys fs.FS) ([]string, error) {
	var files []string
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// fileChecksum returns the lowercase hex-encoded SHA-256 of content.
func fileChecksum(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// splitStatements strips -- comment lines then splits on semicolons.
// Stripping comments first means semicolons inside comments do not cause spurious splits.
// Migration files must not use semicolons inside string literals.
func splitStatements(sql string) []string {
	// Pass 1: drop comment lines so their semicolons are invisible to the splitter.
	var kept []string
	for _, line := range strings.Split(sql, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			kept = append(kept, line)
		}
	}

	// Pass 2: split on semicolons and collect non-empty statements.
	stmts := make([]string, 0, 16)
	for _, part := range strings.Split(strings.Join(kept, "\n"), ";") {
		if s := strings.TrimSpace(part); s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}

// shortQuery returns the first 60 chars of a query for error messages.
func shortQuery(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	if len(q) > 60 {
		return q[:60] + "…"
	}
	return q
}

// ErrChecksumMismatch is returned when an applied migration file has changed.
var ErrChecksumMismatch = errors.New("checksum mismatch")
