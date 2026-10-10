package config_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/uwezukwechibuzor/priority-watch/internal/config"
)

// Note: tests that use t.Setenv cannot call t.Parallel — Go's testing package
// enforces this because Setenv modifies the process-wide environment.

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "valid minimal config",
			env:     map[string]string{"ARBITRUM_RPC_URL": "http://localhost:8545"},
			wantErr: "",
		},
		{
			name:    "missing required ARBITRUM_RPC_URL",
			env:     map[string]string{},
			wantErr: "ARBITRUM_RPC_URL",
		},
		{
			name:    "wrong chain ID",
			env:     map[string]string{"ARBITRUM_RPC_URL": "http://localhost:8545", "ARBITRUM_CHAIN_ID": "1"},
			wantErr: "42161",
		},
		{
			name:    "non-positive RPS",
			env:     map[string]string{"ARBITRUM_RPC_URL": "http://localhost:8545", "ARBITRUM_RPC_RPS": "0"},
			wantErr: "RPS",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			// Ensure ARBITRUM_RPC_URL is unset when not in the map.
			if _, ok := tc.env["ARBITRUM_RPC_URL"]; !ok {
				t.Setenv("ARBITRUM_RPC_URL", "")
			}

			cfg, err := config.Load()

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
			if cfg == nil {
				t.Fatal("cfg is nil")
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ARBITRUM_RPC_URL", "http://localhost:8545")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]any{
		"ClickHouseAddr":     "localhost:9000",
		"ClickHouseUser":     "default",
		"ClickHouseDatabase": "raw",
		"BatchSize":          1000,
		"APIAddr":            ":8080",
		"MetricsAddr":        ":9091",
		"LogLevel":           "info",
		"ArbitrumChainID":    uint64(42161),
	}

	got := map[string]any{
		"ClickHouseAddr":     cfg.ClickHouseAddr,
		"ClickHouseUser":     cfg.ClickHouseUser,
		"ClickHouseDatabase": cfg.ClickHouseDatabase,
		"BatchSize":          cfg.BatchSize,
		"APIAddr":            cfg.APIAddr,
		"MetricsAddr":        cfg.MetricsAddr,
		"LogLevel":           cfg.LogLevel,
		"ArbitrumChainID":    cfg.ArbitrumChainID,
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("defaults mismatch (-want +got):\n%s", diff)
	}
}
