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

export interface PricingParams {
  block: number;
  model: string;
  n: number;
  confidence: number;
  window: number;
}

interface PricingControlsProps {
  params: PricingParams;
  onChange: (p: PricingParams) => void;
  onSubmit: () => void;
  loading?: boolean;
}

const MODEL_OPTIONS = [
  { value: "all", label: "All Models" },
  { value: "floor-estimate", label: "Floor Estimate" },
  { value: "recent-performance", label: "Recent Performance" },
  { value: "competitor-behavior", label: "Competitor Behavior" },
  { value: "market-condition", label: "Market Condition" },
];

export function PricingControls({ params, onChange, onSubmit, loading }: PricingControlsProps) {
  function set<K extends keyof PricingParams>(key: K, val: PricingParams[K]) {
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
        <label htmlFor="pc-block" style={LABEL_STYLE}>
          Block (0 = latest)
        </label>
        <input
          id="pc-block"
          type="number"
          min={0}
          style={INPUT_STYLE}
          value={params.block}
          onChange={(e) => set("block", Number(e.target.value))}
        />
      </div>

      <div>
        <label htmlFor="pc-model" style={LABEL_STYLE}>
          Model
        </label>
        <select
          id="pc-model"
          style={{ ...INPUT_STYLE, cursor: "pointer" }}
          value={params.model}
          onChange={(e) => set("model", e.target.value)}
        >
          {MODEL_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </div>

      <div>
        <label htmlFor="pc-n" style={LABEL_STYLE}>
          Target Rank — N: {params.n}
        </label>
        <input
          id="pc-n"
          type="range"
          min={1}
          max={10}
          step={1}
          style={{ width: "100%" }}
          value={params.n}
          onChange={(e) => set("n", Number(e.target.value))}
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
          <span>1</span>
          <span>10</span>
        </div>
      </div>

      <div>
        <label htmlFor="pc-confidence" style={LABEL_STYLE}>
          Confidence: {(params.confidence * 100).toFixed(0)}%
        </label>
        <input
          id="pc-confidence"
          type="range"
          min={0.5}
          max={0.99}
          step={0.01}
          style={{ width: "100%" }}
          value={params.confidence}
          onChange={(e) => set("confidence", Number(e.target.value))}
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

      <div>
        <label htmlFor="pc-window" style={LABEL_STYLE}>
          Lookback Window: {params.window} blocks
        </label>
        <input
          id="pc-window"
          type="range"
          min={50}
          max={500}
          step={50}
          style={{ width: "100%" }}
          value={params.window}
          onChange={(e) => set("window", Number(e.target.value))}
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
          <span>50</span>
          <span>500</span>
        </div>
      </div>

      <button
        type="button"
        onClick={onSubmit}
        disabled={loading}
        style={{
          marginTop: 8,
          padding: "8px 0",
          background: "var(--color-accent)",
          color: "#fff",
          border: "none",
          borderRadius: 4,
          fontWeight: 600,
          fontSize: 12,
          cursor: loading ? "not-allowed" : "pointer",
          opacity: loading ? 0.7 : 1,
          textTransform: "uppercase",
          letterSpacing: "0.06em",
        }}
      >
        {loading ? "Loading…" : "Get Recommendation"}
      </button>
    </div>
  );
}
