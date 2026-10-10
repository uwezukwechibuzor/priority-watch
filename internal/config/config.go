// Package config loads all runtime configuration from environment variables.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for Priority Watch binaries.
// Values are loaded from environment variables; see .env.example for the full list.
type Config struct {
	// Chain ─────────────────────────────────────────────────────────────────
	ArbitrumRPCURL          string   `env:"ARBITRUM_RPC_URL"`
	ArbitrumWSURL           string   `env:"ARBITRUM_WS_URL"`
	ArbitrumRPCFallbackURLs []string `env:"ARBITRUM_RPC_FALLBACK_URLS"    envSeparator:","`
	ArbitrumRPCRPS          float64  `env:"ARBITRUM_RPC_RPS"              envDefault:"25"`
	ArbitrumChainID         uint64   `env:"ARBITRUM_CHAIN_ID"             envDefault:"42161"`

	// Follower ────────────────────────────────────────────────────────────────
	// GapThreshold is the number of missed blocks that triggers a backfill
	// catch-up instead of block-by-block ingestion on follower start.
	GapThreshold uint64 `env:"FOLLOWER_GAP_THRESHOLD" envDefault:"1000"`

	// ClickHouse ─────────────────────────────────────────────────────────────
	ClickHouseAddr     string `env:"CLICKHOUSE_ADDR"     envDefault:"localhost:9000"`
	ClickHouseUser     string `env:"CLICKHOUSE_USER"     envDefault:"default"`
	ClickHousePassword string `env:"CLICKHOUSE_PASSWORD"`
	ClickHouseDatabase string `env:"CLICKHOUSE_DATABASE" envDefault:"raw"`

	// Read-only ClickHouse credentials (API uses these; ingester uses admin creds above)
	ClickHouseROUser     string `env:"CLICKHOUSE_RO_USER"     envDefault:"pw_reader"`
	ClickHouseROPassword string `env:"CLICKHOUSE_RO_PASSWORD"`

	// Ingester ────────────────────────────────────────────────────────────────
	BatchSize          int           `env:"BATCH_SIZE"           envDefault:"1000"`
	BatchFlushInterval time.Duration `env:"BATCH_FLUSH_INTERVAL" envDefault:"10s"`

	// API ─────────────────────────────────────────────────────────────────────
	APIAddr       string `env:"API_ADDR"        envDefault:":8080"`
	APICORSOrigin string `env:"API_CORS_ORIGIN" envDefault:"*"`

	// PGA chain parameters ───────────────────────────────────────────────────
	PGAActivationBlock uint64 `env:"PGA_ACTIVATION_BLOCK" envDefault:"0"`

	// ClickHouse HTTP endpoint (used by datasets streaming, separate from native port)
	ClickHouseHTTPAddr string `env:"CLICKHOUSE_HTTP_ADDR" envDefault:"http://localhost:8123"`

	// Observability ───────────────────────────────────────────────────────────
	MetricsAddr string `env:"METRICS_ADDR" envDefault:":9091"`
	LogLevel    string `env:"LOG_LEVEL"    envDefault:"info"`
}

// Load parses environment variables into Config and validates invariants.
// Returns an error if any required variable is missing or chain ID ≠ 42161.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if cfg.ArbitrumRPCURL == "" {
		return nil, fmt.Errorf("config: ARBITRUM_RPC_URL is required")
	}
	if cfg.ArbitrumChainID != 42161 {
		return nil, fmt.Errorf("config: ARBITRUM_CHAIN_ID must be 42161 (Arbitrum One), got %d", cfg.ArbitrumChainID)
	}
	if cfg.ArbitrumRPCRPS <= 0 {
		return nil, fmt.Errorf("config: ARBITRUM_RPC_RPS must be positive, got %g", cfg.ArbitrumRPCRPS)
	}
	if cfg.BatchSize <= 0 {
		return nil, fmt.Errorf("config: BATCH_SIZE must be positive, got %d", cfg.BatchSize)
	}
	return cfg, nil
}

// LoadAPI parses environment variables for the API binary.
// Does not require ARBITRUM_RPC_URL (the API does not talk to the chain).
func LoadAPI() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if cfg.ArbitrumChainID != 42161 {
		return nil, fmt.Errorf("config: ARBITRUM_CHAIN_ID must be 42161, got %d", cfg.ArbitrumChainID)
	}
	return cfg, nil
}
