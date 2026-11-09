import { test, expect } from "@playwright/test";
import { loginRealAffiliator, loginRealStudent } from "./helpers/session";

/**
 * Mitra program (docs/specs/partner-program.md P18).
 *
 * Real-backend pattern — see helpers/session.ts. The system_config flag
 * `partner_enabled` must be "true" and E2E_AFFILIATOR_* must be an approved,
 * active Mitra account with at least one allowed school.
 *
 * Not covered here: paying a Mitra order. That path needs the payment gateway;
 * it is covered by the DB-backed service and worker tests instead.
 */

const partner = process.env.E2E_PARTNER === "true";

test.describe("mitra portal", () => {
  test.skip(!partner, "set E2E_PARTNER=true with E2E_AFFILIATOR_* pointing at an active Mitra account");

  test("the Mitra dashboard opens inside the affiliate shell", async ({ page, context, request }) => {
    await loginRealAffiliator(context, request);

    await page.goto("/affiliate/partner");

    await expect(page.getByRole("link", { name: /siswa/i }).first()).toBeVisible();
    await expect(page.getByRole("link", { name: /keranjang/i })).toHaveCount(0);
  });

  test("the student list never shows contact details", async ({ page, context, request }) => {
    await loginRealAffiliator(context, request);

    await page.goto("/affiliate/partner/students");

    await expect(page.locator("table")).not.toContainText("@");
  });

  test("a Mitra account cannot reach the student order endpoints", async ({ context, request }) => {
    const token = await loginRealAffiliator(context, request);
    const base = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";

    const res = await request.get(`${base}/orders`, { headers: { Authorization: `Bearer ${token}` } });

    expect(res.status()).toBe(403);
  });
});

test.describe("student isolation", () => {
  test("a student cannot open the Mitra portal", async ({ page, context, request }) => {
    await loginRealStudent(context, request);

    await page.goto("/affiliate/partner");

    await expect(page).not.toHaveURL(/\/affiliate\/partner$/);
  });
});
