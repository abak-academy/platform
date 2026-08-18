import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

/**
 * Executable proof for FR-24 (spec.md Part D): a DialogContent must never
 * outgrow the viewport. jsdom has no layout (N4), so ProductModal.test.tsx
 * can only assert the `.overflow-y-auto` body scroller exists — the actual
 * "does the dialog's bottom edge stay on screen" claim needs a real render.
 *
 * Follows e2e/question-editor.spec.ts's fake-session pattern: no backend,
 * no E2E_* credentials — seedSession + a page.route("**\/api/v1/**") catch-all.
 */

const FAKE_ADMIN_USER = {
  id: "e2e-fake-admin-id",
  email: "e2e-fake-admin@example.test",
  name: "E2E Fake Admin",
  role: "super_admin",
};

async function mockBackendForFakeSession(page: Page) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" })
  );
}

// Selecting "book" seeds ProductSpecsEditor with its eight canonical rows
// (lib/product-specs.ts SPEC_FIELDS.book) — the long form that originally
// pushed the dialog past the viewport (see ProductModal.tsx's own comment).
async function openProductCreateModal(page: Page) {
  await page.goto("/admin/products");
  await page.getByRole("button", { name: "Buat produk", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.locator("#product-type").selectOption("book");
  await page.locator("#product-name").fill("E2E Test Book");
  await page.locator("#product-price").fill("50000");
}

for (const viewport of [
  { width: 900, height: 600 },
  { width: 390, height: 844 },
]) {
  test(`product modal stays within the viewport at ${viewport.width}x${viewport.height}`, async ({ page, context }) => {
    await page.setViewportSize(viewport);
    await mockBackendForFakeSession(page);
    await seedSession(context, { token: "e2e-fake-token", refreshToken: "e2e-fake-refresh", user: FAKE_ADMIN_USER });

    await openProductCreateModal(page);

    const dialog = page.getByRole("dialog");
    const rect = await dialog.evaluate((el) => {
      const r = el.getBoundingClientRect();
      return { top: r.top, bottom: r.bottom };
    });
    expect(rect.top, "dialog top must not be pushed above the viewport").toBeGreaterThanOrEqual(0);
    expect(rect.bottom, "dialog bottom must stay at or above the viewport bottom").toBeLessThanOrEqual(
      viewport.height
    );

    const saveButton = page.getByRole("button", { name: "Simpan", exact: true });
    await expect(saveButton).toBeVisible();
    const buttonBox = await saveButton.boundingBox();
    expect(buttonBox, "save button must have a real bounding box").not.toBeNull();
    expect(buttonBox!.y).toBeGreaterThanOrEqual(0);
    expect(buttonBox!.y + buttonBox!.height, "save button must be reachable, not clipped below the fold").toBeLessThanOrEqual(
      viewport.height
    );
  });
}
