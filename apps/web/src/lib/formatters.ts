// ── Number formatters ─────────────────────────────────────────────────────────

const ETH_DECIMALS = 1e18;
const GWEI_DECIMALS = 1e9;

/** Format a bigint/string wei value as ETH with up to 6 significant digits. */
export function weiToEth(wei: string | number | bigint): string {
  const n = typeof wei === "bigint" ? wei : BigInt(String(wei));
  const eth = Number(n) / ETH_DECIMALS;
  return formatEth(eth);
}

export function formatEth(eth: number, fractionDigits = 6): string {
  if (eth === 0) return "0 ETH";
  if (eth < 0.000001) return `${(eth * ETH_DECIMALS).toFixed(2)} gwei`;
  return `${eth.toFixed(fractionDigits)} ETH`;
}

export function formatGwei(gwei: number): string {
  if (gwei < 0.001) return `${(gwei * GWEI_DECIMALS).toFixed(0)} mwei`;
  if (gwei < 1) return `${gwei.toFixed(4)} gwei`;
  return `${gwei.toFixed(2)} gwei`;
}

export function formatUsd(usd: number): string {
  if (usd >= 1_000_000) return `$${(usd / 1_000_000).toFixed(2)}M`;
  if (usd >= 1_000) return `$${(usd / 1_000).toFixed(2)}K`;
  if (usd >= 1) return `$${usd.toFixed(2)}`;
  return "<$0.01";
}

/** Compact number: 1234 → "1.2K", 1234567 → "1.2M" */
export function compact(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toLocaleString();
}

export function formatPct(frac: number, decimals = 1): string {
  return `${(frac * 100).toFixed(decimals)}%`;
}

export function formatHhi(hhi: number): string {
  return hhi.toFixed(0).replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

// ── Address formatters ────────────────────────────────────────────────────────

export function shortAddress(addr: string): string {
  if (!addr || addr.length < 10) return addr;
  return `${addr.slice(0, 6)}…${addr.slice(-4)}`;
}

export function arbiscanTx(hash: string): string {
  return `https://arbiscan.io/tx/${hash}`;
}

export function arbiscanBlock(n: number | string): string {
  return `https://arbiscan.io/block/${n}`;
}

export function arbiscanAddress(addr: string): string {
  return `https://arbiscan.io/address/${addr}`;
}

// ── Time formatters ───────────────────────────────────────────────────────────

/** "5s ago", "2m ago", "3h ago", "4d ago" */
export function relativeTime(isoOrMs: string | number): string {
  const ts = typeof isoOrMs === "number" ? isoOrMs : Date.parse(isoOrMs);
  const diffMs = Date.now() - ts;
  const diffS = Math.floor(diffMs / 1000);
  if (diffS < 60) return `${diffS}s ago`;
  const diffM = Math.floor(diffS / 60);
  if (diffM < 60) return `${diffM}m ago`;
  const diffH = Math.floor(diffM / 60);
  if (diffH < 24) return `${diffH}h ago`;
  return `${Math.floor(diffH / 24)}d ago`;
}

export function formatTime(isoOrMs: string | number): string {
  const d = typeof isoOrMs === "number" ? new Date(isoOrMs) : new Date(isoOrMs);
  return d.toLocaleTimeString("en-US", { hour12: false });
}

export function formatDate(isoOrMs: string | number): string {
  const d = typeof isoOrMs === "number" ? new Date(isoOrMs) : new Date(isoOrMs);
  return d.toLocaleDateString("en-US", { month: "2-digit", day: "2-digit", year: "numeric" });
}

// ── HHI band ─────────────────────────────────────────────────────────────────

export type HhiBand = "competitive" | "moderate" | "high";

export function hhiBand(hhi: number): HhiBand {
  if (hhi < 1500) return "competitive";
  if (hhi < 2500) return "moderate";
  return "high";
}

export const HHI_BAND_COLOR: Record<HhiBand, string> = {
  competitive: "var(--color-green)",
  moderate: "var(--color-amber)",
  high: "var(--color-red)",
};

// ── Entity palette ────────────────────────────────────────────────────────────

const ENTITY_COLORS = [
  "#3b82f6",
  "#f59e0b",
  "#a855f7",
  "#22c55e",
  "#ef4444",
  "#ec4899",
  "#14b8a6",
  "#f97316",
] as const;

export function entityColor(colorIndex: number): string {
  return ENTITY_COLORS[colorIndex % ENTITY_COLORS.length] ?? ENTITY_COLORS[0];
}

export function entityColorByName(name: string): string {
  // Stable hash to pick a colour index for unlabelled addresses.
  let h = 0;
  for (let i = 0; i < name.length; i++) {
    h = (h * 31 + name.charCodeAt(i)) >>> 0;
  }
  return ENTITY_COLORS[h % ENTITY_COLORS.length] ?? ENTITY_COLORS[0];
}

// ── Ratio bar band ────────────────────────────────────────────────────────────

export type RatioBand = "low" | "mid" | "high";

export function ratioBand(ratio: number): RatioBand {
  if (ratio < 0.5) return "low";
  if (ratio < 0.85) return "mid";
  return "high";
}

export const RATIO_BAND_COLOR: Record<RatioBand, string> = {
  low: "var(--color-green)",
  mid: "var(--color-amber)",
  high: "var(--color-red)",
};
