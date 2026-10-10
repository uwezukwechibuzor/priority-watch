import type { HealthDailyRow, PayerShareRow } from "@/api/types";
import { KpiTile } from "@/components/ui";
import {
  HHI_BAND_COLOR,
  compact,
  formatEth,
  formatHhi,
  formatPct,
  formatUsd,
  hhiBand,
} from "@/lib/formatters";
import { Sparkline } from "@/lib/sparkline";

interface HealthKpiStripProps {
  daily: HealthDailyRow[];
  leaderboard: PayerShareRow[];
  loading: boolean;
}

export function HealthKpiStrip({ daily, leaderboard, loading }: HealthKpiStripProps) {
  const totalRevEth = daily.reduce((s, d) => s + d.tip_revenue_eth, 0);
  const totalRevUsd = daily.reduce((s, d) => s + d.tip_revenue_usd, 0);
  const totalBlocks = daily.reduce((s, d) => s + d.block_count, 0);
  const avgBlocksPct =
    daily.length > 0 ? daily.reduce((s, d) => s + d.blocks_pct_of_expected, 0) / daily.length : 0;
  const avgHhi = daily.length > 0 ? daily.reduce((s, d) => s + d.hhi, 0) / daily.length : 0;
  const avgContested =
    daily.length > 0 ? daily.reduce((s, d) => s + d.contested_share, 0) / daily.length : 0;
  const avgRevert =
    daily.length > 0 ? daily.reduce((s, d) => s + d.revert_rate_tipped, 0) / daily.length : 0;

  const topPayer = leaderboard[0];
  const band = hhiBand(avgHhi);

  const windowLabel = daily.length > 0 ? `${daily.length}D` : "—";

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
        label={`REVENUE ${windowLabel}`}
        value={formatEth(totalRevEth)}
        subtext={formatUsd(totalRevUsd)}
        loading={loading}
      >
        <Sparkline data={daily.map((d) => d.tip_revenue_eth)} />
      </KpiTile>

      <KpiTile
        label={`BLOCKS ${windowLabel}`}
        value={compact(totalBlocks)}
        subtext={`${formatPct(avgBlocksPct)} of expected`}
        loading={loading}
      >
        <Sparkline data={daily.map((d) => d.block_count)} color="var(--color-green)" />
      </KpiTile>

      <KpiTile
        label={`HHI ${windowLabel}`}
        value={formatHhi(Math.round(avgHhi))}
        subtext={band}
        subtextColor={HHI_BAND_COLOR[band]}
        loading={loading}
      >
        <Sparkline data={daily.map((d) => d.hhi)} color={HHI_BAND_COLOR[band]} />
      </KpiTile>

      <KpiTile label={`CONTESTED ${windowLabel}`} value={formatPct(avgContested)} loading={loading}>
        <Sparkline data={daily.map((d) => d.contested_share)} color="var(--color-purple)" />
      </KpiTile>

      <KpiTile
        label="TOP PAYER"
        value={topPayer ? formatPct(topPayer.share_pct / 100) : "—"}
        subtext={topPayer?.entity_name ?? "—"}
        loading={loading}
      />

      <KpiTile
        label="REVERT RATE"
        value={formatPct(avgRevert)}
        subtext="tipped txs only"
        loading={loading}
      >
        <Sparkline data={daily.map((d) => d.revert_rate_tipped)} color="var(--color-red)" />
      </KpiTile>
    </div>
  );
}
