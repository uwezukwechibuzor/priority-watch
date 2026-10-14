import type { AllPayerRow } from "@/api/types";
import { Drawer } from "@/components/ui/Drawer";
import { formatEth } from "@/lib/formatters";
import { allPayersQuery } from "@/queries/health";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

interface AllPayersDrawerProps {
  open: boolean;
  onClose: () => void;
}

function shortAddr(addr: string): string {
  if (!addr || addr.length < 10) return addr;
  return `${addr.slice(0, 6)}…${addr.slice(-4)}`;
}

export function AllPayersDrawer({ open, onClose }: AllPayersDrawerProps) {
  const { data: rows = [], isLoading } = useQuery({
    ...allPayersQuery,
    enabled: open,
  });
  const [filter, setFilter] = useState("");

  const filtered = filter
    ? rows.filter(
        (r) =>
          r.entity_name.toLowerCase().includes(filter.toLowerCase()) ||
          r.from_address.toLowerCase().includes(filter.toLowerCase()) ||
          r.contract_label.toLowerCase().includes(filter.toLowerCase()),
      )
    : rows;

  return (
    <Drawer open={open} onClose={onClose} title="ALL PAYERS — HISTORICAL" width={780}>
      {/* Search */}
      <div style={{ marginBottom: 12 }}>
        <input
          type="search"
          placeholder="Filter by address, entity or contract…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          style={{
            width: "100%",
            padding: "6px 10px",
            background: "var(--color-bg-base)",
            border: "1px solid var(--color-border)",
            borderRadius: 4,
            color: "var(--color-text-primary)",
            fontSize: 12,
            outline: "none",
            boxSizing: "border-box",
          }}
        />
      </div>

      {/* Table */}
      <div style={{ overflowX: "auto" }}>
        <table
          style={{
            width: "100%",
            borderCollapse: "collapse",
            fontSize: 12,
          }}
        >
          <thead>
            <tr
              style={{
                borderBottom: "1px solid var(--color-border)",
                color: "var(--color-text-muted)",
                fontSize: 10,
                textTransform: "uppercase",
                letterSpacing: "0.06em",
              }}
            >
              <th style={{ padding: "6px 8px", textAlign: "left", fontWeight: 500 }}>#</th>
              <th style={{ padding: "6px 8px", textAlign: "left", fontWeight: 500 }}>From</th>
              <th style={{ padding: "6px 8px", textAlign: "left", fontWeight: 500 }}>Contract</th>
              <th style={{ padding: "6px 8px", textAlign: "left", fontWeight: 500 }}>Name</th>
              <th style={{ padding: "6px 8px", textAlign: "right", fontWeight: 500 }}>Tip (ETH)</th>
              <th style={{ padding: "6px 8px", textAlign: "right", fontWeight: 500 }}>Txs</th>
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array.from({ length: 12 }, (_, i) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: skeleton rows have no stable id
                  <tr key={i}>
                    {Array.from({ length: 6 }, (_, j) => (
                      // biome-ignore lint/suspicious/noArrayIndexKey: skeleton cells
                      <td key={j} style={{ padding: "6px 8px" }}>
                        <div
                          className="skeleton"
                          style={{ height: 14, borderRadius: 2, width: "80%" }}
                        />
                      </td>
                    ))}
                  </tr>
                ))
              : filtered.map((row, idx) => (
                  <PayerRow key={row.from_address} rank={idx + 1} row={row} />
                ))}
          </tbody>
        </table>

        {!isLoading && filtered.length === 0 && (
          <div
            style={{
              padding: "32px 0",
              textAlign: "center",
              color: "var(--color-text-muted)",
              fontSize: 12,
            }}
          >
            No results
          </div>
        )}
      </div>

      <div
        style={{
          marginTop: 12,
          fontSize: 11,
          color: "var(--color-text-muted)",
        }}
      >
        {filtered.length} payers{filter ? ` (filtered from ${rows.length})` : ""} · all-time · contract = most-tipped interaction
      </div>
    </Drawer>
  );
}

function PayerRow({ rank, row }: { rank: number; row: AllPayerRow }) {
  const isEntity = row.entity_name !== row.from_address;

  return (
    <tr
      style={{
        borderBottom: "1px solid var(--color-border)",
      }}
    >
      <td
        style={{
          padding: "7px 8px",
          color: "var(--color-text-muted)",
          fontVariantNumeric: "tabular-nums",
          width: 32,
        }}
      >
        {rank}
      </td>
      <td style={{ padding: "7px 8px" }}>
        <div style={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {isEntity && (
            <span style={{ color: "var(--color-text-primary)", fontWeight: 500 }}>
              {row.entity_name}
            </span>
          )}
          <span
            title={row.from_address}
            style={{
              fontFamily: "var(--font-mono)",
              fontSize: 11,
              color: isEntity ? "var(--color-text-muted)" : "var(--color-text-primary)",
            }}
          >
            {shortAddr(row.from_address)}
          </span>
        </div>
      </td>
      <td style={{ padding: "7px 8px" }}>
        {row.to_address ? (
          <span
            title={row.to_address}
            style={{ fontFamily: "var(--font-mono)", fontSize: 11, color: "var(--color-text-muted)" }}
          >
            {shortAddr(row.to_address)}
          </span>
        ) : (
          <span style={{ color: "var(--color-text-muted)" }}>—</span>
        )}
      </td>
      <td style={{ padding: "7px 8px" }}>
        {row.contract_label ? (
          <span
            style={{
              padding: "2px 6px",
              background: "var(--color-bg-base)",
              border: "1px solid var(--color-border)",
              borderRadius: 3,
              fontSize: 11,
              color: "var(--color-text-primary)",
            }}
          >
            {row.contract_label}
          </span>
        ) : (
          <span style={{ color: "var(--color-text-muted)" }}>—</span>
        )}
      </td>
      <td
        style={{
          padding: "7px 8px",
          textAlign: "right",
          fontVariantNumeric: "tabular-nums",
          color: "var(--color-text-primary)",
        }}
      >
        {formatEth(row.tip_eth)}
      </td>
      <td
        style={{
          padding: "7px 8px",
          textAlign: "right",
          fontVariantNumeric: "tabular-nums",
          color: "var(--color-text-muted)",
        }}
      >
        {row.tx_count.toLocaleString()}
      </td>
    </tr>
  );
}
