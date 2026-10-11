import { expect, test } from "@playwright/test";

const EMPTY = { data: [] };

test.beforeEach(async ({ page }) => {
  await page.route("**/api/v1/opportunity/spread/heatmap*", (route) => route.fulfill({ json: [] }));
  await page.route("**/api/v1/opportunity/spread/histogram*", (route) =>
    route.fulfill({ json: [] }),
  );
  await page.route("**/api/v1/opportunity/markouts*", (route) => route.fulfill({ json: [] }));
  await page.route("**/api/v1/opportunity/position*", (route) => route.fulfill({ json: [] }));
  await page.route("**/api/v1/execution/quality*", (route) => route.fulfill({ json: EMPTY }));
});

test("renders page with range buttons", async ({ page }) => {
  await page.goto("/opportunity");
  await expect(page.getByRole("button", { name: /24h/i })).toBeVisible();
});

test("entropy benchmark note visible", async ({ page }) => {
  await page.goto("/opportunity");
  await expect(page.getByText(/entropy research/i)).toBeVisible();
});

test("range param persists in URL", async ({ page }) => {
  await page.goto("/opportunity");
  await page.getByRole("button", { name: /7d/i }).click();
  await expect(page).toHaveURL(/range=7d/);
});

test("deep link range=30d pre-selects button", async ({ page }) => {
  await page.goto("/opportunity?range=30d");
  await expect(page.getByRole("button", { name: /30d/i })).toHaveAttribute("aria-pressed", "true");
});

test("spread heatmap panel heading visible", async ({ page }) => {
  await page.goto("/opportunity");
  await expect(page.getByText(/spread heatmap/i)).toBeVisible();
});
