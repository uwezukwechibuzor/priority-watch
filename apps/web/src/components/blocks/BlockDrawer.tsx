import type { TxRow } from "@/api/types";
import { Drawer } from "@/components/ui";
import {
  arbiscanBlock,
  arbiscanTx,
  entityColor,
  formatGwei,
  formatTime,
  shortAddress,
} from "@/lib/formatters";
import { useECharts } from "@/lib/use-echarts";
import { blockDetailQuery, blockTxsQuery } from "@/queries/blocks";
import { useQuery } from "@tanstack/react-query";
import type { EChartsOption } from "echarts";

interface BlockDrawerProps {
  blockNumber: number | undefined;
  onClose: () => void;
}

function TxScatter({ txs }: { txs: TxRow[] }) {
  const tipped = txs.filter((t) => t.priority_fee_per_gas_gwei > 0);
  const option: EChartsOption = {
    grid: { left: 48, right: 16, top: 16, bottom: 28, containLabel: false },
    xAxis: {
      type: "value",
      name: "Rank",
      nameLocation: "end",
      nameTextStyle: { fontSize: 10, color: "#6b7280" },
      axisLabel: { fontSize: 10, color: "#6b7280" },
      axisLine: { lineStyle: { color: "#1f2329" } },
      splitLine: { lineStyle: { color: "#1f2329" } },
    },
    yAxis: {
      type: "value",
      name: "gwei",
      nameLocation: "end",
      nameTextStyle: { fontSize: 10, color: "#6b7280" },
      axisLabel: { fontSize: 10, color: "#6b7280" },
      splitLine: { lineStyle: { color: "#1f2329" } },
    },
    tooltip: {
      trigger: "item",
      formatter: (params: unknown) => {
        const p = params as { data: number[] };
        return `Rank: ${p.data[0]}<br/>Fee: ${p.data[1]?.toFixed(4)} gwei`;
      },
    },
    series: [
      {
        type: "scatter",
        data: tipped.map((t) => [t.tip_rank, t.priority_fee_per_gas_gwei]),
        symbolSize: 5,
        itemStyle: { color: "#3b82f6", opacity: 0.7 },
      },
    ],
  };
  const ref = useECharts(option, [txs]);
  return <div ref={ref} style={{ height: 140, marginTop: 16 }} />;
}

export function BlockDrawer({ blockNumber, onClose }: BlockDrawerProps) {
  const open = blockNumber !== undefined;

  const { data: block } = useQuery({
    ...blockDetailQuery(blockNumber ?? 0),
    enabled: open,
  });
  const { data: txsPage } = useQuery({
    ...blockTxsQuery(blockNumber ?? 0),
    enabled: open,
  });

  const txs = txsPage?.data ?? [];

  return (
    <Drawer open={open} onClose={onClose} title={`BLOCK #${blockNumber ?? ""}`} width={480}>
      {block && (
        <div>
          {/* Header metrics */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: 8,
              marginBottom: 12,
            }}
          >
            <div>
              <div className="label" style={{ marginBottom: 2 }}>
                Timestamp
              </div>
              <div style={{ fontSize: 12 }}>{formatTime(block.block_timestamp)}</div>
            </div>
            <div>
              <div className="label" style={{ marginBottom: 2 }}>
                Base Fee
              </div>
              <div className="tabular" style={{ fontSize: 12 }}>
                {formatGwei(block.base_fee_per_gas_gwei)}
              </div>
            </div>
            <div>
              <div className="label" style={{ marginBottom: 2 }}>
                Gas Used
              </div>
              <div className="tabular" style={{ fontSize: 12 }}>
                {block.gas_used.toLocaleString()} / {block.gas_limit.toLocaleString()}
              </div>
            </div>
            <div>
              <div className="label" style={{ marginBottom: 2 }}>
                Gas Used %
              </div>
              <div className="tabular" style={{ fontSize: 12 }}>
                {(block.gas_used_pct * 100).toFixed(1)}%
              </div>
            </div>
          </div>

          {/* Arbiscan link */}
          <a
            href={arbiscanBlock(block.block_number)}
            target="_blank"
            rel="noreferrer"
            style={{
              fontSize: 12,
              color: "var(--color-accent)",
              display: "inline-block",
              marginBottom: 16,
            }}
          >
            View on Arbiscan ↗
          </a>

          {/* Tx scatter */}
          {txs.length > 0 && <TxScatter txs={txs} />}

          {/* Tx table */}
          <div style={{ marginTop: 16 }}>
            <div className="label" style={{ marginBottom: 6 }}>
              Transactions ({txs.length})
            </div>
            <div style={{ overflowX: "auto" }}>
              <table
                style={{
                  width: "100%",
                  borderCollapse: "collapse",
                  fontSize: 11,
                }}
              >
                <thead>
                  <tr>
                    {["#", "TX", "FROM", "FEE (GWEI)", "GAS", "ST"].map((h) => (
                      <th
                        key={h}
                        style={{
                          padding: "4px 6px",
                          textAlign: "left",
                          borderBottom: "1px solid var(--color-border)",
                          fontWeight: 500,
                          textTransform: "uppercase",
                          letterSpacing: "0.06em",
                          fontSize: 10,
                          color: "var(--color-text-muted)",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {txs.map((tx) => (
                    <tr key={tx.tx_hash} style={{ borderBottom: "1px solid var(--color-border)" }}>
                      <td
                        className="tabular"
                        style={{ padding: "4px 6px", color: "var(--color-text-muted)" }}
                      >
                        {tx.tip_rank}
                      </td>
                      <td style={{ padding: "4px 6px" }}>
                        <a
                          href={arbiscanTx(tx.tx_hash)}
                          target="_blank"
                          rel="noreferrer"
                          style={{ color: "var(--color-accent)", fontFamily: "var(--font-mono)" }}
                        >
                          {tx.tx_hash.slice(0, 8)}…
                        </a>
                      </td>
                      <td style={{ padding: "4px 6px" }}>
                        <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                          <span
                            aria-hidden="true"
                            style={{
                              width: 6,
                              height: 6,
                              borderRadius: 1,
                              background: entityColor(tx.color_index),
                              flexShrink: 0,
                            }}
                          />
                          <span
                            className="mono"
                            style={{
                              fontSize: 11,
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whiteSpace: "nowrap",
                              maxWidth: 90,
                            }}
                          >
                            {tx.entity_name || shortAddress(tx.from_address)}
                          </span>
                        </span>
                      </td>
                      <td className="tabular" style={{ padding: "4px 6px", textAlign: "right" }}>
                        {tx.priority_fee_per_gas_gwei.toFixed(4)}
                      </td>
                      <td className="tabular" style={{ padding: "4px 6px", textAlign: "right" }}>
                        {tx.gas_used.toLocaleString()}
                      </td>
                      <td
                        style={{
                          padding: "4px 6px",
                          textAlign: "center",
                          color: tx.status === 1 ? "var(--color-green)" : "var(--color-red)",
                        }}
                      >
                        {tx.status === 1 ? "✓" : "✗"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </Drawer>
  );
}
