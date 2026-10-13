package api

import (
	"fmt"
	"math/big"
	"time"
)

// Wei serialises a big.Int (UInt256 from ClickHouse) as a decimal string in JSON.
// huma v2 represents it as a string schema field.
type Wei struct{ v *big.Int }

func WeiFrom(b *big.Int) Wei {
	if b == nil {
		return Wei{new(big.Int)}
	}
	return Wei{new(big.Int).Set(b)}
}

func WeiFromUint64(n uint64) Wei { return Wei{new(big.Int).SetUint64(n)} }

func (w Wei) MarshalJSON() ([]byte, error) {
	if w.v == nil {
		return []byte(`"0"`), nil
	}
	return []byte(`"` + w.v.String() + `"`), nil
}

func (w *Wei) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	v := new(big.Int)
	if _, ok := v.SetString(s, 10); !ok {
		return fmt.Errorf("invalid wei value: %s", s)
	}
	w.v = v
	return nil
}

func (w Wei) BigInt() *big.Int {
	if w.v == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(w.v)
}

func (w Wei) String() string {
	if w.v == nil {
		return "0"
	}
	return w.v.String()
}

// rangeCutoff converts a range string to the earliest timestamp to include.
func rangeCutoff(r string) (time.Time, error) {
	now := time.Now().UTC()
	switch r {
	case "1h":
		return now.Add(-time.Hour), nil
	case "24h":
		return now.Add(-24 * time.Hour), nil
	case "7d":
		return now.Add(-7 * 24 * time.Hour), nil
	case "30d":
		return now.Add(-30 * 24 * time.Hour), nil
	case "all":
		return now.Add(-365 * 24 * time.Hour), nil
	default:
		return time.Time{}, fmt.Errorf("range must be one of: 1h,24h,7d,30d,all — got %q", r)
	}
}

// Pagination wraps cursor-based page information.
type Pagination struct {
	NextCursor *string `json:"next_cursor,omitempty" doc:"Opaque cursor for the next page; absent when no more pages."`
}

// HHIBand returns the concentration band for an HHI value.
func HHIBand(hhi float64) string {
	switch {
	case hhi < 1500:
		return "competitive"
	case hhi < 2500:
		return "moderate"
	default:
		return "high"
	}
}
