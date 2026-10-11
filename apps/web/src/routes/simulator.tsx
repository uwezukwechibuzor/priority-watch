import type { ReplayResult } from "@/api/types";
import type { SimRun } from "@/components/simulator/PnlChart";
import { PnlChart } from "@/components/simulator/PnlChart";
import { RunSummaryCards } from "@/components/simulator/RunSummaryCards";
import { SimulatorControls } from "@/components/simulator/SimulatorControls";
import type { SimParams } from "@/components/simulator/SimulatorControls";
import { simulatorSearchSchema } from "@/lib/search-params";
import { createBacktest, getBacktestResults, getBacktestRun } from "@/queries/pricing";
import { rootRoute } from "@/router";
import { createRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useRef, useState } from "react";

export const simulatorRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/simulator",
  validateSearch: (s) => simulatorSearchSchema.parse(s),
  component: SimulatorPage,
});

const RUN_COLORS = ["#3b82f6", "#f59e0b", "#22c55e", "#ef4444"];
const POLL_INTERVAL_MS = 1500;

function makeLabel(params: SimParams): string {
  switch (params.strategy) {
    case "fixed-tip":
      return `fixed@${params.tipGwei.toFixed(1)}gwei`;
    case "percentile-of-recent":
      return `p${(params.percentile * 100).toFixed(0)}`;
    case "spread-proportional":
      return `spread-α${params.alpha.toFixed(2)}`;
  }
}

function buildConfig(params: SimParams): Record<string, unknown> {
  switch (params.strategy) {
    case "fixed-tip":
      return { tip_gwei: params.tipGwei };
    case "percentile-of-recent":
      return { percentile: params.percentile };
    case "spread-proportional":
      return { alpha: params.alpha };
  }
}

function SimulatorPage() {
  const [params, setParams] = useState<SimParams>({
    strategy: "percentile-of-recent",
    tipGwei: 1.0,
    percentile: 0.8,
    alpha: 0.5,
    fromBlock: 0,
    toBlock: 0,
  });

  const [runs, setRuns] = useState<SimRun[]>([]);
  const [loading, setLoading] = useState(false);
  const pollTimers = useRef<Map<string, ReturnType<typeof setInterval>>>(new Map());

  const stopPolling = useCallback((runId: string) => {
    const t = pollTimers.current.get(runId);
    if (t) {
      clearInterval(t);
      pollTimers.current.delete(runId);
    }
  }, []);

  useEffect(() => {
    return () => {
      for (const t of pollTimers.current.values()) clearInterval(t);
    };
  }, []);

  async function handleRun() {
    if (runs.length >= 4) return;
    setLoading(true);
    const colorIndex = runs.length % RUN_COLORS.length;
    try {
      const { run_id } = await createBacktest({
        strategy: params.strategy,
        from_block: params.fromBlock,
        to_block: params.toBlock,
        config: buildConfig(params),
      });

      const newRun: SimRun = {
        id: run_id,
        label: makeLabel(params),
        colorIndex,
        status: "pending",
      };
      setRuns((prev) => [...prev, newRun]);

      const timer = setInterval(async () => {
        try {
          const status = await getBacktestRun(run_id);
          if (status.status === "done") {
            stopPolling(run_id);
            try {
              const result = await getBacktestResults(run_id);
              setRuns((prev) =>
                prev.map((r) =>
                  r.id === run_id ? { ...r, status: "done", result: result as ReplayResult } : r,
                ),
              );
            } catch {
              setRuns((prev) =>
                prev.map((r) =>
                  r.id === run_id
                    ? { ...r, status: "error", errorMsg: "Failed to fetch results" }
                    : r,
                ),
              );
            }
          } else if (status.status === "error") {
            stopPolling(run_id);
            setRuns((prev) =>
              prev.map((r) =>
                r.id === run_id
                  ? { ...r, status: "error", errorMsg: status.error_msg ?? "Unknown error" }
                  : r,
              ),
            );
          } else {
            setRuns((prev) =>
              prev.map((r) =>
                r.id === run_id ? { ...r, status: status.status as SimRun["status"] } : r,
              ),
            );
          }
        } catch {
          stopPolling(run_id);
          setRuns((prev) =>
            prev.map((r) =>
              r.id === run_id ? { ...r, status: "error", errorMsg: "Polling failed" } : r,
            ),
          );
        }
      }, POLL_INTERVAL_MS);
      pollTimers.current.set(run_id, timer);
    } catch {
      // ignore submit error — user can retry
    } finally {
      setLoading(false);
    }
  }

  function handleCancel(id: string) {
    stopPolling(id);
    setRuns((prev) => prev.filter((r) => r.id !== id));
  }

  const disclaimer = runs.find((r) => r.result?.disclaimer)?.result?.disclaimer;

  return (
    <div style={{ display: "flex", height: "calc(100vh - 84px)", overflow: "hidden" }}>
      <SimulatorControls
        params={params}
        onChange={setParams}
        onRun={() => {
          void handleRun();
        }}
        runCount={runs.length}
        loading={loading}
      />

      <div
        style={{
          flex: 1,
          overflowY: "auto",
          padding: 16,
          display: "flex",
          flexDirection: "column",
          gap: 8,
        }}
      >
        <PnlChart runs={runs} />
        <RunSummaryCards runs={runs} onCancel={handleCancel} />

        {disclaimer && (
          <div
            style={{
              background: "rgba(107,114,128,0.08)",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-panel)",
              padding: "10px 16px",
              fontSize: 11,
              color: "var(--color-text-muted)",
              lineHeight: 1.6,
            }}
          >
            <strong style={{ color: "var(--color-text-primary)" }}>Disclaimer:</strong> {disclaimer}
          </div>
        )}
      </div>
    </div>
  );
}
