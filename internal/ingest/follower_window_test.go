package ingest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReorgWindow_Push(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		maxSize int
		push    []blockRef
		want    []blockRef
	}{
		{
			name:    "empty_window",
			maxSize: 3,
			push:    nil,
			want:    nil,
		},
		{
			name:    "within_capacity",
			maxSize: 3,
			push:    []blockRef{{1, "a"}, {2, "b"}},
			want:    []blockRef{{1, "a"}, {2, "b"}},
		},
		{
			name:    "exact_capacity",
			maxSize: 3,
			push:    []blockRef{{1, "a"}, {2, "b"}, {3, "c"}},
			want:    []blockRef{{1, "a"}, {2, "b"}, {3, "c"}},
		},
		{
			name:    "evicts_oldest",
			maxSize: 3,
			push:    []blockRef{{1, "a"}, {2, "b"}, {3, "c"}, {4, "d"}},
			want:    []blockRef{{2, "b"}, {3, "c"}, {4, "d"}},
		},
		{
			name:    "evicts_multiple",
			maxSize: 2,
			push:    []blockRef{{1, "a"}, {2, "b"}, {3, "c"}, {4, "d"}},
			want:    []blockRef{{3, "c"}, {4, "d"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := newReorgWindow(tc.maxSize)
			for _, b := range tc.push {
				w.push(b)
			}
			if diff := cmp.Diff(tc.want, w.entries); diff != "" {
				t.Errorf("entries mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReorgWindow_Last(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []blockRef
		wantRef blockRef
		wantOK  bool
	}{
		{
			name:    "empty",
			entries: nil,
			wantOK:  false,
		},
		{
			name:    "single",
			entries: []blockRef{{5, "x"}},
			wantRef: blockRef{5, "x"},
			wantOK:  true,
		},
		{
			name:    "multiple",
			entries: []blockRef{{1, "a"}, {2, "b"}, {3, "c"}},
			wantRef: blockRef{3, "c"},
			wantOK:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := &reorgWindow{entries: tc.entries}
			got, ok := w.last()
			if ok != tc.wantOK {
				t.Errorf("ok: want %v, got %v", tc.wantOK, ok)
			}
			if ok {
				if diff := cmp.Diff(tc.wantRef, got); diff != "" {
					t.Errorf("ref mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestReorgWindow_FindByHash(t *testing.T) {
	t.Parallel()

	entries := []blockRef{{1, "a"}, {2, "b"}, {3, "c"}, {4, "d"}}

	tests := []struct {
		name    string
		hash    string
		wantIdx int
	}{
		{name: "first_entry", hash: "a", wantIdx: 0},
		{name: "middle_entry", hash: "b", wantIdx: 1},
		{name: "last_entry", hash: "d", wantIdx: 3},
		{name: "not_found", hash: "z", wantIdx: -1},
		{name: "empty_hash", hash: "", wantIdx: -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := &reorgWindow{entries: append([]blockRef(nil), entries...)}
			got := w.findByHash(tc.hash)
			if got != tc.wantIdx {
				t.Errorf("findByHash(%q): want %d, got %d", tc.hash, tc.wantIdx, got)
			}
		})
	}
}

func TestReorgWindow_TruncateAfter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []blockRef
		idx     int
		want    []blockRef
	}{
		{
			name:    "truncate_after_first",
			entries: []blockRef{{1, "a"}, {2, "b"}, {3, "c"}},
			idx:     0,
			want:    []blockRef{{1, "a"}},
		},
		{
			name:    "truncate_after_middle",
			entries: []blockRef{{1, "a"}, {2, "b"}, {3, "c"}},
			idx:     1,
			want:    []blockRef{{1, "a"}, {2, "b"}},
		},
		{
			name:    "truncate_after_last_is_noop",
			entries: []blockRef{{1, "a"}, {2, "b"}, {3, "c"}},
			idx:     2,
			want:    []blockRef{{1, "a"}, {2, "b"}, {3, "c"}},
		},
		{
			name:    "negative_idx_is_noop",
			entries: []blockRef{{1, "a"}, {2, "b"}},
			idx:     -1,
			want:    []blockRef{{1, "a"}, {2, "b"}},
		},
		{
			name:    "out_of_range_idx_is_noop",
			entries: []blockRef{{1, "a"}, {2, "b"}},
			idx:     10,
			want:    []blockRef{{1, "a"}, {2, "b"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := &reorgWindow{entries: append([]blockRef(nil), tc.entries...)}
			w.truncateAfter(tc.idx)
			if diff := cmp.Diff(tc.want, w.entries); diff != "" {
				t.Errorf("entries mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReorgWindow_LastNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []blockRef
		want    uint64
	}{
		{name: "empty", entries: nil, want: 0},
		{name: "single", entries: []blockRef{{42, "x"}}, want: 42},
		{name: "multiple", entries: []blockRef{{1, "a"}, {2, "b"}, {100, "z"}}, want: 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := &reorgWindow{entries: tc.entries}
			if got := w.lastNumber(); got != tc.want {
				t.Errorf("lastNumber: want %d, got %d", tc.want, got)
			}
		})
	}
}
