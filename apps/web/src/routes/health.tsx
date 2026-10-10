import { Panel } from "@/components/ui";
import { healthSearchSchema } from "@/lib/search-params";
import { rootRoute } from "@/router";
import { createRoute } from "@tanstack/react-router";

export const healthRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/health",
  validateSearch: (s) => healthSearchSchema.parse(s),
  component: HealthPage,
});

function HealthPage() {
  return (
    <div style={{ padding: 16 }}>
      <Panel title="HEALTH">
        <p style={{ color: "var(--color-text-muted)", fontSize: 12 }}>
          Health dashboard — coming in task 18.
        </p>
      </Panel>
    </div>
  );
}
