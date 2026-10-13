import type { BlockKPIs, SeriesPoint } from "@/api/types";
import { KpiTile } from "@/components/ui";
import {
  HHI_BAND_COLOR,
  compact,
  formatEth,
  formatPct,
  formatUsd,
  hhiBand,
} from "@/lib/formatters";
import { Sparkline } from "@/lib/sparkline";

interface BlocksKpiStripProps {
  kpis: BlockKPIs | undefined;
  series: SeriesPoint[];
  loading: boolean;
}

export function BlocksKpiStrip({ kpis, series, loading }: BlocksKpiStripProps) {
  const band = hhiBand(kpis?.hhi ?? 0);

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "repeat(6, 1fr)",
        gap: 8,
        marginBottom: 8,
      }}
    >
      <KpiTile
        label="BLOCKS"
        value={compact(kpis?.block_count ?? 0)}
        subtext={kpis?.max_block_number ? `height: ${kpis.max_block_number.toLocaleString()}` : "—"}
        loading={loading}
      >
        <Sparkline data={series.map((p) => p.block_count)} />
      </KpiTile>

      <KpiTile
        label="TIP REVENUE"
        value={formatEth(kpis?.tip_revenue_eth ?? 0)}
        subtext={formatUsd(kpis?.tip_revenue_usd ?? 0)}
        loading={loading}
      >
        <Sparkline data={series.map((p) => p.tip_revenue_eth)} />
      </KpiTile>

      <KpiTile
        label="AVG TIP / BLOCK"
        value={formatEth(kpis?.avg_tip_per_block_eth ?? 0)}
        loading={loading}
      >
        <Sparkline
          data={series.map((p) => (p.block_count > 0 ? p.tip_revenue_eth / p.block_count : 0))}
        />
      </KpiTile>

      <KpiTile
        label="CONTESTED"
        value={formatPct(kpis?.contested_share ?? 0)}
        subtext="blocks where 2+ entities paid tips"
        loading={loading}
      >
        <Sparkline data={series.map((p) => p.contested_share)} color="var(--color-purple)" />
      </KpiTile>

      <KpiTile
        label="TOP PAYER"
        value={formatPct(kpis?.top_payer_share ?? 0)}
        subtext={kpis?.top_payer_name ?? "—"}
        loading={loading}
      >
        <Sparkline data={[]} />
      </KpiTile>

      <KpiTile
        label="HHI"
        value={Math.round(kpis?.hhi ?? 0).toLocaleString()}
        subtext={kpis?.hhi_band ?? "—"}
        subtextColor={HHI_BAND_COLOR[band]}
        loading={loading}
      >
        <Sparkline data={series.map((p) => p.hhi)} color={HHI_BAND_COLOR[band]} />
      </KpiTile>
    </div>
  );
}
