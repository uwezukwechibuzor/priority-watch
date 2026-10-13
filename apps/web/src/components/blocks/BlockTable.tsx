import type { BlockRow } from "@/api/types";
import { EntityChip, RatioBar, TypeBadge } from "@/components/ui";
import { compact, formatEth, formatGwei, relativeTime } from "@/lib/formatters";
import { type ListBlocksParams, blocksInfiniteQuery } from "@/queries/blocks";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useEffect, useRef } from "react";

interface BlockTableProps {
  params: ListBlocksParams;
  range?: string;
  selectedBlock?: number;
  onBlockSelect: (b: BlockRow) => void;
  liveMode?: boolean;
}

export function BlockTable({
  params,
  selectedBlock,
  onBlockSelect,
  liveMode = false,
}: BlockTableProps) {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } = useInfiniteQuery({
    ...blocksInfiniteQuery(params),
    refetchInterval: liveMode ? 2000 : false,
  });

  const rows = data?.pages.flatMap((p) => p.data ?? []) ?? [];

  const parentRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const selectedIndexRef = useRef(-1);

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 36,
    overscan: 10,
  });

  // IntersectionObserver sentinel for infinite scroll
  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && hasNextPage && !isFetchingNextPage) {
          void fetchNextPage();
        }
      },
      { threshold: 0.1 },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [fetchNextPage, hasNextPage, isFetchingNextPage]);

  // Keyboard navigation
  useEffect(() => {
    const el = parentRef.current;
    if (!el) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "j" || e.key === "ArrowDown") {
        e.preventDefault();
        selectedIndexRef.current = Math.min(selectedIndexRef.current + 1, rows.length - 1);
        virtualizer.scrollToIndex(selectedIndexRef.current, { align: "auto" });
      } else if (e.key === "k" || e.key === "ArrowUp") {
        e.preventDefault();
        selectedIndexRef.current = Math.max(selectedIndexRef.current - 1, 0);
        virtualizer.scrollToIndex(selectedIndexRef.current, { align: "auto" });
      } else if (e.key === "Enter") {
        const row = rows[selectedIndexRef.current];
        if (row) onBlockSelect(row);
      }
    };
    el.addEventListener("keydown", handler);
    return () => el.removeEventListener("keydown", handler);
  }, [rows, virtualizer, onBlockSelect]);

  // flex: 0 = fixed width; flex > 0 = grows proportionally to fill available space
  const COLS = [
    { key: "block",   header: "BLOCK",       minWidth: 112, flex: 0 },
    { key: "time",    header: "TIME",         minWidth: 100, flex: 1 },
    { key: "txs",     header: "TXS",          minWidth: 50,  flex: 0 },
    { key: "payer",   header: "TOP PAYER",    minWidth: 160, flex: 2 },
    { key: "fee",     header: "TOP FEE",      minWidth: 100, flex: 1 },
    { key: "revenue", header: "TIP REVENUE",  minWidth: 120, flex: 1 },
    { key: "ratio",   header: "2ND/1ST",      minWidth: 100, flex: 0 },
    { key: "tipped",  header: "TIPPED",       minWidth: 60,  flex: 0 },
    { key: "type",    header: "TYPE",         minWidth: 90,  flex: 0 },
  ];

  function renderCell(row: BlockRow, key: string) {
    switch (key) {
      case "block":
        return (
          <span className="tabular" style={{ fontSize: 12, fontWeight: 500 }}>
            {row.block_number.toLocaleString()}
          </span>
        );
      case "time":
        return (
          <span style={{ fontSize: 12, color: "var(--color-text-muted)" }}>
            {relativeTime(row.block_timestamp)}
          </span>
        );
      case "txs":
        return (
          <span className="tabular" style={{ fontSize: 12 }}>
            {compact(row.tx_count)}
          </span>
        );
      case "payer":
        return !row.top_payer_address ? (
          <span style={{ color: "var(--color-text-muted)", fontSize: 12 }}>—</span>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <EntityChip
              name={row.top_payer_name || row.top_payer_address}
              colorIndex={row.top_payer_color_index}
              address={row.top_payer_address}
            />
            {row.top_payer_address_label && (
              <span
                style={{
                  fontSize: 9,
                  color: "var(--color-text-muted)",
                  fontFamily: "var(--font-mono)",
                  paddingLeft: 10,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  maxWidth: 140,
                }}
                title={`Interacted with: ${row.top_payer_address_label}`}
              >
                {row.top_payer_address_label}
              </span>
            )}
          </div>
        );
      case "fee":
        return (
          <span className="tabular" style={{ fontSize: 12 }}>
            {formatGwei(row.top_fee_gwei)}
          </span>
        );
      case "revenue":
        return (
          <span className="tabular" style={{ fontSize: 12 }}>
            {formatEth(row.tip_revenue_eth)}
          </span>
        );
      case "ratio":
        return <RatioBar ratio={row.second_to_first_ratio} width={56} />;
      case "tipped":
        return (
          <span className="tabular" style={{ fontSize: 12 }}>
            {row.tipped_tx_count}
          </span>
        );
      case "type":
        return <TypeBadge type={row.block_type} />;
      default:
        return null;
    }
  }

  return (
    <div
      style={{
        background: "var(--color-bg-panel)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-panel)",
        overflow: "hidden",
      }}
    >
      {/* Sticky header */}
      <div
        style={{
          display: "flex",
          borderBottom: "1px solid var(--color-border)",
          background: "var(--color-bg-panel)",
          position: "sticky",
          top: 0,
          zIndex: 1,
        }}
      >
        {COLS.map((col) => (
          <div
            key={col.key}
            style={{
              minWidth: col.minWidth,
              flex: col.flex > 0 ? col.flex : `0 0 ${col.minWidth}px`,
              padding: "6px 8px",
              fontSize: 10,
              fontWeight: 500,
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              color: "var(--color-text-muted)",
            }}
          >
            {col.header}
          </div>
        ))}
      </div>

      {/* Virtualised body */}
      <div
        ref={parentRef}
        role="grid"
        tabIndex={0}
        style={{ overflowY: "auto", maxHeight: 520, outline: "none" }}
      >
        {isLoading ? (
          <div style={{ padding: 16 }}>
            {Array.from({ length: 8 }, (_, i) => (
              <div
                key={i}
                className="skeleton"
                style={{ height: 28, marginBottom: 4, borderRadius: 3 }}
              />
            ))}
          </div>
        ) : (
          <div
            style={{
              height: virtualizer.getTotalSize(),
              position: "relative",
            }}
          >
            {virtualizer.getVirtualItems().map((vi) => {
              const row = rows[vi.index];
              if (!row) return null;
              const isSelected = selectedBlock === row.block_number;
              return (
                <div
                  key={row.block_number}
                  role="row"
                  tabIndex={-1}
                  onClick={() => onBlockSelect(row)}
                  onKeyDown={(e) => e.key === "Enter" && onBlockSelect(row)}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    transform: `translateY(${vi.start}px)`,
                    height: 36,
                    display: "flex",
                    alignItems: "center",
                    cursor: "pointer",
                    background: isSelected ? "rgba(59,130,246,0.08)" : "transparent",
                    borderBottom: "1px solid var(--color-border)",
                  }}
                >
                  {COLS.map((col) => (
                    <div
                      key={col.key}
                      role="gridcell"
                      style={{
                        minWidth: col.minWidth,
                        flex: col.flex > 0 ? col.flex : `0 0 ${col.minWidth}px`,
                        padding: "0 8px",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                      }}
                    >
                      {renderCell(row, col.key)}
                    </div>
                  ))}
                </div>
              );
            })}
          </div>
        )}

        {/* Infinite scroll sentinel */}
        <div ref={sentinelRef} style={{ height: 1 }} />
        {isFetchingNextPage && (
          <div
            style={{
              padding: "8px 0",
              textAlign: "center",
              fontSize: 11,
              color: "var(--color-text-muted)",
            }}
          >
            Loading more…
          </div>
        )}
        {!isLoading && rows.length === 0 && (
          <div
            style={{
              padding: "40px 0",
              textAlign: "center",
              color: "var(--color-text-muted)",
              fontSize: 12,
            }}
          >
            No data for this range
          </div>
        )}
      </div>
    </div>
  );
}
