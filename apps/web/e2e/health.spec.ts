import { expect, test } from "@playwright/test";

const HEALTH_DAILY = {
  data: [
    {
      date: "2026-09-24",
      tip_revenue_eth: "0.5",
      tip_revenue_usd: 1250,
      block_count: 100,
      contested_pct: 0.4,
      hhi: 3200,
      revert_rate: 0.018,
      top_payer: "0xabc",
      top_payer_share: 0.35,
    },
  ],
};

const TREASURY_DAILY = { data: [] };
const ERA_COMP = { data: [] };
const BASE_FEE = { data: [] };
const SOURCE_STATUS = { data: [] };
const DAG_RUNS = { data: [] };

test.beforeEach(async ({ page }) => {
  await page.route("**/api/v1/blocks/health*", (route) => route.fulfill({ json: HEALTH_DAILY }));
  await page.route("**/api/v1/treasury*", (route) => route.fulfill({ json: TREASURY_DAILY }));
  await page.route("**/api/v1/compare*", (route) => route.fulfill({ json: ERA_COMP }));
  await page.route("**/api/v1/blocks/base-fee*", (route) => route.fulfill({ json: BASE_FEE }));
  await page.route("**/api/v1/health*", (route) => route.fulfill({ json: SOURCE_STATUS }));
  await page.route("**/api/v1/datasets*", (route) => route.fulfill({ json: DAG_RUNS }));
});

test("overview tab renders by default", async ({ page }) => {
  await page.goto("/health");
  await expect(page.getByRole("tab", { name: /overview/i })).toBeVisible();
});

test("tab deep link works — treasury", async ({ page }) => {
  await page.goto("/health?tab=treasury");
  await expect(page.getByRole("tab", { name: /treasury/i })).toHaveAttribute(
    "aria-selected",
    "true",
  );
});

test("tab deep link works — compare", async ({ page }) => {
  await page.goto("/health?tab=compare");
  await expect(page.getByRole("tab", { name: /compare/i })).toHaveAttribute(
    "aria-selected",
    "true",
  );
});

test("window selector is present", async ({ page }) => {
  await page.goto("/health");
  await expect(page.getByRole("button", { name: /7d/i }).first()).toBeVisible();
});

test("switching window updates URL", async ({ page }) => {
  await page.goto("/health");
  await page.getByRole("button", { name: /30d/i }).first().click();
  await expect(page).toHaveURL(/window=30d/);
});

test("payers tab shows leaderboard heading", async ({ page }) => {
  await page.goto("/health?tab=payers");
  await expect(page.getByText(/payer/i).first()).toBeVisible();
});

test("sources tab shows ingester heading", async ({ page }) => {
  await page.goto("/health?tab=sources");
  await expect(page.getByText(/source|ingester/i).first()).toBeVisible();
});
