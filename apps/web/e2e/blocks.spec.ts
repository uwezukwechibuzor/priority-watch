import { expect, test } from "@playwright/test";

const BLOCKS_RESP = {
  data: [
    {
      block_number: 100,
      block_time: "2026-09-24T00:00:00Z",
      tip_eth: "0.001",
      tip_usd: 2.5,
      base_fee_gwei: 0.1,
      tx_count: 42,
      contested: true,
      second_to_first_ratio: 0.85,
      top_tipper: "0xabc",
    },
  ],
  pagination: { next_cursor: null },
};

const KPI_RESP = {
  tip_revenue_eth: "1.5",
  tip_revenue_usd: 3750,
  block_count: 1000,
  contested_pct: 0.42,
  avg_base_fee_gwei: 0.08,
  revert_rate: 0.02,
};

test.beforeEach(async ({ page }) => {
  await page.route("**/api/v1/blocks*", (route) => route.fulfill({ json: BLOCKS_RESP }));
  await page.route("**/api/v1/blocks/kpi*", (route) => route.fulfill({ json: KPI_RESP }));
});

test("renders KPI strip", async ({ page }) => {
  await page.goto("/blocks");
  await expect(page.getByText(/tip revenue/i).first()).toBeVisible();
});

test("block table shows rows", async ({ page }) => {
  await page.goto("/blocks");
  await expect(page.getByText("100")).toBeVisible();
});

test("range=1h deep link sets live mode", async ({ page }) => {
  await page.goto("/blocks?range=1h");
  await expect(page.getByRole("button", { name: /1h/i })).toHaveAttribute("aria-pressed", "true");
});

test("range param persists in URL", async ({ page }) => {
  await page.goto("/blocks");
  await page.getByRole("button", { name: /7d/i }).click();
  await expect(page).toHaveURL(/range=7d/);
});
