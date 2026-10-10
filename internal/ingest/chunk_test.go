package ingest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPlanChunks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		from      uint64
		to        uint64
		chunkSize uint64
		want      []Chunk
	}{
		{
			name:      "exact fit",
			from:      0,
			to:        3,
			chunkSize: 2,
			want:      []Chunk{{0, 1}, {2, 3}},
		},
		{
			name:      "remainder chunk",
			from:      0,
			to:        4,
			chunkSize: 2,
			want:      []Chunk{{0, 1}, {2, 3}, {4, 4}},
		},
		{
			name:      "single block",
			from:      100,
			to:        100,
			chunkSize: 2000,
			want:      []Chunk{{100, 100}},
		},
		{
			name:      "from > to returns nil",
			from:      10,
			to:        5,
			chunkSize: 2,
			want:      nil,
		},
		{
			name:      "zero chunkSize uses default",
			from:      0,
			to:        DefaultChunkSize - 1,
			chunkSize: 0,
			want:      []Chunk{{0, DefaultChunkSize - 1}},
		},
		{
			name:      "large range",
			from:      1_000_000,
			to:        1_000_009,
			chunkSize: 3,
			want: []Chunk{
				{1_000_000, 1_000_002},
				{1_000_003, 1_000_005},
				{1_000_006, 1_000_008},
				{1_000_009, 1_000_009},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PlanChunks(tc.from, tc.to, tc.chunkSize)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("PlanChunks mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCursorKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		jobPrefix string
		chunk     Chunk
		want      string
	}{
		{
			name:      "backfill chunk",
			jobPrefix: "backfill",
			chunk:     Chunk{From: 1000, To: 2999},
			want:      "backfill:1000-2999",
		},
		{
			name:      "zero range",
			jobPrefix: "backfill",
			chunk:     Chunk{From: 0, To: 0},
			want:      "backfill:0-0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := cursorKey(tc.jobPrefix, tc.chunk)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("cursorKey mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
