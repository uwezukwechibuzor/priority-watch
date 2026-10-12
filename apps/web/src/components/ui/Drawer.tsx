import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  children: ReactNode;
  width?: number;
}

export function Drawer({ open, onClose, title, children, width: defaultWidth = 480 }: DrawerProps) {
  const [width, setWidth] = useState(defaultWidth);
  const dragging = useRef(false);
  const startX = useRef(0);
  const startW = useRef(0);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  // Reset width when drawer closes
  useEffect(() => {
    if (!open) setWidth(defaultWidth);
  }, [open, defaultWidth]);

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    dragging.current = true;
    startX.current = e.clientX;
    startW.current = width;
    e.preventDefault();
  }, [width]);

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!dragging.current) return;
      const delta = startX.current - e.clientX;
      const next = Math.min(Math.max(startW.current + delta, 360), window.innerWidth * 0.9);
      setWidth(next);
    };
    const onUp = () => { dragging.current = false; };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    return () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
  }, []);

  return (
    <>
      {/* Backdrop */}
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: backdrop is aria-hidden, keyboard handled by Escape listener */}
      <div
        aria-hidden="true"
        onClick={onClose}
        style={{
          position: "fixed",
          inset: 0,
          background: "rgba(0,0,0,0.5)",
          zIndex: 40,
          opacity: open ? 1 : 0,
          pointerEvents: open ? "auto" : "none",
          transition: "opacity 0.2s",
        }}
      />
      {/* Panel */}
      {/* biome-ignore lint/a11y/useSemanticElements: <dialog> requires imperative .show()/.showModal() which conflicts with CSS transition approach */}
      <aside
        role="dialog"
        aria-modal="true"
        aria-label={title ?? "Details"}
        style={{
          position: "fixed",
          top: 0,
          right: 0,
          bottom: 0,
          width,
          background: "var(--color-bg-panel)",
          borderLeft: "1px solid var(--color-border)",
          zIndex: 50,
          transform: open ? "translateX(0)" : `translateX(${width}px)`,
          transition: dragging.current ? "none" : "transform 0.25s ease",
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        {/* Drag handle */}
        {/* biome-ignore lint/a11y/useKeyWithMouseEvents: resize handle is purely presentational */}
        <div
          onMouseDown={onMouseDown}
          style={{
            position: "absolute",
            left: 0,
            top: 0,
            bottom: 0,
            width: 4,
            cursor: "ew-resize",
            zIndex: 10,
          }}
        />
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            padding: "12px 16px",
            borderBottom: "1px solid var(--color-border)",
            flexShrink: 0,
          }}
        >
          {title && (
            <span className="label" style={{ fontSize: 11 }}>
              {title}
            </span>
          )}
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            style={{
              marginLeft: "auto",
              background: "none",
              border: "none",
              color: "var(--color-text-muted)",
              cursor: "pointer",
              fontSize: 18,
              lineHeight: 1,
              padding: 2,
            }}
          >
            ×
          </button>
        </div>
        <div style={{ flex: 1, overflow: "auto", padding: 16 }}>{children}</div>
      </aside>
    </>
  );
}
