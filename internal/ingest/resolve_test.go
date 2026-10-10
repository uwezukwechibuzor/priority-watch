package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// stubQuerier implements BlockTimestampQuerier using a simple lookup table.
type stubQuerier struct {
	head       uint64
	timestamps map[uint64]uint64 // block number → unix timestamp
}

func (s *stubQuerier) HeadNumber(_ context.Context) (uint64, error) {
	return s.head, nil
}

func (s *stubQuerier) BlockTimestamp(_ context.Context, n uint64) (uint64, error) {
	ts, ok := s.timestamps[n]
	if !ok {
		return 0, errors.New("block not found in stub")
	}
	return ts, nil
}

// newLinearQuerier creates a stub where block n has timestamp base + n*blockTime.
func newLinearQuerier(head uint64, base, blockTime uint64) *stubQuerier {
	m := make(map[uint64]uint64, head+1)
	for i := uint64(0); i <= head; i++ {
		m[i] = base + i*blockTime
	}
	return &stubQuerier{head: head, timestamps: m}
}

func TestDateResolver_Resolve(t *testing.T) {
	t.Parallel()

	// blocks 0..999 with timestamps spaced 2 seconds apart starting at unix 1_000_000.
	// block n has timestamp 1_000_000 + n*2.
	q := newLinearQuerier(999, 1_000_000, 2)

	tests := []struct {
		name    string
		target  time.Time
		want    uint64
		wantErr bool
	}{
		{
			name:   "exact block 0",
			target: time.Unix(1_000_000, 0).UTC(),
			want:   0,
		},
		{
			name:   "exact block 500",
			target: time.Unix(1_000_000+500*2, 0).UTC(),
			want:   500,
		},
		{
			name:   "exact last block",
			target: time.Unix(1_000_000+999*2, 0).UTC(),
			want:   999,
		},
		{
			name:   "target after head clamps to head",
			target: time.Unix(2_000_000, 0).UTC(),
			want:   999,
		},
		{
			// block 250 ts = 1_000_500, block 251 ts = 1_000_502.
			// Target 1_000_501 falls between them; resolver returns block 251.
			name:   "target between blocks returns next block",
			target: time.Unix(1_000_501, 0).UTC(),
			want:   251,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := NewDateResolver(q)
			got, err := r.Resolve(context.Background(), tc.target)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Resolve mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDateResolver_HeadError(t *testing.T) {
	t.Parallel()

	q := &errQuerier{headErr: errors.New("rpc down")}
	r := NewDateResolver(q)
	_, err := r.Resolve(context.Background(), time.Now())
	if err == nil {
		t.Error("expected error from failing HeadNumber, got nil")
	}
}

func TestDateResolver_TimestampError(t *testing.T) {
	t.Parallel()

	// Head succeeds but BlockTimestamp always errors.
	q := &errQuerier{tsErr: errors.New("rpc down"), headTS: 1_000_000, head: 10}
	r := NewDateResolver(q)
	// target is before head so it triggers binary search.
	_, err := r.Resolve(context.Background(), time.Unix(999_999, 0))
	if err == nil {
		t.Error("expected error from failing BlockTimestamp, got nil")
	}
}

// errQuerier lets us inject errors into HeadNumber or BlockTimestamp.
type errQuerier struct {
	head    uint64
	headTS  uint64
	headErr error
	tsErr   error
}

func (q *errQuerier) HeadNumber(_ context.Context) (uint64, error) {
	return q.head, q.headErr
}

func (q *errQuerier) BlockTimestamp(_ context.Context, n uint64) (uint64, error) {
	if n == q.head {
		return q.headTS, nil
	}
	return 0, q.tsErr
}
