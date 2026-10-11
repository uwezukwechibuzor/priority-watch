const LABEL_STYLE: React.CSSProperties = {
  fontSize: 10,
  textTransform: "uppercase",
  letterSpacing: "0.06em",
  color: "var(--color-text-muted)",
  display: "block",
  marginBottom: 4,
};

const INPUT_STYLE: React.CSSProperties = {
  width: "100%",
  background: "var(--color-bg-base)",
  border: "1px solid var(--color-border)",
  borderRadius: 3,
  color: "var(--color-text-primary)",
  fontSize: 12,
  padding: "4px 8px",
};

export type StrategyName = "fixed-tip" | "percentile-of-recent" | "spread-proportional";

export interface SimParams {
  strategy: StrategyName;
  tipGwei: number;
  percentile: number;
  alpha: number;
  fromBlock: number;
  toBlock: number;
}

interface SimulatorControlsProps {
  params: SimParams;
  onChange: (p: SimParams) => void;
  onRun: () => void;
  runCount: number;
  loading?: boolean;
}

const STRATEGY_OPTIONS: { value: StrategyName; label: string }[] = [
  { value: "percentile-of-recent", label: "Percentile of Recent" },
  { value: "fixed-tip", label: "Fixed Tip" },
  { value: "spread-proportional", label: "Spread Proportional" },
];

export function SimulatorControls({
  params,
  onChange,
  onRun,
  runCount,
  loading,
}: SimulatorControlsProps) {
  function set<K extends keyof SimParams>(key: K, val: SimParams[K]) {
    onChange({ ...params, [key]: val });
  }

  return (
    <div
      style={{
        width: 280,
        minWidth: 280,
        background: "var(--color-bg-panel)",
        borderRight: "1px solid var(--color-border)",
        padding: 16,
        display: "flex",
        flexDirection: "column",
        gap: 16,
      }}
    >
      <div>
        <label htmlFor="sc-strategy" style={LABEL_STYLE}>
          Strategy
        </label>
        <select
          id="sc-strategy"
          style={{ ...INPUT_STYLE, cursor: "pointer" }}
          value={params.strategy}
          onChange={(e) => set("strategy", e.target.value as StrategyName)}
        >
          {STRATEGY_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </div>

      {params.strategy === "fixed-tip" && (
        <div>
          <label htmlFor="sc-tip" style={LABEL_STYLE}>
            Tip (gwei): {params.tipGwei.toFixed(2)}
          </label>
          <input
            id="sc-tip"
            type="number"
            min={0.01}
            step={0.1}
            style={INPUT_STYLE}
            value={params.tipGwei}
            onChange={(e) => set("tipGwei", Number(e.target.value))}
          />
        </div>
      )}

      {params.strategy === "percentile-of-recent" && (
        <div>
          <label htmlFor="sc-percentile" style={LABEL_STYLE}>
            Percentile: {(params.percentile * 100).toFixed(0)}%
          </label>
          <input
            id="sc-percentile"
            type="range"
            min={0.5}
            max={0.99}
            step={0.01}
            style={{ width: "100%" }}
            value={params.percentile}
            onChange={(e) => set("percentile", Number(e.target.value))}
          />
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              fontSize: 10,
              color: "var(--color-text-muted)",
              marginTop: 2,
            }}
          >
            <span>50%</span>
            <span>99%</span>
          </div>
        </div>
      )}

      {params.strategy === "spread-proportional" && (
        <div>
          <label htmlFor="sc-alpha" style={LABEL_STYLE}>
            Alpha: {params.alpha.toFixed(2)}
          </label>
          <input
            id="sc-alpha"
            type="range"
            min={0.1}
            max={1.0}
            step={0.05}
            style={{ width: "100%" }}
            value={params.alpha}
            onChange={(e) => set("alpha", Number(e.target.value))}
          />
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              fontSize: 10,
              color: "var(--color-text-muted)",
              marginTop: 2,
            }}
          >
            <span>0.1</span>
            <span>1.0</span>
          </div>
        </div>
      )}

      <div>
        <label htmlFor="sc-from" style={LABEL_STYLE}>
          From Block (0 = latest-200)
        </label>
        <input
          id="sc-from"
          type="number"
          min={0}
          style={INPUT_STYLE}
          value={params.fromBlock}
          onChange={(e) => set("fromBlock", Number(e.target.value))}
        />
      </div>

      <div>
        <label htmlFor="sc-to" style={LABEL_STYLE}>
          To Block (0 = latest)
        </label>
        <input
          id="sc-to"
          type="number"
          min={0}
          style={INPUT_STYLE}
          value={params.toBlock}
          onChange={(e) => set("toBlock", Number(e.target.value))}
        />
      </div>

      <button
        type="button"
        onClick={onRun}
        disabled={loading || runCount >= 4}
        style={{
          marginTop: 8,
          padding: "8px 0",
          background: runCount >= 4 ? "var(--color-border)" : "var(--color-accent)",
          color: "#fff",
          border: "none",
          borderRadius: 4,
          fontWeight: 600,
          fontSize: 12,
          cursor: loading || runCount >= 4 ? "not-allowed" : "pointer",
          opacity: loading || runCount >= 4 ? 0.6 : 1,
          textTransform: "uppercase",
          letterSpacing: "0.06em",
        }}
      >
        {runCount >= 4 ? "Max 4 runs" : "Run Backtest"}
      </button>

      {runCount >= 4 && (
        <p style={{ fontSize: 11, color: "var(--color-text-muted)", textAlign: "center" }}>
          Clear a run to add another
        </p>
      )}
    </div>
  );
}
