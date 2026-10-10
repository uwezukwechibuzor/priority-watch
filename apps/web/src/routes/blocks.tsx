import { Panel } from "@/components/ui";
import { blocksSearchSchema } from "@/lib/search-params";
import { rootRoute } from "@/router";
import { createRoute } from "@tanstack/react-router";

export const blocksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/blocks",
  validateSearch: (s) => blocksSearchSchema.parse(s),
  component: BlocksPage,
});

function BlocksPage() {
  return (
    <div style={{ padding: 16 }}>
      <Panel title="BLOCKS">
        <p style={{ color: "var(--color-text-muted)", fontSize: 12 }}>
          Block table — coming in task 17.
        </p>
      </Panel>
    </div>
  );
}
