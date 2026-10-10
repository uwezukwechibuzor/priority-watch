package api

import (
	"context"
	"fmt"
	"time"

	"github.com/uwezukwechibuzor/priority-watch/internal/models"
)

// ── Pricing history ────────────────────────────────────────────────────────────

// GetPricingHistory returns the most recent windowBlocks tipped blocks,
// ordered oldest-first (required by models.Eval / models.Recommend).
// If atBlock > 0, only blocks with block_number < atBlock are returned.
func (s *Store) GetPricingHistory(ctx context.Context, atBlock uint64, windowBlocks int) ([]models.BlockFeeRecord, error) {
	where := "tipped_tx_count > 0"
	args := []any{namedParam("limit", windowBlocks)}
	if atBlock > 0 {
		where += " AND block_number < @at_block"
		args = append(args, namedParam("at_block", atBlock))
	}

	sql := fmt.Sprintf(`
		SELECT
			block_number, block_timestamp, tipped_tx_count,
			top_fee_gwei, second_to_first_ratio, median_fee_gwei
		FROM marts.fct_blocks FINAL
		WHERE %s
		ORDER BY block_number DESC
		LIMIT @limit`, where)

	rows, err := s.query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("GetPricingHistory: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []models.BlockFeeRecord
	for rows.Next() {
		var r models.BlockFeeRecord
		if err := rows.Scan(
			&r.BlockNumber, &r.Timestamp, &r.TippedCount,
			&r.TopFeeGwei, &r.SecondToFirstRatio, &r.MedianFeeGwei,
		); err != nil {
			return nil, fmt.Errorf("GetPricingHistory scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Reverse so history is oldest-first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// ── Model evaluation ───────────────────────────────────────────────────────────

// ModelEvalRow is one row from marts.model_eval.
type ModelEvalRow struct {
	EvalDate     time.Time `json:"eval_date"`
	ModelName    string    `json:"model_name"`
	N            int32     `json:"n"`
	Confidence   float64   `json:"confidence"`
	WindowBlocks int32     `json:"window_blocks"`
	HitRate      float64   `json:"hit_rate"`
	AvgCostGwei  float64   `json:"avg_cost_gwei"`
	EvalBlocks   int32     `json:"eval_blocks"`
}

// GetModelEval returns the most recent eval rows for all models,
// filtered by n and confidence (0 = unfiltered).
func (s *Store) GetModelEval(ctx context.Context, n int, confidence float64) ([]ModelEvalRow, error) {
	where := "eval_date = (SELECT max(eval_date) FROM marts.model_eval FINAL)"
	args := []any{}
	if n > 0 {
		where += " AND n = @n"
		args = append(args, namedParam("n", int32(n))) //nolint:gosec
	}
	if confidence > 0 {
		where += " AND confidence = @confidence"
		args = append(args, namedParam("confidence", confidence))
	}

	sql := fmt.Sprintf(`
		SELECT eval_date, model_name, n, confidence, window_blocks,
			hit_rate, avg_cost_gwei, eval_blocks
		FROM marts.model_eval FINAL
		WHERE %s
		ORDER BY model_name, n, confidence`, where)

	rows, err := s.query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("GetModelEval: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []ModelEvalRow
	for rows.Next() {
		var r ModelEvalRow
		if err := rows.Scan(
			&r.EvalDate, &r.ModelName, &r.N, &r.Confidence, &r.WindowBlocks,
			&r.HitRate, &r.AvgCostGwei, &r.EvalBlocks,
		); err != nil {
			return nil, fmt.Errorf("GetModelEval scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Backtest runs ──────────────────────────────────────────────────────────────

// BacktestRunRow is a row from meta.backtest_runs.
type BacktestRunRow struct {
	RunID      string    `json:"run_id"`
	Status     string    `json:"status"`
	Strategy   string    `json:"strategy"`
	FromBlock  uint64    `json:"from_block"`
	ToBlock    uint64    `json:"to_block"`
	ConfigJSON string    `json:"config"`
	ResultJSON string    `json:"result,omitempty"`
	ErrorMsg   string    `json:"error_msg,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (s *Store) exec(ctx context.Context, sql string, args ...any) error {
	if s.conn == nil {
		return fmt.Errorf("clickhouse: connection not initialised")
	}
	return s.conn.Exec(ctx, sql, args...)
}

// SaveBacktestRun inserts a new backtest run with status 'pending'.
func (s *Store) SaveBacktestRun(ctx context.Context, run BacktestRunRow) error {
	return s.exec(ctx, `
		INSERT INTO meta.backtest_runs
			(run_id, status, strategy, from_block, to_block, config_json,
			 result_json, error_msg, created_at, updated_at, _version)
		VALUES
			(@run_id, @status, @strategy, @from_block, @to_block, @config_json,
			 '', '', @created_at, @updated_at, @version)`,
		namedParam("run_id", run.RunID),
		namedParam("status", run.Status),
		namedParam("strategy", run.Strategy),
		namedParam("from_block", run.FromBlock),
		namedParam("to_block", run.ToBlock),
		namedParam("config_json", run.ConfigJSON),
		namedParam("created_at", run.CreatedAt),
		namedParam("updated_at", run.UpdatedAt),
		namedParam("version", run.CreatedAt.UnixMilli()),
	)
}

// UpdateBacktestRun updates status, result_json and error_msg for a run.
func (s *Store) UpdateBacktestRun(ctx context.Context, runID, status, resultJSON, errorMsg string) error {
	now := time.Now().UTC()
	return s.exec(ctx, `
		INSERT INTO meta.backtest_runs
			(run_id, status, result_json, error_msg, updated_at, _version)
		VALUES
			(@run_id, @status, @result_json, @error_msg, @now, @version)`,
		namedParam("run_id", runID),
		namedParam("status", status),
		namedParam("result_json", resultJSON),
		namedParam("error_msg", errorMsg),
		namedParam("now", now),
		namedParam("version", now.UnixMilli()),
	)
}

// GetBacktestRun returns a run by ID.
func (s *Store) GetBacktestRun(ctx context.Context, runID string) (BacktestRunRow, error) {
	var r BacktestRunRow
	row := s.queryRow(ctx, `
		SELECT run_id, status, strategy, from_block, to_block,
			config_json, result_json, error_msg, created_at, updated_at
		FROM meta.backtest_runs FINAL
		WHERE run_id = @run_id`,
		namedParam("run_id", runID))
	if err := row.Scan(
		&r.RunID, &r.Status, &r.Strategy, &r.FromBlock, &r.ToBlock,
		&r.ConfigJSON, &r.ResultJSON, &r.ErrorMsg, &r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return BacktestRunRow{}, fmt.Errorf("GetBacktestRun(%s): %w", runID, err)
	}
	return r, nil
}
