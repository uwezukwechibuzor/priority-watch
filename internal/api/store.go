package api

import (
	"context"
	"fmt"
	"math/big"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Store executes read-only ClickHouse queries for the API.
// All methods use named parameters — no SQL is constructed from user input.
type Store struct {
	conn driver.Conn
}

func newStore(conn driver.Conn) *Store { return &Store{conn: conn} }

// ── helpers ────────────────────────────────────────────────────────────────────

func (s *Store) queryRow(ctx context.Context, sql string, args ...any) driver.Row {
	if s.conn == nil {
		return &nilRow{}
	}
	return s.conn.QueryRow(ctx, sql, args...)
}

func (s *Store) query(ctx context.Context, sql string, args ...any) (driver.Rows, error) {
	if s.conn == nil {
		return nil, fmt.Errorf("clickhouse: connection not initialised")
	}
	return s.conn.Query(ctx, sql, args...)
}

// nilRow satisfies driver.Row and always returns an error on Scan.
type nilRow struct{}

func (*nilRow) Scan(...any) error { return fmt.Errorf("clickhouse: connection not initialised") }

func (*nilRow) ScanStruct(any) error { return fmt.Errorf("clickhouse: connection not initialised") }

func (*nilRow) Err() error { return fmt.Errorf("clickhouse: connection not initialised") }

// ── Block types ────────────────────────────────────────────────────────────────

type BlockRow struct {
	BlockNumber      uint64    `json:"block_number"`
	BlockTimestamp   time.Time `json:"block_timestamp"`
	Era              string    `json:"era"`
	TxCount          uint64    `json:"tx_count"`
	TippedTxCount    uint64    `json:"tipped_tx_count"`
	TipRevenueWei    string    `json:"tip_revenue_wei"`
	TipRevenueEth    float64   `json:"tip_revenue_eth"`
	TipRevenueUsd    float64   `json:"tip_revenue_usd"`
	EthPriceUsd      float64   `json:"eth_price_usd"`
	TopFeeGwei       float64   `json:"top_fee_gwei"`
	TopTipUsd        float64   `json:"top_tip_usd"`
	MedianFeeGwei    float64   `json:"median_fee_gwei"`
	TopPayerAddress  string    `json:"top_payer_address"`
	TopPayerName     string    `json:"top_payer_name"`
	TopPayerColorIdx uint8     `json:"top_payer_color_index"`
	RevertCount      uint64    `json:"revert_count"`
	SecondToFirst    float64   `json:"second_to_first_ratio"`
	BlockType        string    `json:"block_type"`
	OrderingScore    float64   `json:"ordering_score"`
	DistinctTippers  uint64    `json:"distinct_tippers"`
	BaseFeeWei       string    `json:"base_fee_per_gas_wei"`
	BaseFeeGwei      float64   `json:"base_fee_per_gas_gwei"`
	GasUsed          uint64    `json:"gas_used"`
	GasLimit         uint64    `json:"gas_limit"`
	GasUsedPct       float64   `json:"gas_used_pct"`
}

// GetBlocks returns a page of blocks, newest first, with optional filters.
func (s *Store) GetBlocks(ctx context.Context, p BlocksParams) ([]BlockRow, error) {
	cutoff, err := rangeCutoff(p.Range)
	if err != nil {
		return nil, err
	}

	where := "block_timestamp >= @cutoff"
	args := []any{namedParam("cutoff", cutoff)}

	if p.Day != "" {
		where = "toDate(block_timestamp) = @day"
		args = []any{namedParam("day", p.Day)}
	}
	if p.ContestedOnly {
		where += " AND block_type = 'contested'"
	}
	if p.MinFeeGwei > 0 {
		where += " AND top_fee_gwei >= @min_fee"
		args = append(args, namedParam("min_fee", p.MinFeeGwei))
	}
	if p.Search != "" {
		where += " AND (toString(block_number) LIKE @search OR top_payer_address LIKE @search OR top_payer_name LIKE @search)"
		args = append(args, namedParam("search", "%"+p.Search+"%"))
	}
	if p.Entity != "" {
		where += " AND top_payer_name = @entity"
		args = append(args, namedParam("entity", p.Entity))
	}
	if p.FromBlock > 0 {
		where += " AND block_number >= @from_block"
		args = append(args, namedParam("from_block", p.FromBlock))
	}
	if p.ToBlock > 0 {
		where += " AND block_number <= @to_block"
		args = append(args, namedParam("to_block", p.ToBlock))
	}
	if p.CursorBlock != nil {
		where += " AND block_number < @cursor_block"
		args = append(args, namedParam("cursor_block", *p.CursorBlock))
	}

	sql := fmt.Sprintf(`
		SELECT
			block_number, block_timestamp, era, tx_count, tipped_tx_count,
			toString(tip_revenue_wei) AS tip_revenue_wei,
			tip_revenue_eth, tip_revenue_usd, eth_price_usd,
			top_fee_gwei, top_tip_usd, median_fee_gwei,
			top_payer_address, top_payer_name, top_payer_color_index,
			revert_count, second_to_first_ratio, block_type, ordering_score,
			distinct_tippers,
			toString(base_fee_per_gas) AS base_fee_per_gas_wei,
			base_fee_per_gas_gwei, gas_used, gas_limit,
				coalesce(gas_used_pct, 0.0) AS gas_used_pct
		FROM marts.fct_blocks FINAL
		WHERE %s
		ORDER BY block_number DESC
		LIMIT @limit`, where)
	args = append(args, namedParam("limit", p.Limit))

	rows, err := s.query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("GetBlocks: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []BlockRow
	for rows.Next() {
		var r BlockRow
		if err := rows.Scan(
			&r.BlockNumber, &r.BlockTimestamp, &r.Era, &r.TxCount, &r.TippedTxCount,
			&r.TipRevenueWei, &r.TipRevenueEth, &r.TipRevenueUsd, &r.EthPriceUsd,
			&r.TopFeeGwei, &r.TopTipUsd, &r.MedianFeeGwei,
			&r.TopPayerAddress, &r.TopPayerName, &r.TopPayerColorIdx,
			&r.RevertCount, &r.SecondToFirst, &r.BlockType, &r.OrderingScore,
			&r.DistinctTippers, &r.BaseFeeWei, &r.BaseFeeGwei,
			&r.GasUsed, &r.GasLimit, &r.GasUsedPct,
		); err != nil {
			return nil, fmt.Errorf("GetBlocks scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BlockKPIs aggregates summary statistics for a range.
type BlockKPIs struct {
	BlockCount     uint64  `json:"block_count"`
	TipRevenueWei  string  `json:"tip_revenue_wei"`
	TipRevenueEth  float64 `json:"tip_revenue_eth"`
	TipRevenueUsd  float64 `json:"tip_revenue_usd"`
	AvgTipEth      float64 `json:"avg_tip_per_block_eth"`
	ContestedShare float64 `json:"contested_share"`
	TopPayerName   string  `json:"top_payer_name"`
	TopPayerShare  float64 `json:"top_payer_share"`
	HHI            float64 `json:"hhi"`
	HHIBand        string  `json:"hhi_band"`
	AvgTopFeeGwei  float64 `json:"avg_top_fee_gwei"`
}

func (s *Store) GetBlockKPIs(ctx context.Context, rangeStr string, day *string) (BlockKPIs, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return BlockKPIs{}, err
	}
	where := "hour >= @cutoff"
	args := []any{namedParam("cutoff", cutoff)}
	if day != nil {
		where = "toDate(hour) = @day"
		args = []any{namedParam("day", *day)}
	}

	sql := fmt.Sprintf(`
		SELECT
			tot_blocks                                                  AS block_count,
			toString(tot_tip_wei)                                       AS tip_revenue_wei,
			tot_tip_eth                                                 AS tip_revenue_eth,
			tot_tip_usd                                                 AS tip_revenue_usd,
			if(tot_blocks > 0, tot_tip_eth / tot_blocks, 0)            AS avg_tip_eth,
			if(tot_blocks > 0,
				toFloat64(tot_contested) / toFloat64(tot_blocks), 0)   AS contested_share,
			''                                                          AS top_payer_name,
			avg_top_payer_share                                         AS top_payer_share,
			avg_hhi_val                                                 AS avg_hhi,
			avg_top_fee                                                 AS avg_top_fee_gwei
		FROM (
			SELECT
				sum(block_count)                AS tot_blocks,
				sum(tip_revenue_wei)            AS tot_tip_wei,
				sum(tip_revenue_eth)            AS tot_tip_eth,
				sum(tip_revenue_usd)            AS tot_tip_usd,
				sum(contested_block_count)      AS tot_contested,
				avg(top_payer_share)            AS avg_top_payer_share,
				avg(toFloat64(hhi))             AS avg_hhi_val,
				avg(top_fee_gwei)               AS avg_top_fee
			FROM marts.agg_blocks_hourly FINAL
			WHERE %s
		)`, where)

	var (
		kpi BlockKPIs
		hhi float64
	)
	row := s.queryRow(ctx, sql, args...)
	if err := row.Scan(
		&kpi.BlockCount, &kpi.TipRevenueWei, &kpi.TipRevenueEth, &kpi.TipRevenueUsd,
		&kpi.AvgTipEth, &kpi.ContestedShare, &kpi.TopPayerName, &kpi.TopPayerShare,
		&hhi, &kpi.AvgTopFeeGwei,
	); err != nil {
		return BlockKPIs{}, fmt.Errorf("GetBlockKPIs: %w", err)
	}
	kpi.HHI = hhi
	kpi.HHIBand = HHIBand(hhi)
	return kpi, nil
}

// SeriesPoint is one time-bucket in a chart series.
type SeriesPoint struct {
	Ts             time.Time `json:"ts"`
	TipRevenueEth  float64   `json:"tip_revenue_eth"`
	TipRevenueUsd  float64   `json:"tip_revenue_usd"`
	TopFeeGwei     float64   `json:"top_fee_gwei"`
	BlockCount     uint64    `json:"block_count"`
	TippedTxCount  uint64    `json:"tipped_tx_count"`
	ContestedShare float64   `json:"contested_share"`
	HHI            uint32    `json:"hhi"`
}

func (s *Store) GetBlockSeries(ctx context.Context, rangeStr string) ([]SeriesPoint, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}

	// Use minutely for 1h, hourly for everything else
	table := "marts.agg_blocks_hourly FINAL"
	tsCol := "hour"
	if rangeStr == "1h" {
		table = "marts.agg_blocks_minutely FINAL"
		tsCol = "minute"
	}

	sql := fmt.Sprintf(`
		SELECT %s AS ts,
			tip_revenue_eth, tip_revenue_usd, top_fee_gwei,
			block_count, tipped_tx_count, contested_share, hhi
		FROM %s
		WHERE ts >= @cutoff
		ORDER BY ts ASC`, tsCol, table)

	rows, err := s.query(ctx, sql, namedParam("cutoff", cutoff))
	if err != nil {
		return nil, fmt.Errorf("GetBlockSeries: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []SeriesPoint
	for rows.Next() {
		var p SeriesPoint
		if err := rows.Scan(
			&p.Ts, &p.TipRevenueEth, &p.TipRevenueUsd, &p.TopFeeGwei,
			&p.BlockCount, &p.TippedTxCount, &p.ContestedShare, &p.HHI,
		); err != nil {
			return nil, fmt.Errorf("GetBlockSeries scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PayerShareRow is one entity row in the payer-share panel.
type PayerShareRow struct {
	EntityName string  `json:"entity_name"`
	EntityType string  `json:"entity_type"`
	ColorIndex uint8   `json:"color_index"`
	TipEth     float64 `json:"tip_eth"`
	TipUsd     float64 `json:"tip_usd"`
	SharePct   float64 `json:"share_pct"`
	TxCount    uint64  `json:"tx_count"`
	RevertRate float64 `json:"revert_rate"`
	AvgTipRank float64 `json:"avg_tip_rank"`
}

func (s *Store) GetPayerShare(ctx context.Context, rangeStr string) ([]PayerShareRow, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}

	sql := `
		WITH totals AS (
			SELECT sum(tip_eth) AS total_eth
			FROM marts.agg_payer_share_daily FINAL
			WHERE date >= toDate(@cutoff)
		),
		agg AS (
			SELECT
				entity_name, entity_type, color_index,
				sum(tip_eth)      AS tip_eth,
				sum(tip_usd)      AS tip_usd,
				sum(tx_count)     AS tx_count,
				avg(revert_rate)  AS revert_rate,
				avg(avg_tip_rank) AS avg_tip_rank
			FROM marts.agg_payer_share_daily FINAL
			WHERE date >= toDate(@cutoff)
			GROUP BY entity_name, entity_type, color_index
		)
		SELECT
			a.entity_name, a.entity_type, a.color_index,
			a.tip_eth,
			a.tip_usd,
			if(t.total_eth > 0, a.tip_eth / t.total_eth, 0) AS share_pct,
			a.tx_count,
			a.revert_rate,
			a.avg_tip_rank
		FROM agg a
		CROSS JOIN totals t
		ORDER BY a.tip_eth DESC
		LIMIT 10`

	rows, err := s.query(ctx, sql, namedParam("cutoff", cutoff))
	if err != nil {
		return nil, fmt.Errorf("GetPayerShare: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []PayerShareRow
	for rows.Next() {
		var r PayerShareRow
		if err := rows.Scan(
			&r.EntityName, &r.EntityType, &r.ColorIndex,
			&r.TipEth, &r.TipUsd, &r.SharePct,
			&r.TxCount, &r.RevertRate, &r.AvgTipRank,
		); err != nil {
			return nil, fmt.Errorf("GetPayerShare scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// TippedHistRow is one bin of the tipped-tx-count histogram.
type TippedHistRow struct {
	Bin        string `json:"bin"`
	BlockCount uint64 `json:"block_count"`
}

func (s *Store) GetTippedHistogram(ctx context.Context, rangeStr string) ([]TippedHistRow, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}
	sql := `
		SELECT tipped_tx_bin, sum(block_count) AS block_count
		FROM marts.agg_tipped_tx_hist_daily FINAL
		WHERE date >= toDate(@cutoff)
		GROUP BY tipped_tx_bin
		ORDER BY tipped_tx_bin`

	rows, err := s.query(ctx, sql, namedParam("cutoff", cutoff))
	if err != nil {
		return nil, fmt.Errorf("GetTippedHistogram: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []TippedHistRow
	for rows.Next() {
		var r TippedHistRow
		if err := rows.Scan(&r.Bin, &r.BlockCount); err != nil {
			return nil, fmt.Errorf("GetTippedHistogram scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetBlock returns one block by number.
func (s *Store) GetBlock(ctx context.Context, blockNumber uint64) (*BlockRow, error) {
	sql := `
		SELECT
			block_number, block_timestamp, era, tx_count, tipped_tx_count,
			toString(tip_revenue_wei) AS tip_revenue_wei,
			tip_revenue_eth, tip_revenue_usd, eth_price_usd,
			top_fee_gwei, top_tip_usd, median_fee_gwei,
			top_payer_address, top_payer_name, top_payer_color_index,
			revert_count, second_to_first_ratio, block_type, ordering_score,
			distinct_tippers,
			toString(base_fee_per_gas) AS base_fee_per_gas_wei,
			base_fee_per_gas_gwei, gas_used, gas_limit,
				coalesce(gas_used_pct, 0.0) AS gas_used_pct
		FROM marts.fct_blocks FINAL
		WHERE block_number = @block_number`

	var r BlockRow
	row := s.queryRow(ctx, sql, namedParam("block_number", blockNumber))
	if err := row.Scan(
		&r.BlockNumber, &r.BlockTimestamp, &r.Era, &r.TxCount, &r.TippedTxCount,
		&r.TipRevenueWei, &r.TipRevenueEth, &r.TipRevenueUsd, &r.EthPriceUsd,
		&r.TopFeeGwei, &r.TopTipUsd, &r.MedianFeeGwei,
		&r.TopPayerAddress, &r.TopPayerName, &r.TopPayerColorIdx,
		&r.RevertCount, &r.SecondToFirst, &r.BlockType, &r.OrderingScore,
		&r.DistinctTippers, &r.BaseFeeWei, &r.BaseFeeGwei,
		&r.GasUsed, &r.GasLimit, &r.GasUsedPct,
	); err != nil {
		return nil, fmt.Errorf("GetBlock(%d): %w", blockNumber, err)
	}
	return &r, nil
}

// TxRow is one row from fct_transactions.
type TxRow struct {
	BlockNumber      uint64  `json:"block_number"`
	TxIndex          uint32  `json:"tx_index"`
	TxHash           string  `json:"tx_hash"`
	TxType           uint8   `json:"tx_type"`
	FromAddress      string  `json:"from_address"`
	ToAddress        string  `json:"to_address"`
	Status           uint8   `json:"status"`
	GasUsed          uint64  `json:"gas_used"`
	PriorityFeeGwei  float64 `json:"priority_fee_per_gas_gwei"`
	TipPaidEth       float64 `json:"tip_paid_eth"`
	TipPaidUsd       float64 `json:"tip_paid_usd"`
	TipPaidWei       string  `json:"tip_paid_wei"`
	TipRank          uint32  `json:"tip_rank"`
	TxClass          string  `json:"tx_class"`
	EntityName         string `json:"entity_name"`
	EntityType         string `json:"entity_type"`
	ColorIndex         uint8  `json:"color_index"`
	EntityAddressLabel string `json:"entity_address_label"`
	IsRevertedTipped   uint8  `json:"is_reverted_tipped"`
}

func (s *Store) GetBlockTxs(ctx context.Context, blockNumber uint64, cursor *uint32, limit int) ([]TxRow, error) {
	where := "block_number = @block_number"
	args := []any{namedParam("block_number", blockNumber)}
	if cursor != nil {
		where += " AND tx_index > @cursor"
		args = append(args, namedParam("cursor", *cursor))
	}
	args = append(args, namedParam("limit", limit))

	sql := fmt.Sprintf(`
		SELECT
			block_number, tx_index, tx_hash, tx_type, from_address, to_address,
			status, gas_used, priority_fee_per_gas_gwei,
			tip_paid_eth, tip_paid_usd, toString(tip_paid_wei) AS tip_paid_wei,
			tip_rank, tx_class, entity_name, entity_type, color_index,
			entity_address_label, is_reverted_tipped
		FROM marts.fct_transactions FINAL
		WHERE %s
		ORDER BY tx_index ASC
		LIMIT @limit`, where)

	rows, err := s.query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("GetBlockTxs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []TxRow
	for rows.Next() {
		var r TxRow
		if err := rows.Scan(
			&r.BlockNumber, &r.TxIndex, &r.TxHash, &r.TxType, &r.FromAddress, &r.ToAddress,
			&r.Status, &r.GasUsed, &r.PriorityFeeGwei,
			&r.TipPaidEth, &r.TipPaidUsd, &r.TipPaidWei,
			&r.TipRank, &r.TxClass, &r.EntityName, &r.EntityType, &r.ColorIndex,
			&r.EntityAddressLabel, &r.IsRevertedTipped,
		); err != nil {
			return nil, fmt.Errorf("GetBlockTxs scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Health types ───────────────────────────────────────────────────────────────

type HealthDailyRow struct {
	Date              time.Time `json:"date"`
	Era               string    `json:"era"`
	TipRevenueEth     float64   `json:"tip_revenue_eth"`
	TipRevenueUsd     float64   `json:"tip_revenue_usd"`
	TipRevenueWei     string    `json:"tip_revenue_wei"`
	TxCount           uint64    `json:"tx_count"`
	TippedTxCount     uint64    `json:"tipped_tx_count"`
	P10FeeGwei        float64   `json:"p10_fee_gwei"`
	P50FeeGwei        float64   `json:"p50_fee_gwei"`
	P90FeeGwei        float64   `json:"p90_fee_gwei"`
	BlockCount        uint64    `json:"block_count"`
	BlocksExpected    uint32    `json:"blocks_expected"`
	BlocksPctExpected float64   `json:"blocks_pct_of_expected"`
	ContestedShare    float64   `json:"contested_share"`
	HHI               uint32    `json:"hhi"`
	HHIBand           string    `json:"hhi_band"`
	TopPayerShare     float64   `json:"top_payer_share"`
	RevertRateTipped  float64   `json:"revert_rate_tipped"`
	MedianUserCostUsd float64   `json:"median_user_cost_usd"`
}

func (s *Store) GetHealthDaily(ctx context.Context, rangeStr string) ([]HealthDailyRow, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}

	rows, err := s.query(ctx, `
		SELECT
			date, era, tip_revenue_eth, tip_revenue_usd,
			toString(tip_revenue_wei) AS tip_revenue_wei,
			tx_count, tipped_tx_count,
			p10_fee_gwei, p50_fee_gwei, p90_fee_gwei,
			block_count, expected_block_count, blocks_pct_of_expected,
			contested_share, hhi, hhi_band, top_payer_share,
			revert_rate_tipped,
			if(isNaN(median_user_cost_usd) OR isInfinite(median_user_cost_usd), 0.0, median_user_cost_usd) AS median_user_cost_usd
		FROM marts.agg_health_daily FINAL
		WHERE date >= toDate(@cutoff)
		ORDER BY date DESC`,
		namedParam("cutoff", cutoff))
	if err != nil {
		return nil, fmt.Errorf("GetHealthDaily: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []HealthDailyRow
	for rows.Next() {
		var r HealthDailyRow
		if err := rows.Scan(
			&r.Date, &r.Era, &r.TipRevenueEth, &r.TipRevenueUsd, &r.TipRevenueWei,
			&r.TxCount, &r.TippedTxCount,
			&r.P10FeeGwei, &r.P50FeeGwei, &r.P90FeeGwei,
			&r.BlockCount, &r.BlocksExpected, &r.BlocksPctExpected,
			&r.ContestedShare, &r.HHI, &r.HHIBand, &r.TopPayerShare,
			&r.RevertRateTipped, &r.MedianUserCostUsd,
		); err != nil {
			return nil, fmt.Errorf("GetHealthDaily scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// IngesterStatusRow is one row from meta.source_status.
type IngesterStatusRow struct {
	Source          string    `json:"source"`
	Status          string    `json:"status"`
	LagSeconds      float64   `json:"lag_seconds"`
	LatestPosition  string    `json:"latest_position"`
	LatestTimestamp time.Time `json:"latest_timestamp"`
	LastUpdatedAt   time.Time `json:"last_updated_at"`
}

func (s *Store) GetIngesterStatus(ctx context.Context) ([]IngesterStatusRow, error) {
	rows, err := s.query(ctx, `
		SELECT
			source,
			if(dateDiff('second', updated_at, now()) > 120, 'lagging',
				if(dateDiff('second', updated_at, now()) > 300, 'down', 'live')) AS status,
			dateDiff('second', latest_timestamp, now()) AS lag_seconds,
			latest_position, latest_timestamp, updated_at
		FROM meta.source_status FINAL
		ORDER BY source`)
	if err != nil {
		return nil, fmt.Errorf("GetIngesterStatus: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []IngesterStatusRow
	for rows.Next() {
		var r IngesterStatusRow
		if err := rows.Scan(
			&r.Source, &r.Status, &r.LagSeconds,
			&r.LatestPosition, &r.LatestTimestamp, &r.LastUpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("GetIngesterStatus scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Treasury types ─────────────────────────────────────────────────────────────

type TreasuryDailyRow struct {
	Date              time.Time `json:"date"`
	Era               string    `json:"era"`
	TotalTipWei       string    `json:"total_tip_wei"`
	TotalTipEth       float64   `json:"total_tip_eth"`
	TotalTipUsd       float64   `json:"total_tip_usd"`
	TreasuryWei       string    `json:"treasury_wei"`
	DeveloperGuildWei string    `json:"developer_guild_wei"`
	TxCount           uint64    `json:"tx_count"`
	TippedTxCount     uint64    `json:"tipped_tx_count"`
	Status            string    `json:"status"`
}

func (s *Store) GetTreasuryDaily(ctx context.Context, rangeStr string) ([]TreasuryDailyRow, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}

	rows, err := s.query(ctx, `
		SELECT
			date, era,
			toString(total_tip_wei) AS total_tip_wei,
			total_tip_eth, total_tip_usd,
			toString(treasury_wei) AS treasury_wei,
			toString(developer_guild_wei) AS developer_guild_wei,
			tx_count, tipped_tx_count, status
		FROM marts.fct_treasury_daily FINAL
		WHERE date >= toDate(@cutoff)
		ORDER BY date DESC`,
		namedParam("cutoff", cutoff))
	if err != nil {
		return nil, fmt.Errorf("GetTreasuryDaily: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []TreasuryDailyRow
	for rows.Next() {
		var r TreasuryDailyRow
		if err := rows.Scan(
			&r.Date, &r.Era, &r.TotalTipWei, &r.TotalTipEth, &r.TotalTipUsd,
			&r.TreasuryWei, &r.DeveloperGuildWei,
			&r.TxCount, &r.TippedTxCount, &r.Status,
		); err != nil {
			return nil, fmt.Errorf("GetTreasuryDaily scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type TreasurySummary struct {
	TotalTipEth        float64 `json:"total_tip_eth"`
	TotalTipUsd        float64 `json:"total_tip_usd"`
	TotalTipWei        string  `json:"total_tip_wei"`
	TreasuryEth        float64 `json:"treasury_eth"`
	DeveloperGuildEth  float64 `json:"developer_guild_eth"`
	AvgDailyTipEth     float64 `json:"avg_daily_tip_eth"`
	ProjectedAnnualEth float64 `json:"projected_annual_eth"`
	DayCount           uint64  `json:"day_count"`
}

func (s *Store) GetTreasurySummary(ctx context.Context) (TreasurySummary, error) {
	var s2 TreasurySummary
	var wei big.Int
	row := s.queryRow(ctx, `
		SELECT
			sum(total_tip_eth) AS total_tip_eth,
			sum(total_tip_usd) AS total_tip_usd,
			toString(sum(total_tip_wei)) AS total_tip_wei,
			sum(total_tip_eth) * 0.97 AS treasury_eth,
			sum(total_tip_eth) * 0.03 AS developer_guild_eth,
			if(count() > 0, sum(total_tip_eth)/count(), 0) AS avg_daily,
			if(count() > 0, sum(total_tip_eth)/count()*365, 0) AS projected_annual,
			count() AS day_count
		FROM marts.fct_treasury_daily FINAL`)
	if err := row.Scan(
		&s2.TotalTipEth, &s2.TotalTipUsd, &wei,
		&s2.TreasuryEth, &s2.DeveloperGuildEth,
		&s2.AvgDailyTipEth, &s2.ProjectedAnnualEth, &s2.DayCount,
	); err != nil {
		return TreasurySummary{}, fmt.Errorf("GetTreasurySummary: %w", err)
	}
	s2.TotalTipWei = wei.String()
	return s2, nil
}

// ── Compare types ──────────────────────────────────────────────────────────────

type EraComparisonRow struct {
	Era                   string  `json:"era"`
	WindowDays            int32   `json:"window_days"`
	WindowTipEth          float64 `json:"window_tip_eth"`
	AvgDailyRevenueEth    float64 `json:"avg_daily_revenue_eth"`
	AvgContestedShare     float64 `json:"avg_contested_share"`
	AvgHHI                float64 `json:"avg_hhi"`
	AvgTopPayerShare      float64 `json:"avg_top_payer_share"`
	AvgRevertRate         float64 `json:"avg_revert_rate"`
	AvgMedianFeeGwei      float64 `json:"avg_median_fee_gwei"`
	CaptureRatePct        float64 `json:"capture_rate_pct"`
	EntropyCaptureRatePct float64 `json:"entropy_capture_rate_pct"`
}

func (s *Store) GetEraComparison(ctx context.Context, windowDays int) ([]EraComparisonRow, error) {
	rows, err := s.query(ctx, `
		SELECT
			era, window_days_var AS window_days, window_tip_eth,
			avg_daily_revenue_eth, avg_contested_share, avg_hhi,
			avg_top_payer_share, avg_revert_rate, avg_median_fee_gwei,
			capture_rate_pct, entropy_capture_rate_pct
		FROM marts.agg_era_comparison FINAL
		WHERE window_days_var = @window_days
		ORDER BY era`,
		namedParam("window_days", int32(windowDays))) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("GetEraComparison: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []EraComparisonRow
	for rows.Next() {
		var r EraComparisonRow
		if err := rows.Scan(
			&r.Era, &r.WindowDays, &r.WindowTipEth,
			&r.AvgDailyRevenueEth, &r.AvgContestedShare, &r.AvgHHI,
			&r.AvgTopPayerShare, &r.AvgRevertRate, &r.AvgMedianFeeGwei,
			&r.CaptureRatePct, &r.EntropyCaptureRatePct,
		); err != nil {
			return nil, fmt.Errorf("GetEraComparison scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Opportunity types ──────────────────────────────────────────────────────────

type SpreadRow struct {
	BlockNumber  uint64  `json:"block_number"`
	PoolAddress  string  `json:"pool_address"`
	SpreadBps    float64 `json:"spread_bps"`
	AbsSpreadBps float64 `json:"abs_spread_bps"`
}

func (s *Store) GetSpreads(ctx context.Context, rangeStr string, limit int) ([]SpreadRow, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}

	// Join to fct_blocks to get block_timestamp for time filtering
	rows, err := s.query(ctx, `
		SELECT sp.block_number, sp.pool_address, sp.spread_bps, sp.abs_spread_bps
		FROM marts.fct_spread_per_block sp FINAL
		JOIN marts.fct_blocks fb FINAL ON sp.block_number = fb.block_number
		WHERE fb.block_timestamp >= @cutoff
		ORDER BY sp.block_number DESC
		LIMIT @limit`,
		namedParam("cutoff", cutoff), namedParam("limit", limit))
	if err != nil {
		return nil, fmt.Errorf("GetSpreads: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []SpreadRow
	for rows.Next() {
		var r SpreadRow
		if err := rows.Scan(&r.BlockNumber, &r.PoolAddress, &r.SpreadBps, &r.AbsSpreadBps); err != nil {
			return nil, fmt.Errorf("GetSpreads scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type HeatmapCell struct {
	PoolAddress     string  `json:"pool_address"`
	HourOfDay       uint8   `json:"hour_of_day"`
	DayOfWeek       uint8   `json:"day_of_week"`
	MedianSpreadBps float64 `json:"median_spread_bps"`
}

func (s *Store) GetSpreadHeatmap(ctx context.Context) ([]HeatmapCell, error) {
	rows, err := s.query(ctx, `
		SELECT pool_address, hour_of_day, day_of_week, median_spread_bps
		FROM marts.agg_spread_heatmap FINAL
		ORDER BY pool_address, day_of_week, hour_of_day`)
	if err != nil {
		return nil, fmt.Errorf("GetSpreadHeatmap: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []HeatmapCell
	for rows.Next() {
		var r HeatmapCell
		if err := rows.Scan(&r.PoolAddress, &r.HourOfDay, &r.DayOfWeek, &r.MedianSpreadBps); err != nil {
			return nil, fmt.Errorf("GetSpreadHeatmap scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type HistogramBin struct {
	PoolAddress string  `json:"pool_address"`
	Bin         string  `json:"bin"`
	BlockCount  uint64  `json:"block_count"`
	BlockPct    float64 `json:"block_pct"`
}

func (s *Store) GetSpreadHistogram(ctx context.Context) ([]HistogramBin, error) {
	rows, err := s.query(ctx, `
		SELECT pool_address, spread_bin, block_count, block_pct
		FROM marts.agg_spread_histogram FINAL
		ORDER BY pool_address, spread_bin`)
	if err != nil {
		return nil, fmt.Errorf("GetSpreadHistogram: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []HistogramBin
	for rows.Next() {
		var r HistogramBin
		if err := rows.Scan(&r.PoolAddress, &r.Bin, &r.BlockCount, &r.BlockPct); err != nil {
			return nil, fmt.Errorf("GetSpreadHistogram scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type MarkoutRow struct {
	BlockNumber  uint64   `json:"block_number"`
	TxHash       string   `json:"tx_hash"`
	EthDelta     float64  `json:"eth_delta"`
	ExecPriceUsd float64  `json:"exec_price_usd"`
	Markout1s    *float64 `json:"markout_usd_1s"`
	Markout300s  *float64 `json:"markout_usd_300s"`
}

func (s *Store) GetMarkouts(ctx context.Context, rangeStr string, limit int) ([]MarkoutRow, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}
	rows, err := s.query(ctx, `
		SELECT sm.block_number, sm.tx_hash, sm.eth_delta, sm.exec_price_usd,
			sm.markout_usd_1s, sm.markout_usd_300s
		FROM marts.fct_swap_markouts sm FINAL
		JOIN marts.fct_blocks fb FINAL ON sm.block_number = fb.block_number
		WHERE fb.block_timestamp >= @cutoff
		ORDER BY sm.block_number DESC
		LIMIT @limit`,
		namedParam("cutoff", cutoff), namedParam("limit", limit))
	if err != nil {
		return nil, fmt.Errorf("GetMarkouts: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []MarkoutRow
	for rows.Next() {
		var r MarkoutRow
		if err := rows.Scan(
			&r.BlockNumber, &r.TxHash, &r.EthDelta, &r.ExecPriceUsd,
			&r.Markout1s, &r.Markout300s,
		); err != nil {
			return nil, fmt.Errorf("GetMarkouts scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type PositionRow struct {
	Date                time.Time `json:"date"`
	EntityName          string    `json:"entity_name"`
	AvgFeeGwei          float64   `json:"avg_fee_gwei"`
	AvgTipRank          float64   `json:"avg_tip_rank"`
	DeltaRankVsPosition float64   `json:"delta_rank_vs_position"`
	Top3PositionShare   float64   `json:"top3_position_share"`
	FeeEfficiency       float64   `json:"fee_efficiency"`
}

func (s *Store) GetPositionSensitivity(ctx context.Context, rangeStr string) ([]PositionRow, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}
	rows, err := s.query(ctx, `
		SELECT date, entity_name, avg_fee_gwei, avg_tip_rank,
			delta_rank_vs_position, top3_position_share, fee_efficiency
		FROM marts.agg_entity_position_daily FINAL
		WHERE date >= toDate(@cutoff)
		ORDER BY date DESC, delta_rank_vs_position DESC`,
		namedParam("cutoff", cutoff))
	if err != nil {
		return nil, fmt.Errorf("GetPositionSensitivity: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []PositionRow
	for rows.Next() {
		var r PositionRow
		if err := rows.Scan(
			&r.Date, &r.EntityName, &r.AvgFeeGwei, &r.AvgTipRank,
			&r.DeltaRankVsPosition, &r.Top3PositionShare, &r.FeeEfficiency,
		); err != nil {
			return nil, fmt.Errorf("GetPositionSensitivity scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Execution quality ──────────────────────────────────────────────────────────

type ExecQualityRow struct {
	Date           time.Time `json:"date"`
	EntityName     string    `json:"entity_name"`
	RevertRate     float64   `json:"revert_rate"`
	AvgMarkout1s   *float64  `json:"avg_markout_usd_1s"`
	AvgMarkout300s *float64  `json:"avg_markout_usd_300s"`
}

func (s *Store) GetExecutionQuality(ctx context.Context, rangeStr string) ([]ExecQualityRow, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}
	rows, err := s.query(ctx, `
		SELECT date, entity_name, revert_rate, avg_markout_usd_1s, avg_markout_usd_300s
		FROM marts.fct_execution_quality_daily FINAL
		WHERE date >= toDate(@cutoff)
		ORDER BY date DESC`,
		namedParam("cutoff", cutoff))
	if err != nil {
		return nil, fmt.Errorf("GetExecutionQuality: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []ExecQualityRow
	for rows.Next() {
		var r ExecQualityRow
		if err := rows.Scan(
			&r.Date, &r.EntityName, &r.RevertRate, &r.AvgMarkout1s, &r.AvgMarkout300s,
		); err != nil {
			return nil, fmt.Errorf("GetExecutionQuality scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Fees types ─────────────────────────────────────────────────────────────────

type BaseFeeRow struct {
	BlockNumber      uint64    `json:"block_number"`
	BlockTimestamp   time.Time `json:"block_timestamp"`
	BaseFeeWei       string    `json:"base_fee_wei"`
	BaseFeeGwei      float64   `json:"base_fee_gwei"`
	GasUsed          uint64    `json:"gas_used"`
	GasLimit         uint64    `json:"gas_limit"`
	GasUtilisation   float64   `json:"gas_utilisation"`
	BaseFeeChangePct float64   `json:"base_fee_change_pct"`
	Era              string    `json:"era"`
}

func (s *Store) GetFeeHistory(ctx context.Context, rangeStr string, limit int) ([]BaseFeeRow, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}
	rows, err := s.query(ctx, `
		SELECT
			block_number, block_timestamp,
			toString(base_fee_per_gas) AS base_fee_wei,
			base_fee_per_gas_gwei, gas_used, gas_limit,
			gas_utilisation, base_fee_change_pct, era
		FROM marts.fct_base_fee_changes FINAL
		WHERE block_timestamp >= @cutoff
		ORDER BY block_number DESC
		LIMIT @limit`,
		namedParam("cutoff", cutoff), namedParam("limit", limit))
	if err != nil {
		return nil, fmt.Errorf("GetFeeHistory: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []BaseFeeRow
	for rows.Next() {
		var r BaseFeeRow
		if err := rows.Scan(
			&r.BlockNumber, &r.BlockTimestamp, &r.BaseFeeWei, &r.BaseFeeGwei,
			&r.GasUsed, &r.GasLimit, &r.GasUtilisation, &r.BaseFeeChangePct, &r.Era,
		); err != nil {
			return nil, fmt.Errorf("GetFeeHistory scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type BaseFeeHourlyRow struct {
	Hour           time.Time `json:"hour"`
	Era            string    `json:"era"`
	MinBaseFeeGwei float64   `json:"min_base_fee_gwei"`
	MaxBaseFeeGwei float64   `json:"max_base_fee_gwei"`
	AvgBaseFeeGwei float64   `json:"avg_base_fee_gwei"`
	MedBaseFeeGwei float64   `json:"median_base_fee_gwei"`
	HourChangePct  float64   `json:"hour_change_pct"`
}

func (s *Store) GetFeeHourly(ctx context.Context, rangeStr string) ([]BaseFeeHourlyRow, error) {
	cutoff, err := rangeCutoff(rangeStr)
	if err != nil {
		return nil, err
	}
	rows, err := s.query(ctx, `
		SELECT hour, era, min_base_fee_gwei, max_base_fee_gwei,
			avg_base_fee_gwei, median_base_fee_gwei, hour_change_pct
		FROM marts.agg_base_fee_hourly FINAL
		WHERE hour >= @cutoff
		ORDER BY hour ASC`,
		namedParam("cutoff", cutoff))
	if err != nil {
		return nil, fmt.Errorf("GetFeeHourly: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []BaseFeeHourlyRow
	for rows.Next() {
		var r BaseFeeHourlyRow
		if err := rows.Scan(
			&r.Hour, &r.Era, &r.MinBaseFeeGwei, &r.MaxBaseFeeGwei,
			&r.AvgBaseFeeGwei, &r.MedBaseFeeGwei, &r.HourChangePct,
		); err != nil {
			return nil, fmt.Errorf("GetFeeHourly scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetCurrentFee(ctx context.Context) (BaseFeeRow, error) {
	var r BaseFeeRow
	row := s.queryRow(ctx, `
		SELECT
			block_number, block_timestamp,
			toString(base_fee_per_gas) AS base_fee_wei,
			base_fee_per_gas_gwei, gas_used, gas_limit,
			gas_utilisation, base_fee_change_pct, era
		FROM marts.fct_base_fee_changes FINAL
		ORDER BY block_number DESC
		LIMIT 1`)
	if err := row.Scan(
		&r.BlockNumber, &r.BlockTimestamp, &r.BaseFeeWei, &r.BaseFeeGwei,
		&r.GasUsed, &r.GasLimit, &r.GasUtilisation, &r.BaseFeeChangePct, &r.Era,
	); err != nil {
		return BaseFeeRow{}, fmt.Errorf("GetCurrentFee: %w", err)
	}
	return r, nil
}

// ── Events ─────────────────────────────────────────────────────────────────────

type EventRow struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	EventDate   *string `json:"event_date,omitempty"`
	Block       *uint64 `json:"block,omitempty"`
	Description string  `json:"description"`
	SourceURL   string  `json:"source_url"`
}

func (s *Store) GetEvents(ctx context.Context) ([]EventRow, error) {
	rows, err := s.query(ctx, `
		SELECT id, label,
			nullIf(toString(event_date), '0000-00-00') AS event_date,
			nullIf(block, 0) AS block,
			description, source_url
		FROM marts.dim_events FINAL
		ORDER BY event_date ASC NULLS LAST`)
	if err != nil {
		return nil, fmt.Errorf("GetEvents: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []EventRow
	for rows.Next() {
		var r EventRow
		if err := rows.Scan(
			&r.ID, &r.Label, &r.EventDate, &r.Block, &r.Description, &r.SourceURL,
		); err != nil {
			return nil, fmt.Errorf("GetEvents scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Search ─────────────────────────────────────────────────────────────────────

type SearchResult struct {
	Type    string `json:"type"`
	Label   string `json:"label"`
	Value   string `json:"value"`
	SubText string `json:"sub_text,omitempty"`
}

func (s *Store) Search(ctx context.Context, q string) ([]SearchResult, error) {
	like := "%" + q + "%"
	rows, err := s.query(ctx, `
		SELECT 'block' AS type, toString(block_number) AS label,
			toString(block_number) AS value,
			formatDateTime(block_timestamp, '%Y-%m-%d %H:%M:%S') AS sub_text
		FROM marts.fct_blocks FINAL
		WHERE toString(block_number) LIKE @like
		LIMIT 5
		UNION ALL
		SELECT 'tx' AS type, tx_hash AS label, tx_hash AS value,
			toString(block_number) AS sub_text
		FROM marts.fct_transactions FINAL
		WHERE tx_hash LIKE @like
		LIMIT 5
		UNION ALL
		SELECT 'address' AS type, address AS label, address AS value,
			entity_name AS sub_text
		FROM marts.dim_entities FINAL
		WHERE address LIKE @like OR entity_name LIKE @like
		LIMIT 5
		ORDER BY type, label
		LIMIT 15`,
		namedParam("like", like))
	if err != nil {
		return nil, fmt.Errorf("Search: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Type, &r.Label, &r.Value, &r.SubText); err != nil {
			return nil, fmt.Errorf("Search scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// namedParam is a convenience wrapper for clickhouse named parameters.
func namedParam(name string, value any) driver.NamedValue {
	return clickhouse.Named(name, value)
}
