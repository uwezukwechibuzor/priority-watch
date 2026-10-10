import { Panel } from "@/components/ui";
import { simulatorSearchSchema } from "@/lib/search-params";
import { rootRoute } from "@/router";
import { createRoute } from "@tanstack/react-router";

export const simulatorRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/simulator",
  validateSearch: (s) => simulatorSearchSchema.parse(s),
  component: SimulatorPage,
});

function SimulatorPage() {
  return (
    <div style={{ padding: 16 }}>
      <Panel title="SIMULATOR">
        <p style={{ color: "var(--color-text-muted)", fontSize: 12 }}>
          Strategy simulator — coming in task 19.
        </p>
      </Panel>
    </div>
  );
}
