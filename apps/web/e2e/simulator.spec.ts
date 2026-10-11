import { expect, test } from "@playwright/test";

const BACKTEST_CREATE_RESP = { run_id: "test-run-1" };
const BACKTEST_STATUS_DONE = {
  run_id: "test-run-1",
  status: "done",
  error_msg: null,
};
const BACKTEST_RESULTS_RESP = {
  run_id: "test-run-1",
  strategy: "percentile-of-recent",
  from_block: 0,
  to_block: 100,
  wins: 80,
  total: 100,
  win_rate: 0.8,
  avg_cost_gwei: 1.2,
  total_pnl_gwei: 15.5,
  pnl_curve: [
    { block: 1, cumulative_pnl_gwei: 0.1 },
    { block: 2, cumulative_pnl_gwei: 0.3 },
  ],
  disclaimer: "Backtest results are simulated.",
};

test.beforeEach(async ({ page }) => {
  await page.route("**/api/v1/backtest", (route) => {
    if (route.request().method() === "POST") {
      route.fulfill({ json: BACKTEST_CREATE_RESP });
    } else {
      route.continue();
    }
  });
  await page.route("**/api/v1/backtest/test-run-1", (route) =>
    route.fulfill({ json: BACKTEST_STATUS_DONE }),
  );
  await page.route("**/api/v1/backtest/test-run-1/results", (route) =>
    route.fulfill({ json: BACKTEST_RESULTS_RESP }),
  );
});

test("renders strategy selector", async ({ page }) => {
  await page.goto("/simulator");
  await expect(page.getByLabel(/strategy/i)).toBeVisible();
});

test("run backtest button present and enabled", async ({ page }) => {
  await page.goto("/simulator");
  await expect(page.getByRole("button", { name: /run backtest/i })).toBeEnabled();
});

test("run creates a summary card", async ({ page }) => {
  await page.goto("/simulator");
  await page.getByRole("button", { name: /run backtest/i }).click();
  await expect(page.getByText(/win rate/i)).toBeVisible({ timeout: 5000 });
});

test("disclaimer appears after run", async ({ page }) => {
  await page.goto("/simulator");
  await page.getByRole("button", { name: /run backtest/i }).click();
  await expect(page.getByText(/backtest results are simulated/i)).toBeVisible({ timeout: 5000 });
});

test("max 4 runs disables button", async ({ page }) => {
  await page.goto("/simulator");
  for (let i = 0; i < 4; i++) {
    await page.getByRole("button", { name: /run backtest/i }).click();
    await page.waitForTimeout(100);
  }
  await expect(page.getByRole("button", { name: /max 4 runs/i })).toBeDisabled();
});
