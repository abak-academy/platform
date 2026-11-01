import { test, expect } from "@playwright/test";
import { loginRealAffiliator, loginRealStudent } from "./helpers/session";

/**
 * Affiliate program, referral path (docs/specs/affiliate-program.md A1, A11, A12).
 *
 * Real-backend pattern — see helpers/session.ts. Registration + OTP stays out
 * of e2e scope, so the affiliator is provisioned and approved by hand. The
 * system_config flag `affiliate_enabled` must be "true" and
 * E2E_AFFILIATE_CODE must be that affiliator's referral code.
 *
 * Not covered here: a paid order producing a commission. That path needs the
 * payment gateway; it is covered by the DB-backed service tests instead.
 */

const code = process.env.E2E_AFFILIATE_CODE;

test.describe("referral landing", () => {
  test.skip(!code, "set E2E_AFFILIATE_CODE to an approved affiliator's referral code");

  test("/r/{code} stores the cookie, lands on home, and prefills the register form", async ({ page, context }) => {
    await page.goto(`/r/${code}`);
    await page.waitForURL((url) => url.pathname === "/" || url.pathname === "/login");

    const cookies = await context.cookies();
    const ref = cookies.find((c) => c.name === "aff_ref");
    expect(ref?.value).toBe(code);

    await page.goto("/register");
    await expect(page.getByLabel(/kode referral/i)).toHaveValue(code!);
  });

  test("the public validate endpoint recognises the code without a session", async ({ request }) => {
    const base = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";
    const res = await request.get(`${base}/affiliates/public/validate?code=${encodeURIComponent(code!)}`);
    expect(res.ok()).toBeTruthy();
    expect((await res.json()).valid).toBe(true);
  });
});

test.describe("affiliator portal", () => {
  test("shows the code and link on the dashboard in the affiliate shell", async ({ page, context, request }) => {
    await loginRealAffiliator(context, request);

    await page.goto("/affiliate");

    await expect(page.getByText(/kode referral/i).first()).toBeVisible();
    await expect(page.getByRole("main").getByRole("link", { name: /siswa binaan/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /keranjang/i })).toHaveCount(0);
  });

  test("an affiliator who opens the student home is sent to the portal", async ({ page, context, request }) => {
    await loginRealAffiliator(context, request);

    await page.goto("/");

    await expect(page).toHaveURL(/\/affiliate$/);
  });

  test("the referred-students page lists names only, never contact details", async ({ page, context, request }) => {
    await loginRealAffiliator(context, request);

    await page.goto("/affiliate/students");

    await expect(page.getByRole("heading", { name: /siswa binaan/i })).toBeVisible();
    await expect(page.locator("table")).not.toContainText("@");
  });
});

test.describe("student isolation", () => {
  test("a student cannot open the affiliate portal", async ({ page, context, request }) => {
    await loginRealStudent(context, request);

    await page.goto("/affiliate");

    await expect(page).not.toHaveURL(/\/affiliate$/);
  });
});
