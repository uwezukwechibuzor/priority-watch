// Auto-generated from /openapi.json via `pnpm gen:api`.
// Run `pnpm gen:api` to regenerate after API changes.

export interface IngesterStatusRow {
  source: string;
  status: "live" | "lagging" | "down";
  lag_seconds: number;
  latest_position: string;
  latest_timestamp: string;
  last_updated_at: string;
}

export interface SearchResult {
  type: "block" | "tx" | "address";
  label: string;
  value: string;
  sub_text?: string;
}

export interface BlockRow {
  block_number: number;
  block_timestamp: string;
  era: string;
  tx_count: number;
  tipped_tx_count: number;
  tip_revenue_wei: string;
  tip_revenue_eth: number;
  tip_revenue_usd: number;
  eth_price_usd: number;
  top_fee_gwei: number;
  top_tip_usd: number;
  median_fee_gwei: number;
  top_payer_address: string;
  top_payer_name: string;
  top_payer_color_index: number;
  revert_count: number;
  second_to_first_ratio: number;
  block_type: "contested" | "single" | "untipped";
  ordering_score: number;
  distinct_tippers: number;
  base_fee_per_gas_wei: string;
  base_fee_per_gas_gwei: number;
  gas_used: number;
  gas_limit: number;
  gas_used_pct: number;
}

export interface BlocksResponse {
  data: BlockRow[];
  next_cursor?: string;
}

export interface BlockKPIs {
  block_count: number;
  tip_revenue_wei: string;
  tip_revenue_eth: number;
  tip_revenue_usd: number;
  avg_tip_per_block_eth: number;
  contested_share: number;
  top_payer_name: string;
  top_payer_share: number;
  hhi: number;
  hhi_band: string;
  avg_top_fee_gwei: number;
}

export interface PayerShareRow {
  entity_name: string;
  entity_type: string;
  color_index: number;
  tip_eth: number;
  tip_usd: number;
  share_pct: number;
  tx_count: number;
  revert_rate: number;
  avg_tip_rank: number;
}

export interface ModelEvalRow {
  eval_date: string;
  model_name: string;
  n: number;
  confidence: number;
  window_blocks: number;
  hit_rate: number;
  avg_cost_gwei: number;
  eval_blocks: number;
}

export interface Recommendation {
  fee_gwei: number;
  low_gwei: number;
  high_gwei: number;
  hit_rate: number;
  avg_cost_gwei: number;
  model_name: string;
  reasoning: Array<{ label: string; value: number; unit?: string }>;
}

export interface SeriesPoint {
  ts: string;
  tip_revenue_eth: number;
  tip_revenue_usd: number;
  top_fee_gwei: number;
  block_count: number;
  tipped_tx_count: number;
  contested_share: number;
  hhi: number;
}

export interface TippedHistRow {
  bin: string;
  block_count: number;
}

export interface TxRow {
  tx_hash: string;
  tx_index: number;
  block_number: number;
  from_address: string;
  to_address: string;
  entity_name: string;
  entity_type: string;
  color_index: number;
  tx_type: number;
  tx_class: string;
  priority_fee_per_gas_gwei: number;
  tip_paid_eth: number;
  tip_paid_usd: number;
  tip_paid_wei: string;
  gas_used: number;
  status: number;
  tip_rank: number;
  is_reverted_tipped: number;
}

export interface Pagination {
  next_cursor?: string;
}

export interface EventRow {
  id: string;
  event_date: string;
  block: number;
  label: string;
  description: string;
  source_url: string;
}

export interface HealthDailyRow {
  date: string;
  era: string;
  tip_revenue_eth: number;
  tip_revenue_usd: number;
  tip_revenue_wei: string;
  tx_count: number;
  tipped_tx_count: number;
  p10_fee_gwei: number;
  p50_fee_gwei: number;
  p90_fee_gwei: number;
  block_count: number;
  blocks_expected: number;
  blocks_pct_of_expected: number;
  contested_share: number;
  hhi: number;
  hhi_band: string;
  top_payer_share: number;
  revert_rate_tipped: number;
  median_user_cost_usd: number;
}

export interface TreasuryDailyRow {
  date: string;
  era: string;
  total_tip_wei: string;
  total_tip_eth: number;
  total_tip_usd: number;
  treasury_wei: string;
  developer_guild_wei: string;
  tx_count: number;
  tipped_tx_count: number;
  status: string;
}

export interface TreasurySummary {
  total_tip_eth: number;
  total_tip_usd: number;
  total_tip_wei: string;
  treasury_eth: number;
  developer_guild_eth: number;
  avg_daily_tip_eth: number;
  projected_annual_eth: number;
  day_count: number;
}

export interface EraComparisonRow {
  era: string;
  window_days: number;
  window_tip_eth: number;
  avg_daily_revenue_eth: number;
  avg_contested_share: number;
  avg_hhi: number;
  avg_top_payer_share: number;
  avg_revert_rate: number;
  avg_median_fee_gwei: number;
  capture_rate_pct: number;
  entropy_capture_rate_pct: number;
}

export interface BaseFeeHourlyRow {
  hour: string;
  era: string;
  min_base_fee_gwei: number;
  max_base_fee_gwei: number;
  avg_base_fee_gwei: number;
  median_base_fee_gwei: number;
  hour_change_pct: number;
}
