interface ErrorBannerProps {
  message: string;
  onRetry?: () => void;
}

export function ErrorBanner({ message, onRetry }: ErrorBannerProps) {
  return (
    <div
      role="alert"
      style={{
        display: "flex",
        alignItems: "center",
        gap: 8,
        padding: "8px 12px",
        background: "rgba(239,68,68,0.1)",
        border: "1px solid var(--color-red)",
        borderRadius: "var(--radius-badge)",
        color: "var(--color-red)",
        fontSize: 12,
      }}
    >
      <span>⚠ Failed to load — {message}</span>
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          style={{
            marginLeft: "auto",
            padding: "2px 8px",
            background: "transparent",
            border: "1px solid var(--color-red)",
            borderRadius: 3,
            color: "var(--color-red)",
            cursor: "pointer",
            fontSize: 11,
          }}
        >
          Retry
        </button>
      )}
    </div>
  );
}
