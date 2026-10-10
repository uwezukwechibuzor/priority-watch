// Package tests contains integration and end-to-end tests.
// This file is the canonical reference for the table-driven test pattern
// used throughout the repository (see CLAUDE.md §8).
package tests_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// priorityFeePerGas computes the EIP-1559 effective priority fee.
// priority_fee_per_gas = min(maxPriorityFeePerGas, maxFeePerGas - baseFee)
func priorityFeePerGas(maxPriority, maxFee, baseFee uint64) (uint64, error) {
	if maxFee < baseFee {
		return 0, errors.New("maxFeePerGas below baseFee")
	}
	cap := maxFee - baseFee
	if maxPriority < cap {
		return maxPriority, nil
	}
	return cap, nil
}

func TestPriorityFeePerGas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		maxPriority uint64
		maxFee      uint64
		baseFee     uint64
		want        uint64
		wantErr     error
	}{
		{
			name:        "priority fee capped by fee gap",
			maxPriority: 10,
			maxFee:      15,
			baseFee:     8,
			want:        7, // min(10, 15-8=7) = 7
		},
		{
			name:        "priority fee below cap",
			maxPriority: 3,
			maxFee:      20,
			baseFee:     8,
			want:        3, // min(3, 12) = 3
		},
		{
			name:        "zero priority fee",
			maxPriority: 0,
			maxFee:      10,
			baseFee:     8,
			want:        0,
		},
		{
			name:        "maxFee below baseFee returns error",
			maxPriority: 5,
			maxFee:      7,
			baseFee:     10,
			wantErr:     errors.New("maxFeePerGas below baseFee"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := priorityFeePerGas(tc.maxPriority, tc.maxFee, tc.baseFee)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.wantErr)
				}
				if tc.wantErr.Error() != err.Error() {
					t.Fatalf("error mismatch: want %q, got %q", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("priorityFeePerGas() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
