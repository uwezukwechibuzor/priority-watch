package sqd_test

import (
	"bytes"
	"compress/gzip"
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
	"github.com/uwezukwechibuzor/priority-watch/internal/chain/sqd"
)

const goldenBatch = "../../../testdata/sqd_batch_001.jsonl"

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// collect drains a Range into slices of block numbers and the first error.
func collect(t *testing.T, blocks <-chan *chain.FetchedBlock, errc <-chan error) ([]uint64, *chain.ReorgInfo, error) {
	t.Helper()
	var nums []uint64
	var reorg *chain.ReorgInfo
	for fb := range blocks {
		nums = append(nums, uint64(fb.Block.Number))
		if fb.Reorg != nil {
			reorg = fb.Reorg
		}
	}
	return nums, reorg, <-errc
}

func TestRangeParsing(t *testing.T) {
	t.Parallel()

	goldenNDJSON, err := os.ReadFile(goldenBatch)
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
			name: "plain ndjson batch",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/x-ndjson")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(goldenNDJSON)
			},
			from: 100, to: 102,
			wantBlocks: []uint64{100, 101, 102},
		},
		{
			name: "gzip compressed batch",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/x-ndjson")
				w.Header().Set("Content-Encoding", "gzip")
				w.WriteHeader(http.StatusOK)
				gz := gzip.NewWriter(w)
				_, _ = gz.Write(goldenNDJSON)
				_ = gz.Close()
			},
			from: 100, to: 102,
			wantBlocks: []uint64{100, 101, 102},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			src := sqd.New(srv.URL, discardLogger())
			blocks, errc := src.Range(context.Background(), tc.from, tc.to, nil)
			nums, _, err := collect(t, blocks, errc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.wantBlocks, nums); diff != "" {
				t.Errorf("blocks mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRangeConvertsFields(t *testing.T) {
	t.Parallel()

	goldenNDJSON, err := os.ReadFile(goldenBatch)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	srv := httptest.NewServer(func() http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = w.Write(goldenNDJSON)
		}
	}())
	defer srv.Close()

	src := sqd.New(srv.URL, discardLogger())
	blocks, errc := src.Range(context.Background(), 100, 100, nil)

	var first *chain.FetchedBlock
	for fb := range blocks {
		if first == nil {
			first = fb
		}
	}
	if err := <-errc; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first == nil {
		t.Fatal("no block received")
	}
	if got := uint64(first.Block.GasUsed); got != 0x5208 {
		t.Errorf("gasUsed = %d, want %d", got, 0x5208)
	}
	if got := len(first.Block.Transactions); got != 2 {
		t.Errorf("tx count = %d, want 2", got)
	}
	if got := len(first.Receipts); got != 2 {
		t.Errorf("receipt count = %d, want 2", got)
	}
	if got := uint64(first.Receipts[0].GasUsedForL1); got != 0x1388 {
		t.Errorf("gasUsedForL1 = %d, want %d", got, 0x1388)
	}
	if got := len(first.Logs); got != 1 {
		t.Errorf("log count = %d, want 1", got)
	}
	if first.FinalizedHead != 102 {
		t.Errorf("finalized head = %d, want 102", first.FinalizedHead)
	}
}

func TestRetryAfter429(t *testing.T) {
	t.Parallel()

	goldenNDJSON, err := os.ReadFile(goldenBatch)
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
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write(goldenNDJSON)
	}))
	defer srv.Close()

	src := sqd.New(srv.URL, discardLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	blocks, errc := src.Range(ctx, 100, 102, nil)
	nums, _, cerr := collect(t, blocks, errc)
	if cerr != nil {
		t.Fatalf("unexpected error: %v", cerr)
	}
	if diff := cmp.Diff([]uint64{100, 101, 102}, nums); diff != "" {
		t.Errorf("blocks mismatch (-want +got):\n%s", diff)
	}
	if calls.Load() < 2 {
		t.Errorf("expected at least 2 calls (retry), got %d", calls.Load())
	}
}

func TestForkReorg(t *testing.T) {
	t.Parallel()

	goldenNDJSON, err := os.ReadFile(goldenBatch)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w,
				`{"error":"fork","previousBlocks":[{"number":100,"hash":"0xaaa0"},{"number":99,"hash":"0x9fff"}]}`)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write(goldenNDJSON)
	}))
	defer srv.Close()

	src := sqd.New(srv.URL, discardLogger())
	blocks, errc := src.Range(context.Background(), 100, 102, nil)
	nums, _, cerr := collect(t, blocks, errc)
	if cerr != nil {
		t.Fatalf("unexpected error: %v", cerr)
	}
	// After the 409 fork, the source resumes from the common ancestor (99) and
	// streams the golden batch, so blocks 100..102 are produced.
	if diff := cmp.Diff([]uint64{100, 101, 102}, nums); diff != "" {
		t.Errorf("blocks mismatch (-want +got):\n%s", diff)
	}
	if calls.Load() < 2 {
		t.Errorf("expected 2 calls (fork then resume), got %d", calls.Load())
	}
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	// Handler that blocks until the request context is cancelled or released.
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)

	src := sqd.New(srv.URL, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())

	blocks, errc := src.Range(ctx, 100, 102, nil)
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	for range blocks { //nolint:revive
	}
	if err := <-errc; err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}

// FuzzStatusLineDetection ensures the batch/status discriminator never panics.
func FuzzStatusLineDetection(f *testing.F) {
	f.Add([]byte(`{"status":"finalizedHeadNumber","value":1}`))
	f.Add([]byte(`{"header":{"number":1}}`))
	f.Add([]byte(`not json`))
	f.Fuzz(func(t *testing.T, line []byte) {
		goldenNDJSON := append(bytes.TrimSpace(line), '\n')
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = w.Write(goldenNDJSON)
		}))
		defer srv.Close()

		src := sqd.New(srv.URL, discardLogger())
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		blocks, errc := src.Range(ctx, 1, 1, nil)
		for range blocks { //nolint:revive
		}
		<-errc // ignore error; we only assert no panic
	})
}
