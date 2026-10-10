import { relativeTime } from "@/lib/formatters";
import { ingesterStatusQuery } from "@/queries/health";
import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";

interface ContextBarProps {
  day?: string;
  onDayChange?: (day: string) => void;
  extraRight?: ReactNode;
}

function pad(n: number): string {
  return n.toString().padStart(2, "0");
}

function today(): string {
  const d = new Date();
  return `${pad(d.getMonth() + 1)}/${pad(d.getDate())}/${d.getFullYear()}`;
}

function prevDay(dayStr: string): string {
  const parts = dayStr.split("/");
  if (parts.length !== 3) return dayStr;
  const d = new Date(Number(parts[2]), Number(parts[0]) - 1, Number(parts[1]));
  d.setDate(d.getDate() - 1);
  return `${pad(d.getMonth() + 1)}/${pad(d.getDate())}/${d.getFullYear()}`;
}

function nextDay(dayStr: string): string {
  const parts = dayStr.split("/");
  if (parts.length !== 3) return dayStr;
  const d = new Date(Number(parts[2]), Number(parts[0]) - 1, Number(parts[1]));
  d.setDate(d.getDate() + 1);
  return `${pad(d.getMonth() + 1)}/${pad(d.getDate())}/${d.getFullYear()}`;
}

export function ContextBar({ day, onDayChange, extraRight }: ContextBarProps) {
  const currentDay = day ?? today();
  const { data: sources } = useQuery(ingesterStatusQuery);
  const chainSource = sources?.find((s) => s.source === "chain");
  const lagText = chainSource?.latest_timestamp ? relativeTime(chainSource.latest_timestamp) : "—";
  const isStale = chainSource ? chainSource.lag_seconds > 2 : false;

  const apiBase = import.meta.env.VITE_API_BASE ?? window.location.origin;
  const sourceLabel = apiBase.replace(/^https?:\/\//, "").replace(/:\d+$/, "") || "localhost";

  return (
    <div
      style={{
        position: "fixed",
        top: 48,
        left: 0,
        right: 0,
        height: 36,
        background: "var(--color-bg-base)",
        borderBottom: "1px solid var(--color-border)",
        display: "flex",
        alignItems: "center",
        gap: 20,
        paddingLeft: 16,
        paddingRight: 16,
        zIndex: 25,
        fontSize: 11,
        color: "var(--color-text-muted)",
        overflowX: "auto",
        whiteSpace: "nowrap",
      }}
      aria-label="Context bar"
    >
      {/* Day picker */}
      <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
        <button
          type="button"
          aria-label="Previous day"
          onClick={() => onDayChange?.(prevDay(currentDay))}
          style={arrowBtnStyle}
        >
          ‹
        </button>
        <span style={{ fontVariantNumeric: "tabular-nums", color: "var(--color-text-primary)" }}>
          {currentDay}
        </span>
        <button
          type="button"
          aria-label="Next day"
          onClick={() => onDayChange?.(nextDay(currentDay))}
          style={arrowBtnStyle}
        >
          ›
        </button>
      </div>

      <Divider />

      <ContextItem label="Source" value={sourceLabel} />
      <ContextItem label="Network" value="Arbitrum One" />
      <ContextItem
        label="Live data"
        value={lagText}
        valueColor={isStale ? "var(--color-red)" : undefined}
      />
      <ContextItem label="Block time" value="250ms" />
      <ContextItem label="PGA round" value="125ms" />

      {extraRight && (
        <>
          <div style={{ marginLeft: "auto" }} />
          {extraRight}
        </>
      )}
    </div>
  );
}

const arrowBtnStyle: React.CSSProperties = {
  background: "none",
  border: "none",
  color: "var(--color-text-muted)",
  cursor: "pointer",
  padding: "0 2px",
  fontSize: 13,
  lineHeight: 1,
};

function Divider() {
  return (
    <span
      style={{
        width: 1,
        height: 14,
        background: "var(--color-border)",
        display: "inline-block",
        flexShrink: 0,
      }}
    />
  );
}

function ContextItem({
  label,
  value,
  valueColor,
}: {
  label: string;
  value: string;
  valueColor?: string;
}) {
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 5 }}>
      <span className="label" style={{ fontSize: 10 }}>
        {label}
      </span>
      <span
        className="tabular"
        style={{ color: valueColor ?? "var(--color-text-primary)", fontSize: 11 }}
      >
        {value}
      </span>
    </span>
  );
}
