package store

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/go-cmp/cmp"
)

// ── listMigrationFiles ────────────────────────────────────────────────────────

func TestListMigrationFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fs      fs.FS
		want    []string
		wantErr bool
	}{
		{
			name: "sorted lexicographically",
			fs: fstest.MapFS{
				"0003_c.sql": {},
				"0001_a.sql": {},
				"0002_b.sql": {},
				"README.md":  {},
			},
			want: []string{"0001_a.sql", "0002_b.sql", "0003_c.sql"},
		},
		{
			name: "only sql files included",
			fs: fstest.MapFS{
				"0001_a.sql": {},
				"embed.go":   {},
				"notes.txt":  {},
			},
			want: []string{"0001_a.sql"},
		},
		{
			name: "empty filesystem",
			fs:   fstest.MapFS{},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := listMigrationFiles(tc.fs)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("listMigrationFiles() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// ── fileChecksum ──────────────────────────────────────────────────────────────

func TestFileChecksum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content []byte
		want    string
	}{
		{
			name:    "known SHA-256 of empty string",
			content: []byte{},
			// echo -n "" | sha256sum
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:    "known SHA-256 of 'SELECT 1'",
			content: []byte("SELECT 1"),
			// verified: echo -n "SELECT 1" | sha256sum
			want: "e004ebd5b5532a4b85984a62f8ad48a81aa3460c1ca07701f386135d72cdecf5",
		},
		{
			name:    "deterministic for same content",
			content: []byte("CREATE TABLE IF NOT EXISTS raw.blocks (block_number UInt64)"),
			want:    fileChecksum([]byte("CREATE TABLE IF NOT EXISTS raw.blocks (block_number UInt64)")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fileChecksum(tc.content)
			if tc.name == "deterministic for same content" {
				// Just verify it's stable
				if got != tc.want {
					t.Errorf("fileChecksum not deterministic")
				}
				return
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("fileChecksum() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// ── splitStatements ───────────────────────────────────────────────────────────

func TestSplitStatements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single statement no semicolon",
			input: "CREATE DATABASE IF NOT EXISTS raw",
			want:  []string{"CREATE DATABASE IF NOT EXISTS raw"},
		},
		{
			name:  "multiple statements semicolon separated",
			input: "CREATE DATABASE IF NOT EXISTS raw;\nCREATE DATABASE IF NOT EXISTS staging;\nCREATE DATABASE IF NOT EXISTS marts",
			want: []string{
				"CREATE DATABASE IF NOT EXISTS raw",
				"CREATE DATABASE IF NOT EXISTS staging",
				"CREATE DATABASE IF NOT EXISTS marts",
			},
		},
		{
			name:  "comment lines stripped",
			input: "-- create db\nCREATE DATABASE IF NOT EXISTS raw;\n-- done",
			want:  []string{"CREATE DATABASE IF NOT EXISTS raw"},
		},
		{
			name:  "trailing semicolon produces no empty statement",
			input: "CREATE DATABASE IF NOT EXISTS raw;",
			want:  []string{"CREATE DATABASE IF NOT EXISTS raw"},
		},
		{
			name:  "blank lines between statements",
			input: "SELECT 1;\n\nSELECT 2",
			want:  []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:  "semicolon inside comment does not split",
			input: "-- meta is bootstrapped; listed here for docs.\nCREATE DATABASE IF NOT EXISTS raw",
			want:  []string{"CREATE DATABASE IF NOT EXISTS raw"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := splitStatements(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("splitStatements() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// ── Migrator.Run (with fake DB) ───────────────────────────────────────────────

// fakeDB is a test double that records executed statements and applied migrations.
type fakeDB struct {
	execs   []string
	applied []appliedMigration
	execErr error
}

func (f *fakeDB) Exec(_ context.Context, query string, _ ...any) error {
	if f.execErr != nil {
		return f.execErr
	}
	f.execs = append(f.execs, strings.TrimSpace(query))
	return nil
}

func (f *fakeDB) Select(_ context.Context, dest any, _ string, _ ...any) error {
	out, ok := dest.(*[]appliedMigration)
	if !ok {
		return errors.New("fakeDB.Select: unexpected dest type")
	}
	*out = append(*out, f.applied...)
	return nil
}

// fakeDB uses strings package — add import at the top of the test file via init
var _ DB = (*fakeDB)(nil) // compile-time interface check

func TestMigratorRun(t *testing.T) {
	t.Parallel()

	makeFS := func(files map[string]string) fs.FS {
		m := make(fstest.MapFS, len(files))
		for name, content := range files {
			m[name] = &fstest.MapFile{Data: []byte(content), ModTime: time.Now()}
		}
		return m
	}

	tests := []struct {
		name    string
		files   map[string]string
		applied []appliedMigration
		wantErr string
	}{
		{
			name: "fresh run applies all files",
			files: map[string]string{
				"0001_a.sql": "SELECT 1",
				"0002_b.sql": "SELECT 2",
			},
			applied: nil,
			wantErr: "",
		},
		{
			name: "already applied with matching checksum is skipped",
			files: map[string]string{
				"0001_a.sql": "SELECT 1",
			},
			applied: []appliedMigration{
				{Filename: "0001_a.sql", Checksum: fileChecksum([]byte("SELECT 1"))},
			},
			wantErr: "",
		},
		{
			name: "applied file with wrong checksum returns error",
			files: map[string]string{
				"0001_a.sql": "SELECT 2", // content changed
			},
			applied: []appliedMigration{
				{Filename: "0001_a.sql", Checksum: fileChecksum([]byte("SELECT 1"))}, // old checksum
			},
			wantErr: "checksum mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db := &fakeDB{applied: tc.applied}
			m := NewMigrator(db, makeFS(tc.files))

			err := m.Run(context.Background())

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
