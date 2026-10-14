import { useVirtualizer } from "@tanstack/react-virtual";
import { type ReactNode, useCallback, useEffect, useRef } from "react";

export interface Column<T> {
  key: string;
  header: string;
  width?: number;
  minWidth?: number;
  render: (row: T) => ReactNode;
  align?: "left" | "right" | "center";
}

interface DataTableProps<T> {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string | number;
  onRowClick?: (row: T) => void;
  selectedKey?: string | number;
  caption?: string;
  rowHeight?: number;
  maxHeight?: number;
  loading?: boolean;
}

function buildGridTemplate(columns: Column<unknown>[]): string {
  const parts = columns.map((col) => {
    if (col.width) return `${col.width}px`;
    if (col.minWidth) return `minmax(${col.minWidth}px, 1fr)`;
    return "1fr";
  });
  // If every column is fixed-width, make the last one fill to avoid a
  // dead gap on the right side of wide containers.
  const hasFlexible = parts.some((p) => p.includes("minmax") || p === "1fr");
  if (!hasFlexible && parts.length > 0) {
    const last = columns[parts.length - 1];
    parts[parts.length - 1] = last?.minWidth ? `minmax(${last.minWidth}px, 1fr)` : "1fr";
  }
  return parts.join(" ");
}

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  onRowClick,
  selectedKey,
  caption,
  rowHeight = 36,
  maxHeight = 600,
  loading = false,
}: DataTableProps<T>) {
  const parentRef = useRef<HTMLDivElement>(null);
  const selectedIndexRef = useRef(-1);
  const gridTemplate = buildGridTemplate(columns as Column<unknown>[]);

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => rowHeight,
    overscan: 5,
  });

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (!onRowClick) return;
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
        if (row) onRowClick(row);
      }
    },
    [onRowClick, rows, virtualizer],
  );

  useEffect(() => {
    const el = parentRef.current;
    if (!el) return;
    el.addEventListener("keydown", handleKeyDown);
    return () => el.removeEventListener("keydown", handleKeyDown);
  }, [handleKeyDown]);

  const justifyFor = (align: Column<T>["align"]) =>
    align === "right" ? "flex-end" : align === "center" ? "center" : "flex-start";

  return (
    <div style={{ width: "100%" }}>
      {caption && (
        <div
          style={{
            padding: "4px 0",
            fontSize: 11,
            color: "var(--color-text-muted)",
          }}
        >
          {caption}
        </div>
      )}

      {/* ── Sticky header ────────────────────────────────────────────────────── */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: gridTemplate,
          borderBottom: "1px solid var(--color-border)",
          background: "var(--color-bg-panel)",
          position: "sticky",
          top: 0,
          zIndex: 1,
        }}
      >
        {columns.map((col) => (
          <div
            key={col.key}
            style={{
              padding: "6px 8px",
              display: "flex",
              alignItems: "center",
              justifyContent: justifyFor(col.align),
              fontWeight: 500,
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              fontSize: 10,
              color: "var(--color-text-muted)",
              whiteSpace: "nowrap",
              overflow: "hidden",
            }}
          >
            {col.header}
          </div>
        ))}
      </div>

      {/* ── Scrollable body ──────────────────────────────────────────────────── */}
      <div
        ref={parentRef}
        role="grid"
        tabIndex={0}
        style={{ overflowY: "auto", maxHeight, outline: "none" }}
      >
        {loading ? (
          Array.from({ length: 8 }, (_, i) => (
            <div
              key={`skeleton-${i}`}
              style={{
                display: "grid",
                gridTemplateColumns: gridTemplate,
                height: rowHeight,
                borderBottom: "1px solid var(--color-border)",
                alignItems: "center",
              }}
            >
              {columns.map((col) => (
                <div key={col.key} style={{ padding: "0 8px" }}>
                  <span className="skeleton" style={{ display: "block", height: 12 }} />
                </div>
              ))}
            </div>
          ))
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
              const key = rowKey(row);
              const isSelected = selectedKey === key;
              return (
                <div
                  key={key}
                  role="row"
                  tabIndex={onRowClick ? 0 : undefined}
                  onClick={() => onRowClick?.(row)}
                  onKeyDown={(e) => e.key === "Enter" && onRowClick?.(row)}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    transform: `translateY(${vi.start}px)`,
                    height: rowHeight,
                    display: "grid",
                    gridTemplateColumns: gridTemplate,
                    alignItems: "center",
                    cursor: onRowClick ? "pointer" : "default",
                    background: isSelected ? "rgba(59,130,246,0.08)" : "transparent",
                    borderBottom: "1px solid var(--color-border)",
                  }}
                >
                  {columns.map((col) => (
                    <div
                      key={col.key}
                      role="gridcell"
                      style={{
                        padding: "0 8px",
                        height: "100%",
                        display: "flex",
                        alignItems: "center",
                        justifyContent: justifyFor(col.align),
                        overflow: "hidden",
                        fontSize: 12,
                      }}
                    >
                      <span
                        style={{
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                          maxWidth: "100%",
                        }}
                      >
                        {col.render(row)}
                      </span>
                    </div>
                  ))}
                </div>
              );
            })}
          </div>
        )}

        {!loading && rows.length === 0 && (
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
