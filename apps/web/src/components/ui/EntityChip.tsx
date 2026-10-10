import { entityColor, shortAddress } from "@/lib/formatters";

interface EntityChipProps {
  name: string;
  colorIndex?: number;
  address?: string;
  size?: number;
}

export function EntityChip({ name, colorIndex, address, size = 8 }: EntityChipProps) {
  const isAddress = !name || name === address;
  const displayName = isAddress ? shortAddress(address ?? name) : name;
  const color =
    colorIndex !== undefined ? entityColor(colorIndex) : entityColor(Math.abs(hashStr(name)) % 8);

  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 5,
        maxWidth: "100%",
      }}
    >
      <span
        style={{
          width: size,
          height: size,
          borderRadius: 1,
          background: color,
          flexShrink: 0,
        }}
        aria-hidden="true"
      />
      <span
        className={isAddress ? "mono" : ""}
        style={{
          color: "var(--color-text-primary)",
          fontSize: 12,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        {displayName}
      </span>
    </span>
  );
}

function hashStr(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) {
    h = (h * 31 + s.charCodeAt(i)) >>> 0;
  }
  return h;
}
