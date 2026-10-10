interface FilterInputProps {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  width?: string | number;
}

export function FilterInput({
  value,
  onChange,
  placeholder = "Filter…",
  width = 200,
}: FilterInputProps) {
  return (
    <div style={{ position: "relative", width }}>
      <svg
        aria-hidden="true"
        style={{
          position: "absolute",
          left: 8,
          top: "50%",
          transform: "translateY(-50%)",
          color: "var(--color-text-muted)",
          pointerEvents: "none",
        }}
        width="12"
        height="12"
        viewBox="0 0 16 16"
        fill="none"
      >
        <circle cx="7" cy="7" r="5" stroke="currentColor" strokeWidth="1.5" />
        <path d="M11 11l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        style={{
          width: "100%",
          padding: "4px 8px 4px 26px",
          background: "var(--color-bg-panel)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-badge)",
          color: "var(--color-text-primary)",
          fontSize: 12,
          outline: "none",
        }}
      />
    </div>
  );
}
