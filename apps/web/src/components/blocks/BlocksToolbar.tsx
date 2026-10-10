import type { BlockRow } from "@/api/types";
import { FilterInput, RangeButtons, Toggle } from "@/components/ui";
import type { Range } from "@/lib/search-params";
import type { RefObject } from "react";

interface BlocksToolbarProps {
  range: string;
  onRangeChange: (r: string) => void;
  search: string;
  onSearchChange: (v: string) => void;
  contested: boolean;
  onContestedChange: (v: boolean) => void;
  payer: string;
  onPayerChange: (v: string) => void;
  onRefresh: () => void;
  rows: BlockRow[];
  searchInputRef: RefObject<HTMLInputElement | null>;
}

const iconBtnStyle: React.CSSProperties = {
  padding: "4px 8px",
  fontSize: 11,
  background: "transparent",
  color: "var(--color-text-muted)",
  border: "1px solid var(--color-border)",
  borderRadius: "var(--radius-badge)",
  cursor: "pointer",
};

function exportCsv(rows: BlockRow[]) {
  const headers = [
    "block_number",
    "block_timestamp",
    "tx_count",
    "tipped_tx_count",
    "tip_revenue_eth",
    "top_fee_gwei",
    "second_to_first_ratio",
    "block_type",
  ];
  const lines = [
    headers.join(","),
    ...rows.map((r) =>
      [
        r.block_number,
        r.block_timestamp,
        r.tx_count,
        r.tipped_tx_count,
        r.tip_revenue_eth,
        r.top_fee_gwei,
        r.second_to_first_ratio,
        r.block_type,
      ].join(","),
    ),
  ];
  const blob = new Blob([lines.join("\n")], { type: "text/csv" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "blocks.csv";
  a.click();
  URL.revokeObjectURL(url);
}

export function BlocksToolbar({
  range,
  onRangeChange,
  search,
  onSearchChange,
  contested,
  onContestedChange,
  payer,
  onPayerChange,
  onRefresh,
  rows,
  searchInputRef,
}: BlocksToolbarProps) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 8,
        marginBottom: 8,
        flexWrap: "wrap",
      }}
    >
      <FilterInput
        ref={searchInputRef}
        value={search}
        onChange={onSearchChange}
        placeholder="Search block, address, entity…"
        width={240}
      />
      <RangeButtons value={range as Range} onChange={onRangeChange} />
      <Toggle checked={contested} onChange={onContestedChange} label="Contested only" />
      <FilterInput
        value={payer}
        onChange={onPayerChange}
        placeholder="Filter by payer…"
        width={180}
      />
      <div style={{ flex: 1 }} />
      <button type="button" onClick={onRefresh} aria-label="Refresh" style={iconBtnStyle}>
        ↻
      </button>
      <button
        type="button"
        onClick={() => exportCsv(rows)}
        aria-label="Export CSV"
        style={iconBtnStyle}
      >
        ↓ CSV
      </button>
    </div>
  );
}
