package timeboost

import (
	"context"
	"errors"
	"log/slog"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// ── stub implementations ──────────────────────────────────────────────────────

// stubBatch records Append calls and tracks Flush calls.
type stubBatch struct {
	rows    [][]any
	flushN  int
	failOn  int // if > 0, fail on the Nth Append (1-indexed)
	callCnt int
}

func (s *stubBatch) Append(_ context.Context, cols ...any) error {
	s.callCnt++
	if s.failOn > 0 && s.callCnt == s.failOn {
		return errors.New("injected append error")
	}
	row := make([]any, len(cols))
	copy(row, cols)
	s.rows = append(s.rows, row)
	return nil
}

func (s *stubBatch) Flush(_ context.Context) error {
	s.flushN++
	return nil
}

// stubRow implements Row with a pre-set scan result.
type stubRow struct {
	values []any
	err    error
}

func (r *stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		if i >= len(r.values) {
			break
		}
		switch v := d.(type) {
		case *string:
			*v = r.values[i].(string)
		}
	}
	return nil
}

// stubQuerier returns a pre-configured Row.
type stubQuerier struct {
	row Row
}

func (q *stubQuerier) QueryRow(_ context.Context, _ string, _ ...any) Row {
	return q.row
}

// ── helpers ────────────────────────────────────────────────────────────────────

func mustBigInt(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("invalid big.Int string: " + s)
	}
	return n
}

// bigIntComparer allows cmp.Diff to compare *big.Int values.
var bigIntComparer = cmp.Comparer(func(a, b *big.Int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Cmp(b) == 0
})

// ── tests ──────────────────────────────────────────────────────────────────────

func TestParseNDJSON_Bids(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   string
		want    []Bid
		wantErr bool
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name: "single bid",
			input: `{"round":1,"bidder":"0xABC","express_lane_controller":"0xDEF","amount_wei":"1000000000000000000","bid_timestamp":"2026-09-24T00:00:00Z"}` + "\n",
			want: []Bid{
				{
					Round:                 1,
					Bidder:                "0xabc",
					ExpressLaneController: "0xdef",
					AmountWei:             mustBigInt("1000000000000000000"),
					BidTimestamp:          ts,
				},
			},
		},
		{
			name: "multiple bids",
			input: `{"round":1,"bidder":"0xA","express_lane_controller":"0xB","amount_wei":"1000","bid_timestamp":"2026-09-24T00:00:00Z"}` + "\n" +
				`{"round":2,"bidder":"0xC","express_lane_controller":"0xD","amount_wei":"2000","bid_timestamp":"2026-09-24T00:00:00Z"}` + "\n",
			want: []Bid{
				{Round: 1, Bidder: "0xa", ExpressLaneController: "0xb", AmountWei: mustBigInt("1000"), BidTimestamp: ts},
				{Round: 2, Bidder: "0xc", ExpressLaneController: "0xd", AmountWei: mustBigInt("2000"), BidTimestamp: ts},
			},
		},
		{
			name:    "invalid JSON",
			input:   `{bad json}`,
			wantErr: true,
		},
		{
			name:    "invalid amount_wei",
			input:   `{"round":1,"bidder":"0xA","express_lane_controller":"0xB","amount_wei":"notanumber","bid_timestamp":"2026-09-24T00:00:00Z"}` + "\n",
			wantErr: true,
		},
		{
			name:    "invalid bid_timestamp",
			input:   `{"round":1,"bidder":"0xA","express_lane_controller":"0xB","amount_wei":"1000","bid_timestamp":"notadate"}` + "\n",
			wantErr: true,
		},
		{
			name:  "blank lines skipped",
			input: "\n\n",
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseNDJSONBids(strings.NewReader(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got, bigIntComparer, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("ParseNDJSONBids mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseNDJSON_Results(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 24, 0, 1, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   string
		want    []Result
		wantErr bool
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name: "single result",
			input: `{"round":1,"winner":"0xABC","express_lane_controller":"0xDEF","first_price_wei":"1000000000000000000","second_price_wei":"900000000000000000","round_start":"2026-09-24T00:00:00Z","round_end":"2026-09-24T00:01:00Z"}` + "\n",
			want: []Result{
				{
					Round:                 1,
					Winner:                "0xabc",
					ExpressLaneController: "0xdef",
					FirstPriceWei:         mustBigInt("1000000000000000000"),
					SecondPriceWei:        mustBigInt("900000000000000000"),
					RoundStart:            start,
					RoundEnd:              end,
				},
			},
		},
		{
			name:    "invalid JSON",
			input:   `{bad}`,
			wantErr: true,
		},
		{
			name:    "invalid first_price_wei",
			input:   `{"round":1,"winner":"0xA","express_lane_controller":"0xB","first_price_wei":"bad","second_price_wei":"0","round_start":"2026-09-24T00:00:00Z","round_end":"2026-09-24T00:01:00Z"}` + "\n",
			wantErr: true,
		},
		{
			name:    "invalid second_price_wei",
			input:   `{"round":1,"winner":"0xA","express_lane_controller":"0xB","first_price_wei":"1000","second_price_wei":"bad","round_start":"2026-09-24T00:00:00Z","round_end":"2026-09-24T00:01:00Z"}` + "\n",
			wantErr: true,
		},
		{
			name:    "invalid round_start",
			input:   `{"round":1,"winner":"0xA","express_lane_controller":"0xB","first_price_wei":"1000","second_price_wei":"500","round_start":"notadate","round_end":"2026-09-24T00:01:00Z"}` + "\n",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseNDJSONResults(strings.NewReader(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got, bigIntComparer, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("ParseNDJSONResults mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCrossCheck(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	matchResult := Result{
		Round:                 1,
		Winner:                "0xabc",
		ExpressLaneController: "0xdef",
		FirstPriceWei:         mustBigInt("1000"),
		SecondPriceWei:        mustBigInt("500"),
		RoundStart:            time.Now().UTC(),
		RoundEnd:              time.Now().UTC().Add(time.Minute),
	}
	mismatchResult := Result{
		Round:                 2,
		Winner:                "0x111",
		ExpressLaneController: "0x222",
		FirstPriceWei:         mustBigInt("2000"),
		SecondPriceWei:        mustBigInt("1000"),
		RoundStart:            time.Now().UTC(),
		RoundEnd:              time.Now().UTC().Add(time.Minute),
	}

	tests := []struct {
		name    string
		results []Result
		// stub returns this winner address (topic1) from raw.logs
		logWinner string
		logErr    error
	}{
		{
			name:      "match — no warning expected",
			results:   []Result{matchResult},
			logWinner: "0xabc",
		},
		{
			name:      "mismatch — warning logged",
			results:   []Result{mismatchResult},
			logWinner: "0x999",
		},
		{
			name:    "scan error — skip gracefully",
			results: []Result{matchResult},
			logErr:  errors.New("no rows"),
		},
		{
			name:    "empty results — no-op",
			results: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			bids := &stubBatch{}
			results := &stubBatch{}
			q := &stubQuerier{
				row: &stubRow{
					values: []any{tc.logWinner, "0xdata"},
					err:    tc.logErr,
				},
			}

			imp := NewImporter(Config{}, bids, results, q, logger)
			// crossCheck must not panic or return an error.
			imp.crossCheck(context.Background(), tc.results)
		})
	}
}
