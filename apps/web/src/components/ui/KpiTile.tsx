import { Skeleton } from "./Skeleton";

interface KpiTileProps {
  label: string;
  value?: string | number;
  unit?: string;
  subtext?: string;
  subtextColor?: string;
  loading?: boolean;
}

export function KpiTile({
  label,
  value,
  unit,
  subtext,
  subtextColor,
  loading = false,
}: KpiTileProps) {
  return (
    <div
      style={{
        background: "var(--color-bg-panel)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-panel)",
        padding: "14px var(--spacing-panel)",
        display: "flex",
        flexDirection: "column",
        gap: 4,
        minWidth: 0,
      }}
    >
      <span className="label">{label}</span>
      {loading ? (
        <>
          <Skeleton width={100} height={28} />
          <Skeleton width={80} height={12} />
        </>
      ) : (
        <>
          <div
            className="tabular"
            style={{
              fontSize: 24,
              fontWeight: 600,
              color: "var(--color-text-primary)",
              lineHeight: 1.2,
              display: "flex",
              alignItems: "baseline",
              gap: 4,
            }}
          >
            <span>{value ?? "—"}</span>
            {unit && (
              <span
                style={{
                  fontSize: 13,
                  fontWeight: 400,
                  color: "var(--color-text-muted)",
                }}
              >
                {unit}
              </span>
            )}
          </div>
          {subtext && (
            <span
              style={{
                fontSize: 11,
                color: subtextColor ?? "var(--color-text-muted)",
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {subtext}
            </span>
          )}
        </>
      )}
    </div>
  );
}
