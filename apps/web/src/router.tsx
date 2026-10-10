import { ContextBar } from "@/components/layout/ContextBar";
import { TopBar } from "@/components/layout/TopBar";
import {
  Outlet,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from "@tanstack/react-router";

// Root layout route
export const rootRoute = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  return (
    <>
      <TopBar />
      <ContextBar />
      {/* Page content offset by top bar (48px) + context bar (36px) */}
      <main
        style={{
          marginTop: 84,
          minHeight: "calc(100vh - 84px)",
        }}
      >
        <Outlet />
      </main>
    </>
  );
}

// Index redirect to /blocks
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  beforeLoad: () => {
    throw redirect({ to: "/blocks" });
  },
});

// Import page routes lazily to avoid circular imports
import { blocksRoute } from "@/routes/blocks";
import { healthRoute } from "@/routes/health";
import { opportunityRoute } from "@/routes/opportunity";
import { pricingRoute } from "@/routes/pricing";
import { simulatorRoute } from "@/routes/simulator";

const routeTree = rootRoute.addChildren([
  indexRoute,
  blocksRoute,
  healthRoute,
  pricingRoute,
  opportunityRoute,
  simulatorRoute,
]);

export const router = createRouter({
  routeTree,
  defaultPreload: "intent",
  scrollRestoration: true,
});

// Type augmentation for TanStack Router
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
