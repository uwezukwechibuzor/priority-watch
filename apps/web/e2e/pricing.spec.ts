import { expect, test } from "@playwright/test";

const RECOMMEND_RESP = [
  {
    model: "floor-estimate",
    recommended_tip_gwei: 1.5,
    interval_lo_gwei: 1.2,
    interval_hi_gwei: 1.8,
    hit_rate: 0.82,
    cost_est_wei: "500000000000000",
    reasoning: ["Tip set to 75th percentile of recent blocks."],
  },
];

const EVAL_RESP = {
  data: [
    {
      model: "floor-estimate",
      n: 1,
      confidence: 0.8,
      window: 200,
      hit_rate: 0.82,
      avg_cost_gwei: 1.5,
      eval_block_count: 1000,
      evaluated_at: "2026-09-24T00:00:00Z",
    },
  ],
};

test.beforeEach(async ({ page }) => {
  await page.route("**/api/v1/pricing/recommend*", (route) =>
    route.fulfill({ json: RECOMMEND_RESP }),
  );
  await page.route("**/api/v1/pricing/models/eval*", (route) => route.fulfill({ json: EVAL_RESP }));
});

test("renders sidebar controls", async ({ page }) => {
  await page.goto("/pricing");
  await expect(page.getByRole("button", { name: /get recommendation/i })).toBeVisible();
});

test("shows recommendation panel", async ({ page }) => {
  await page.goto("/pricing");
  await expect(page.getByText(/1\.5/)).toBeVisible();
});

test("model eval table renders", async ({ page }) => {
  await page.goto("/pricing");
  await expect(page.getByText(/floor-estimate/i)).toBeVisible();
});

test("block param reflects in URL", async ({ page }) => {
  await page.goto("/pricing?block=12345");
  const input = page.getByLabel(/block/i);
  await expect(input).toHaveValue("12345");
});

test("model selector updates URL", async ({ page }) => {
  await page.goto("/pricing");
  const select = page.getByLabel(/model/i);
  await select.selectOption("floor-estimate");
  await expect(page).toHaveURL(/model=floor-estimate/);
});
