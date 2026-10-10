import { Panel } from "@/components/ui";
import { pricingSearchSchema } from "@/lib/search-params";
import { rootRoute } from "@/router";
import { createRoute } from "@tanstack/react-router";

export const pricingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/pricing",
  validateSearch: (s) => pricingSearchSchema.parse(s),
  component: PricingPage,
});

function PricingPage() {
  return (
    <div style={{ padding: 16 }}>
      <Panel title="PRICING">
        <p style={{ color: "var(--color-text-muted)", fontSize: 12 }}>
          Pricing models — coming in task 19.
        </p>
      </Panel>
    </div>
  );
}
