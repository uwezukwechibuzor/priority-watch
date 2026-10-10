import { relativeTime } from "@/lib/formatters";
import { ingesterStatusQuery } from "@/queries/health";
import { useQuery } from "@tanstack/react-query";

export function LivePill() {
  const { data, isError } = useQuery(ingesterStatusQuery);

  // Pick the chain source as the primary live indicator
  const chainSource = data?.find((r) => r.source === "chain") ?? data?.[0];
  const isLive = chainSource?.status === "live";
  const block = chainSource?.latest_position ?? "—";
  const lag = chainSource?.latest_timestamp ? relativeTime(chainSource.latest_timestamp) : null;

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 6,
        padding: "3px 8px",
        background: "var(--color-bg-panel)",
        border: "1px solid var(--color-border)",
        borderRadius: 12,
        fontSize: 11,
        color: isError
          ? "var(--color-text-muted)"
          : isLive
            ? "var(--color-text-primary)"
            : "var(--color-amber)",
      }}
      title={
        chainSource
          ? `Source: ${chainSource.source} · Lag: ${chainSource.lag_seconds.toFixed(1)}s`
          : "No data"
      }
    >
      <span
        className="pulse-dot"
        style={{
          width: 6,
          height: 6,
          borderRadius: "50%",
          background: isError
            ? "var(--color-text-muted)"
            : isLive
              ? "var(--color-green)"
              : "var(--color-amber)",
          flexShrink: 0,
        }}
        aria-hidden="true"
      />
      <span className="tabular">#{block}</span>
      {lag && <span style={{ color: "var(--color-text-muted)" }}>· {lag}</span>}
    </div>
  );
}
