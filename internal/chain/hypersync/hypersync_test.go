package hypersync_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain/hypersync"
)

const goldenResponse = "../../../testdata/hs_response_001.json"

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func collect(t *testing.T, blocks <-chan *chain.FetchedBlock, errc <-chan error) ([]*chain.FetchedBlock, error) {
	t.Helper()
	var out []*chain.FetchedBlock
	for fb := range blocks {
		out = append(out, fb)
	}
	return out, <-errc
}

func nums(fbs []*chain.FetchedBlock) []uint64 {
	var out []uint64
	for _, fb := range fbs {
		out = append(out, uint64(fb.Block.Number))
	}
	return out
}

func TestRange(t *testing.T) {
	t.Parallel()

	golden, err := os.ReadFile(goldenResponse)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		from, to   uint64
		wantBlocks []uint64
	}{
		{
			name: "single page all blocks",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(golden)
			},
			from: 100, to: 102,
			wantBlocks: []uint64{100, 101, 102},
		},
		{
			name: "range clips to requested bound",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(golden)
			},
			from: 100, to: 101,
			wantBlocks: []uint64{100, 101},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			src := hypersync.New(srv.URL, "", discardLogger())
			blocks, errc := src.Range(context.Background(), tc.from, tc.to, nil)
			fbs, cerr := collect(t, blocks, errc)
			if cerr != nil {
				t.Fatalf("unexpected error: %v", cerr)
			}
			if diff := cmp.Diff(tc.wantBlocks, nums(fbs)); diff != "" {
				t.Errorf("blocks mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRangeConvertsFields(t *testing.T) {
	t.Parallel()

	golden, err := os.ReadFile(goldenResponse)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(golden)
	}))
	defer srv.Close()

	src := hypersync.New(srv.URL, "", discardLogger())
	blocks, errc := src.Range(context.Background(), 100, 100, nil)
	fbs, cerr := collect(t, blocks, errc)
	if cerr != nil {
		t.Fatalf("unexpected error: %v", cerr)
	}
	if len(fbs) != 1 {
		t.Fatalf("got %d blocks, want 1", len(fbs))
	}
	b := fbs[0]
	if got := uint64(b.Block.GasUsed); got != 21000 {
		t.Errorf("gasUsed = %d, want 21000", got)
	}
	if got := b.Block.BaseFeePerGas.ToInt().Uint64(); got != 1000000000 {
		t.Errorf("baseFee = %d, want 1e9", got)
	}
	if got := len(b.Block.Transactions); got != 2 {
		t.Errorf("tx count = %d, want 2", got)
	}
	if got := len(b.Receipts); got != 2 {
		t.Errorf("receipt count = %d, want 2", got)
	}
	if got := uint64(b.Receipts[0].GasUsedForL1); got != 5000 {
		t.Errorf("gasUsedForL1 = %d, want 5000", got)
	}
	if got := len(b.Logs); got != 1 {
		t.Errorf("log count = %d, want 1", got)
	}
	if got := len(b.Logs[0].Topics); got != 1 {
		t.Errorf("topic count = %d, want 1", got)
	}
}

func TestRetryAfter429(t *testing.T) {
	t.Parallel()

	golden, err := os.ReadFile(goldenResponse)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(golden)
	}))
	defer srv.Close()

	src := hypersync.New(srv.URL, "", discardLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	blocks, errc := src.Range(ctx, 100, 102, nil)
	fbs, cerr := collect(t, blocks, errc)
	if cerr != nil {
		t.Fatalf("unexpected error: %v", cerr)
	}
	if diff := cmp.Diff([]uint64{100, 101, 102}, nums(fbs)); diff != "" {
		t.Errorf("blocks mismatch (-want +got):\n%s", diff)
	}
	if calls.Load() < 2 {
		t.Errorf("expected at least 2 calls, got %d", calls.Load())
	}
}

func TestHead(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/height" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, `{"height":12345}`)
	}))
	defer srv.Close()

	src := hypersync.New(srv.URL, "", discardLogger())
	got, err := src.Head(context.Background())
	if err != nil {
		t.Fatalf("head error: %v", err)
	}
	if got != 12345 {
		t.Errorf("head = %d, want 12345", got)
	}
}

func TestAuthHeader(t *testing.T) {
	t.Parallel()

	var gotAuth atomic.Value
	gotAuth.Store("")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, `{"height":1}`)
	}))
	defer srv.Close()

	src := hypersync.New(srv.URL, "secret-token", discardLogger())
	if _, err := src.Head(context.Background()); err != nil {
		t.Fatalf("head error: %v", err)
	}
	if got := gotAuth.Load().(string); got != "Bearer secret-token" {
		t.Errorf("auth header = %q, want %q", got, "Bearer secret-token")
	}
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	// Release the handler first, then close the server (LIFO defer order).
	defer srv.Close()
	defer close(release)

	src := hypersync.New(srv.URL, "", discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	blocks, errc := src.Range(ctx, 100, 102, nil)
	for range blocks { //nolint:revive
	}
	if err := <-errc; err == nil {
		t.Error("expected cancellation error, got nil")
	}
}
