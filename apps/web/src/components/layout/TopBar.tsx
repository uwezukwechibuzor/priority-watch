import { LivePill } from "@/components/ui/LivePill";
import { Link, useRouterState } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { CommandPalette } from "./CommandPalette";

const TABS = [
  { label: "Blocks", to: "/blocks" },
  { label: "Health", to: "/health" },
  { label: "Pricing", to: "/pricing" },
  { label: "Opportunity", to: "/opportunity" },
  { label: "Simulator", to: "/simulator" },
] as const;

export function TopBar() {
  const [paletteOpen, setPaletteOpen] = useState(false);
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  // Global Cmd/Ctrl+K shortcut
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setPaletteOpen((o) => !o);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  return (
    <>
      <header
        style={{
          position: "fixed",
          top: 0,
          left: 0,
          right: 0,
          height: 48,
          background: "var(--color-bg-panel)",
          borderBottom: "1px solid var(--color-border)",
          display: "flex",
          alignItems: "center",
          gap: 0,
          zIndex: 30,
          paddingLeft: 16,
          paddingRight: 12,
        }}
      >
        {/* Wordmark + badge */}
        <Link
          to="/blocks"
          style={{
            display: "flex",
            alignItems: "center",
            gap: 7,
            textDecoration: "none",
            marginRight: 24,
            flexShrink: 0,
          }}
        >
          <span
            style={{
              fontSize: 14,
              fontWeight: 600,
              color: "var(--color-text-primary)",
              letterSpacing: "-0.01em",
            }}
          >
            priority watch
          </span>
          <span
            style={{
              padding: "1px 6px",
              background: "var(--color-bg-base)",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-badge)",
              fontSize: 10,
              fontWeight: 600,
              color: "var(--color-text-muted)",
              letterSpacing: "0.06em",
              textTransform: "uppercase",
            }}
          >
            PGA
          </span>
        </Link>

        {/* Navigation tabs */}
        <nav
          style={{ display: "flex", alignItems: "center", height: "100%", gap: 0, flex: 1 }}
          aria-label="Main navigation"
        >
          {TABS.map((tab) => {
            const active = currentPath === tab.to || currentPath.startsWith(`${tab.to}/`);
            return (
              <Link
                key={tab.to}
                to={tab.to}
                style={{
                  display: "flex",
                  alignItems: "center",
                  height: "100%",
                  padding: "0 14px",
                  fontSize: 13,
                  fontWeight: active ? 500 : 400,
                  color: active ? "var(--color-text-primary)" : "var(--color-text-muted)",
                  textDecoration: "none",
                  borderBottom: active ? "2px solid var(--color-accent)" : "2px solid transparent",
                  transition: "color 0.1s, border-color 0.1s",
                  whiteSpace: "nowrap",
                }}
                aria-current={active ? "page" : undefined}
              >
                {tab.label}
              </Link>
            );
          })}
        </nav>

        {/* Right side: search trigger + live pill + icons */}
        <div style={{ display: "flex", alignItems: "center", gap: 8, flexShrink: 0 }}>
          <button
            type="button"
            onClick={() => setPaletteOpen(true)}
            aria-label="Search (⌘K)"
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              padding: "4px 10px",
              background: "var(--color-bg-base)",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-badge)",
              color: "var(--color-text-muted)",
              fontSize: 12,
              cursor: "pointer",
              width: 220,
              justifyContent: "space-between",
            }}
          >
            <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <svg aria-hidden="true" width="12" height="12" viewBox="0 0 16 16" fill="none">
                <circle cx="7" cy="7" r="5" stroke="currentColor" strokeWidth="1.5" />
                <path
                  d="M11 11l3 3"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                />
              </svg>
              Search blocks, txs…
            </span>
            <kbd
              style={{
                padding: "1px 4px",
                background: "var(--color-bg-panel)",
                border: "1px solid var(--color-border)",
                borderRadius: 3,
                fontSize: 10,
              }}
            >
              ⌘K
            </kbd>
          </button>

          <LivePill />

          <button
            type="button"
            aria-label="Settings"
            style={{
              background: "none",
              border: "none",
              color: "var(--color-text-muted)",
              cursor: "pointer",
              padding: 4,
              borderRadius: 4,
              fontSize: 16,
              lineHeight: 1,
            }}
          >
            ⚙
          </button>
          <button
            type="button"
            aria-label="Help"
            style={{
              background: "none",
              border: "none",
              color: "var(--color-text-muted)",
              cursor: "pointer",
              padding: 4,
              borderRadius: 4,
              fontSize: 16,
              lineHeight: 1,
            }}
          >
            ?
          </button>
        </div>
      </header>

      <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} />
    </>
  );
}
