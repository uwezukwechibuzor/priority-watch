import { Panel } from "@/components/ui";
import { opportunitySearchSchema } from "@/lib/search-params";
import { rootRoute } from "@/router";
import { createRoute } from "@tanstack/react-router";

export const opportunityRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/opportunity",
  validateSearch: (s) => opportunitySearchSchema.parse(s),
  component: OpportunityPage,
});

function OpportunityPage() {
  return (
    <div style={{ padding: 16 }}>
      <Panel title="OPPORTUNITY">
        <p style={{ color: "var(--color-text-muted)", fontSize: 12 }}>
          Opportunity analysis — coming in task 19.
        </p>
      </Panel>
    </div>
  );
}
