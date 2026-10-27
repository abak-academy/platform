import { test, expect, type APIRequestContext, type Page } from "@playwright/test";
import { API_BASE, loginRealAdmin } from "./helpers/session";

/**
 * Affiliate program, end to end on the real backend (docs/specs/affiliate-program-v2.md).
 *
 * Needs a super_admin in E2E_ADMIN_IDENTIFIER/PASSWORD (see helpers/session.ts)
 * and at least one published, orderable exam product (the affiliator reads the
 * catalog through its own route). The spec creates everything else — two
 * schools, an affiliator, an affiliator-only 100% promo — through the API, so
 * it can be re-run; each run uses its own suffix. The worker must be running:
 * the free (100% promo) order is fulfilled by the outbox.
 *
 * The student account is created through the affiliator's own registration, so
 * the credentials captured from the dialog also prove the student can sign in.
 */

const AFFILIATOR_PASSWORD = "E2eAff#2026";

interface Ctx {
  adminToken: string;
  affiliatorId: string;
  affiliatorEmail: string;
  schoolA: { id: string; name: string };
  schoolB: { id: string; name: string };
  promoCode: string;
  productName: string;
  studentName: string;
  studentUsername: string;
  studentPassword: string;
}

const ctx = {} as Ctx;

async function api<T>(request: APIRequestContext, method: "post" | "get" | "patch", path: string, token: string | null, data?: unknown) {
  const res = await request[method](`${API_BASE}${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    data,
  });
  return { res, body: (await res.json().catch(() => ({}))) as T };
}

async function loginAffiliatorUI(page: Page) {
  await page.goto("/admin/login");
  await page.locator("#identifier").fill(ctx.affiliatorEmail);
  await page.locator("#password").fill(AFFILIATOR_PASSWORD);
  await page.locator('button[type="submit"]').click();
  // Leaving this before the redirect lands makes the next goto race the sign-in.
  await page.waitForURL(/\/admin\/affiliate$/);
}

async function pickOption(page: Page, trigger: string, option: string | RegExp) {
  await page.getByRole("combobox", { name: trigger }).click();
  await page.getByRole("option", { name: option }).click();
}

test.describe.serial("affiliate program", () => {
  test.beforeAll(async ({ request }) => {
    // beforeAll cannot take the browser context, so this logs in over the API only.
    const adminLogin = await api<{ access_token: string }>(request, "post", "/auth/login", null, {
      identifier: process.env.E2E_ADMIN_IDENTIFIER,
      password: process.env.E2E_ADMIN_PASSWORD,
    });
    expect(adminLogin.res.ok(), "set E2E_ADMIN_IDENTIFIER/PASSWORD to a super_admin").toBeTruthy();
    ctx.adminToken = adminLogin.body.access_token;
    const suffix = `${Date.now()}`;

    const school = async (label: string) => {
      const name = `E2E Affiliate School ${label} ${suffix}`;
      const { res, body } = await api<{ id: string }>(request, "post", "/admin/schools", ctx.adminToken, { name, code: `E2E${label}${suffix}` });
      expect(res.status(), `create school ${label}: ${JSON.stringify(body)}`).toBe(201);
      return { id: body.id, name };
    };
    ctx.schoolA = await school("A");
    ctx.schoolB = await school("B");

    ctx.affiliatorEmail = `aff-${suffix}@example.test`;
    const created = await api<{ id: string }>(request, "post", "/admin/system/affiliators", ctx.adminToken, {
      email: ctx.affiliatorEmail,
      name: "E2E Affiliator",
      password: AFFILIATOR_PASSWORD,
      school_ids: [ctx.schoolA.id],
    });
    expect(created.res.status(), JSON.stringify(created.body)).toBe(201);
    ctx.affiliatorId = created.body.id;

    ctx.promoCode = `AFF100${suffix}`;
    const promo = await api(request, "post", "/admin/promo-codes", ctx.adminToken, {
      code: ctx.promoCode,
      discount_percent: 100,
      affiliator_only: true,
    });
    expect(promo.res.status(), JSON.stringify(promo.body)).toBe(201);

    const login = await api<{ access_token: string }>(request, "post", "/auth/login", null, {
      identifier: ctx.affiliatorEmail,
      password: AFFILIATOR_PASSWORD,
    });
    const products = await api<{ data: { name: string; type: string }[] }>(request, "get", "/affiliates/me/products", login.body.access_token);
    const exam = products.body.data?.find((p) => p.type === "exam");
    test.skip(!exam, "needs a published, orderable exam product in the database");
    ctx.productName = exam!.name;
    ctx.studentName = `E2E Siswa ${suffix}`;
  });

  test("an affiliator signs in at the admin login and lands on its own dashboard", async ({ page }) => {
    await loginAffiliatorUI(page);

    await expect(page).toHaveURL(/\/admin\/affiliate$/);
    await expect(page.getByRole("heading", { name: "Dashboard Affiliator" })).toBeVisible();
    for (const link of ["Siswa", "Pembelian", "Promo"]) {
      await expect(page.getByRole("link", { name: link, exact: true })).toBeVisible();
    }
    await expect(page.getByRole("link", { name: "Akun" })).toHaveCount(0);

    const schools = page.getByRole("region", { name: "Sekolah binaan" });
    await expect(schools.getByText(ctx.schoolA.name)).toBeVisible();
    await expect(schools.getByText(ctx.schoolB.name)).toHaveCount(0);
  });

  test("registers a student into the assigned school and shows the credentials once", async ({ page }) => {
    await loginAffiliatorUI(page);
    await page.goto("/admin/affiliate/students");

    await page.getByRole("button", { name: "Daftarkan siswa" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Nama").fill(ctx.studentName);
    await pickOption(page, "Jenjang", "SMA");
    await dialog.getByRole("button", { name: "Daftarkan siswa" }).click();

    const username = dialog.locator("dd").first();
    await expect(username).toBeVisible();
    ctx.studentUsername = (await username.textContent())!.trim();
    ctx.studentPassword = (await dialog.locator("dd").nth(1).textContent())!.trim();
    expect(ctx.studentUsername).not.toBe("");
    expect(ctx.studentPassword.length).toBeGreaterThanOrEqual(8);
    await dialog.getByRole("button", { name: "Tutup" }).click();

    const row = page.getByRole("row", { name: new RegExp(ctx.studentName) });
    await expect(row).toBeVisible();
    await expect(row).toContainText(ctx.schoolA.name);
    await expect(row).toContainText("Saya");
  });

  test("keeps the affiliator inside its schools and its promo away from everyone else", async ({ request }) => {
    const login = await api<{ access_token: string }>(request, "post", "/auth/login", null, { identifier: ctx.affiliatorEmail, password: AFFILIATOR_PASSWORD });
    const token = login.body.access_token;

    const foreign = await api<{ code: string }>(request, "post", `/admin/students?school_id=${ctx.schoolB.id}`, token, { name: "Intruder", jenjang: "SMA" });
    expect(foreign.res.status()).toBe(403);
    expect(foreign.body.code).toBe("school_not_assigned");

    const missing = await api(request, "post", "/admin/students", token, { name: "No School", jenjang: "SMA" });
    expect(missing.res.status()).toBe(400);

    const anonymous = await api<{ code: string }>(request, "post", "/promo-codes/validate", null, { code: ctx.promoCode, subtotal: 50000 });
    expect(anonymous.res.status()).toBe(422);
    expect(anonymous.body.code).toBe("invalid_promo");

    const studentLogin = await api<{ access_token: string }>(request, "post", "/auth/login", null, { identifier: ctx.studentUsername, password: ctx.studentPassword });
    expect(studentLogin.res.ok(), "the registered student can sign in").toBeTruthy();
    const asStudent = await api(request, "get", "/affiliates/me/products", studentLogin.body.access_token);
    expect(asStudent.res.status()).toBe(403);
  });

  test("buys an exam for the student with the affiliator promo, and the student gets access", async ({ page, request }) => {
    await loginAffiliatorUI(page);
    await page.goto("/admin/affiliate/orders/new");

    await pickOption(page, "Sekolah", ctx.schoolA.name);
    await pickOption(page, "Produk", new RegExp(ctx.productName));
    await page.getByLabel(ctx.studentName).check();
    await page.getByLabel("Kode promo").fill(ctx.promoCode);
    await page.getByRole("button", { name: "Pratinjau" }).click();

    const preview = page.getByTestId("aff-preview");
    await expect(preview).toContainText("Rp50.000");
    await expect(preview).toContainText("-Rp50.000");
    await expect(preview).toContainText("Rp0");

    await preview.getByRole("button", { name: "Buat pesanan" }).click();
    await expect(page.getByRole("heading", { name: "Pesanan dibuat" })).toBeVisible();
    await expect(page.getByTestId("aff-order-status")).toContainText("Belum dibayar");
    await page.getByRole("button", { name: "Bayar" }).click();
    // The order is free, so there is no payment popup: the status line is the proof.
    await expect(page.getByTestId("aff-order-status")).toContainText("Selesai", { timeout: 30_000 });

    await page.goto("/admin/affiliate/orders");
    const orderRow = page.getByRole("row", { name: new RegExp(ctx.productName) });
    await expect(orderRow).toContainText(ctx.promoCode);
    await expect(orderRow).toContainText(ctx.schoolA.name);

    const studentLogin = await api<{ access_token: string }>(request, "post", "/auth/login", null, { identifier: ctx.studentUsername, password: ctx.studentPassword });
    await expect
      .poll(async () => {
        const regs = await api<{ data?: unknown[] } | unknown[]>(request, "get", "/exam/registrations", studentLogin.body.access_token);
        const list = Array.isArray(regs.body) ? regs.body : (regs.body.data ?? []);
        return list.length;
      }, { message: "the worker should register the student for the exam", timeout: 30_000 })
      .toBeGreaterThan(0);
  });

  test("the dashboard and the order list reflect the purchase", async ({ page }) => {
    await loginAffiliatorUI(page);

    await expect(page.getByText("Order dibayar")).toBeVisible();
    await expect(page.getByTestId("no-access")).toHaveCount(0);
    const paidCard = page.locator(".md-card-elevated", { hasText: "Order dibayar" });
    await expect(paidCard).toContainText("1");
    const top = page.getByRole("region", { name: "Produk terlaris per sekolah" });
    await expect(top).toContainText(ctx.productName);
    await expect(top).toContainText(ctx.schoolA.name);
    await expect(page.getByText("siswa aktif belum dibelikan produk")).toBeVisible();
    await expect(page.locator("strong", { hasText: /^0$/ })).toBeVisible();
  });

  test("the super admin sees the affiliator's results and can deactivate it", async ({ page, context, request }) => {
    await loginRealAdmin(context, request);
    await page.goto("/admin/system/affiliators");

    const row = page.getByRole("row", { name: /E2E Affiliator/ }).filter({ hasText: ctx.affiliatorEmail });
    await expect(row).toBeVisible();
    // Exact: the row's own "Nonaktifkan" button contains the text "Nonaktif".
    await expect(row.getByText("Aktif", { exact: true })).toBeVisible();
    await expect(row).toContainText("Rp0");

    page.once("dialog", (d) => d.accept());
    const patched = page.waitForResponse((r) => r.request().method() === "PATCH" && r.url().includes(`/accounts/${ctx.affiliatorId}/status`));
    await row.getByRole("button", { name: "Nonaktifkan" }).click();
    expect((await patched).ok(), "the status change must reach the server").toBeTruthy();
    await expect(row.getByText("Nonaktif", { exact: true })).toBeVisible();
  });

  test("a deactivated affiliator can look but not act", async ({ page, request }) => {
    const login = await api<{ access_token: string }>(request, "post", "/auth/login", null, { identifier: ctx.affiliatorEmail, password: AFFILIATOR_PASSWORD });
    expect(login.res.ok(), "a deactivated affiliator can still sign in").toBeTruthy();
    const token = login.body.access_token;

    const read = await api(request, "get", "/affiliates/me/dashboard", token);
    expect(read.res.status()).toBe(200);
    const write = await api<{ code: string }>(request, "post", `/admin/students?school_id=${ctx.schoolA.id}`, token, { name: "Late", jenjang: "SMA" });
    expect(write.res.status()).toBe(403);
    expect(write.body.code).toBe("account_deactivated");

    await loginAffiliatorUI(page);
    await expect(page).toHaveURL(/\/admin\/affiliate$/);
    await expect(page.getByRole("status")).toContainText("Akun Anda dinonaktifkan");
    await page.goto("/admin/affiliate/students");
    await expect(page.getByRole("button", { name: "Daftarkan siswa" })).toBeDisabled();
    await expect(page.getByRole("button", { name: "Impor massal" })).toBeDisabled();
  });
});
