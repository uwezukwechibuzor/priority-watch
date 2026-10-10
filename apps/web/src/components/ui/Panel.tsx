import type { ReactNode } from "react";

interface PanelProps {
  title?: string;
  children: ReactNode;
  className?: string;
  action?: ReactNode;
}

export function Panel({ title, children, className = "", action }: PanelProps) {
  return (
    <section
      className={className}
      style={{
        background: "var(--color-bg-panel)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-panel)",
        overflow: "hidden",
      }}
    >
      {title && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            padding: "10px var(--spacing-panel)",
            borderBottom: "1px solid var(--color-border)",
          }}
        >
          <span className="label" style={{ fontSize: 11 }}>
            {title}
          </span>
          {action && <div>{action}</div>}
        </div>
      )}
      <div style={{ padding: "var(--spacing-panel)" }}>{children}</div>
    </section>
  );
}
