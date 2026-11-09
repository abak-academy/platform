import { test, expect } from "@playwright/test";
import { loginRealAdmin, loginRealAffiliator, loginRealStudent } from "./helpers/session";

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

test.describe("mitra portal pages", () => {
  test.skip(!partner, "set E2E_PARTNER=true with E2E_AFFILIATOR_* pointing at an active Mitra account");
  const base = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";

  test("the schools page lists the schools the admin allowed", async ({ page, context, request }) => {
    const token = await loginRealAffiliator(context, request);
    const api = await request.get(`${base}/partners/me/schools`, { headers: { Authorization: `Bearer ${token}` } });
    expect(api.ok()).toBeTruthy();
    const schools = (await api.json()).data as { name: string }[];

    await page.goto("/affiliate/partner/schools");

    await expect(page.getByRole("heading", { name: /sekolah saya/i })).toBeVisible();
    if (schools.length > 0) await expect(page.getByText(schools[0].name)).toBeVisible();
  });

  test("an order shows its products, students and voucher on the detail page", async ({ page, context, request }) => {
    const token = await loginRealAffiliator(context, request);
    const headers = { Authorization: `Bearer ${token}` };
    const products = (await (await request.get(`${base}/partners/me/products`, { headers })).json()).data as { id: string; name: string }[];
    const students = (await (await request.get(`${base}/partners/me/students?limit=5`, { headers })).json()).data as { id: string; name: string }[];
    test.skip(products.length === 0 || students.length === 0, "needs an open product and a student for this Mitra");

    const created = await request.post(`${base}/partners/me/orders`, {
      headers,
      data: { product_id: products[0].id, student_ids: [students[0].id] },
    });
    test.skip(!created.ok(), "the first student may already own the first product");
    const order = (await created.json()) as { id: string };

    await page.goto(`/affiliate/partner/orders/${order.id}`);

    await expect(page.getByRole("heading", { name: /rincian pembelian/i })).toBeVisible();
    await expect(page.getByText(products[0].name).first()).toBeVisible();
    await expect(page.getByText(students[0].name).first()).toBeVisible();
    await expect(page.getByText(/tanpa voucher/i)).toBeVisible();
  });
});

test.describe("admin Mitra sub menu", () => {
  test.skip(!process.env.E2E_ADMIN_IDENTIFIER, "set E2E_ADMIN_IDENTIFIER / E2E_ADMIN_PASSWORD");

  test("lists only Mitra accounts", async ({ page, context, request }) => {
    await loginRealAdmin(context, request);

    await page.goto("/admin/affiliates/partners");

    await expect(page.getByRole("heading", { name: "Mitra", exact: true })).toBeVisible();
    await expect(page.getByRole("tab", { name: "Affiliator" })).toHaveCount(0);
  });
});

test.describe("student isolation", () => {
  test("a student cannot open the Mitra portal", async ({ page, context, request }) => {
    await loginRealStudent(context, request);

    await page.goto("/affiliate/partner");

    await expect(page).not.toHaveURL(/\/affiliate\/partner$/);
  });
});
