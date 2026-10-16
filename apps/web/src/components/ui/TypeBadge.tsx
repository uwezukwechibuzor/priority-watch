type BlockType = "contested" | "single" | "untipped";

const BADGE_STYLE: Record<string, { bg: string; color: string }> = {
  contested: { bg: "rgba(168,85,247,0.15)", color: "var(--color-purple)" },
  single: { bg: "rgba(59,130,246,0.15)", color: "var(--color-accent)" },
  untipped: { bg: "rgba(107,114,128,0.15)", color: "var(--color-text-muted)" },
};

const FALLBACK = { bg: "rgba(107,114,128,0.15)", color: "var(--color-text-muted)" };

export function TypeBadge({ type }: { type: BlockType | string }) {
  const s = BADGE_STYLE[type] ?? FALLBACK;
  return (
    <span
      style={{
        padding: "1px 6px",
        borderRadius: "var(--radius-badge)",
        fontSize: 11,
        fontWeight: 500,
        background: s.bg,
        color: s.color,
        whiteSpace: "nowrap",
      }}
    >
      {type}
    </span>
  );
}
