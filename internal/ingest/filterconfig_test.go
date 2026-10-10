package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadLogFilterAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		poolsYAML string
		contsYAML string
		want      []string
	}{
		{
			name:      "both empty",
			poolsYAML: "pools: []\n",
			contsYAML: "contracts: []\n",
			want:      nil,
		},
		{
			name: "one pool one contract",
			poolsYAML: `pools:
  - address: "0xAAA"
    name: foo
`,
			contsYAML: `contracts:
  - address: "0xBBB"
    name: bar
`,
			want: []string{"0xaaa", "0xbbb"},
		},
		{
			name: "duplicate addresses deduplicated",
			poolsYAML: `pools:
  - address: "0xAAA"
  - address: "0xAAA"
`,
			contsYAML: `contracts:
  - address: "0xAAA"
`,
			want: []string{"0xaaa"},
		},
		{
			name: "lowercases addresses",
			poolsYAML: `pools:
  - address: "0XDEADBEEF00000000000000000000000000000000"
`,
			contsYAML: "contracts: []\n",
			want:      []string{"0xdeadbeef00000000000000000000000000000000"},
		},
		{
			name:      "empty address entries skipped",
			poolsYAML: "pools:\n  - address: \"\"\n",
			contsYAML: "contracts: []\n",
			want:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			poolsPath := writeYAML(t, dir, "pools.yaml", tc.poolsYAML)
			contsPath := writeYAML(t, dir, "contracts.yaml", tc.contsYAML)

			got, err := LoadLogFilterAddresses(poolsPath, contsPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("LoadLogFilterAddresses mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadLogFilterAddresses_MissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := LoadLogFilterAddresses(
		filepath.Join(dir, "missing.yaml"),
		filepath.Join(dir, "also_missing.yaml"),
	)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
