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

  return (
    <div
      ref={parentRef}
      role="grid"
      tabIndex={0}
      style={{
        overflowY: "auto",
        maxHeight,
        position: "relative",
        outline: "none",
      }}
    >
      <table
        style={{
          width: "100%",
          borderCollapse: "collapse",
          tableLayout: "fixed",
        }}
      >
        {caption && (
          <caption
            style={{
              captionSide: "top",
              textAlign: "left",
              padding: "4px 0",
              fontSize: 11,
              color: "var(--color-text-muted)",
            }}
          >
            {caption}
          </caption>
        )}
        <thead
          style={{
            position: "sticky",
            top: 0,
            zIndex: 1,
            background: "var(--color-bg-panel)",
          }}
        >
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                style={{
                  padding: "6px 8px",
                  textAlign: col.align ?? "left",
                  width: col.width,
                  minWidth: col.minWidth,
                  borderBottom: "1px solid var(--color-border)",
                  fontWeight: 500,
                  textTransform: "uppercase",
                  letterSpacing: "0.06em",
                  fontSize: 10,
                  color: "var(--color-text-muted)",
                  whiteSpace: "nowrap",
                }}
              >
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody
          style={{
            height: virtualizer.getTotalSize(),
            position: "relative",
          }}
        >
          {loading
            ? Array.from({ length: 8 }, (_, i) => (
                <tr key={`loading-row-${i}`} style={{ height: rowHeight }}>
                  {columns.map((col) => (
                    <td key={col.key} style={{ padding: "6px 8px" }}>
                      <span className="skeleton" style={{ display: "block", height: 12 }} />
                    </td>
                  ))}
                </tr>
              ))
            : virtualizer.getVirtualItems().map((vi) => {
                const row = rows[vi.index];
                if (!row) return null;
                const key = rowKey(row);
                const isSelected = selectedKey === key;
                return (
                  <tr
                    key={key}
                    onClick={() => onRowClick?.(row)}
                    onKeyDown={(e) => e.key === "Enter" && onRowClick?.(row)}
                    style={{
                      position: "absolute",
                      top: 0,
                      left: 0,
                      width: "100%",
                      transform: `translateY(${vi.start}px)`,
                      height: rowHeight,
                      cursor: onRowClick ? "pointer" : "default",
                      background: isSelected ? "rgba(59,130,246,0.08)" : "transparent",
                      borderBottom: "1px solid var(--color-border)",
                    }}
                  >
                    {columns.map((col) => (
                      <td
                        key={col.key}
                        style={{
                          padding: "0 8px",
                          textAlign: col.align ?? "left",
                          width: col.width,
                          height: rowHeight,
                          fontSize: 12,
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {col.render(row)}
                      </td>
                    ))}
                  </tr>
                );
              })}
        </tbody>
      </table>
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
  );
}
