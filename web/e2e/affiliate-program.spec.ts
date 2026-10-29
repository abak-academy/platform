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
 *
 * The bulk CSV case needs the object storage the app uses for uploads (MinIO in
 * the local compose stack); it skips itself when presigning is not available.
 */

const AFFILIATOR_PASSWORD = "E2eAff#2026";

interface Ctx {
  adminToken: string;
  affiliatorId: string;
  affiliatorEmail: string;
  schoolA: { id: string; name: string; code: string };
  schoolB: { id: string; name: string; code: string };
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
      const code = `E2E${label}${suffix}`;
      const { res, body } = await api<{ id: string }>(request, "post", "/admin/schools", ctx.adminToken, { name, code });
      expect(res.status(), `create school ${label}: ${JSON.stringify(body)}`).toBe(201);
      return { id: body.id, name, code };
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

    await pickOption(page, "Produk", new RegExp(ctx.productName));
    await page.getByLabel("Cari nama siswa").fill(ctx.studentName);
    await page.getByLabel(ctx.studentName).check();
    await page.getByLabel("Kode promo").fill(ctx.promoCode);
    await page.getByRole("button", { name: "Pratinjau" }).click();

    const preview = page.getByTestId("aff-preview");
    await expect(preview).toContainText(ctx.schoolA.name);
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

  test("the super admin finds the affiliator in its own menu, reads the report and exports it", async ({ page, context, request }) => {
    await loginRealAdmin(context, request);
    await page.goto("/admin/affiliators/reports");

    // The affiliator menu is a group of its own, apart from System.
    await expect(page.getByRole("link", { name: "Laporan", exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "Affiliator", exact: true })).toBeVisible();

    await page.getByRole("searchbox", { name: "Cari nama atau email" }).fill(ctx.affiliatorEmail);
    const row = page.locator("tr", { hasText: ctx.affiliatorEmail });
    await expect(row).toBeVisible();
    await expect(row).toContainText("Aktif");
    // One free order paid today: 1 paid order, 1 unit, no money.
    const cells = (await row.locator("td").allTextContents()).map((c) => c.trim());
    expect(cells[3]).toBe("1");
    expect(cells[4]).toBe("1");
    expect(cells[5]).toBe("Rp0");

    await row.getByRole("button", { name: "Detail" }).click();
    await expect(page.getByRole("dialog")).toContainText(ctx.productName);
    await page.getByRole("dialog").getByRole("button", { name: "Tutup" }).click();

    await page.getByRole("tab", { name: "Per sekolah" }).click();
    await page.getByRole("searchbox", { name: "Cari sekolah" }).fill(ctx.schoolA.name);
    const schoolRow = page.locator("tr", { hasText: ctx.schoolA.name });
    await expect(schoolRow).toBeVisible();
    // 1 student, covered by the order, none left uncovered.
    const schoolCells = (await schoolRow.locator("td").allTextContents()).map((c) => c.trim());
    expect(schoolCells.slice(2, 5)).toEqual(["1", "1", "0"]);

    await page.getByRole("tab", { name: "Penggunaan promo" }).click();
    await page.getByRole("searchbox", { name: "Cari kode promo" }).fill(ctx.promoCode);
    await expect(page.locator("tr", { hasText: ctx.promoCode })).toContainText("Affiliator");

    await page.getByRole("tab", { name: "Per affiliator" }).click();
    await page.getByRole("searchbox", { name: "Cari nama atau email" }).fill(ctx.affiliatorEmail);
    const [download] = await Promise.all([page.waitForEvent("download"), page.getByRole("button", { name: "Ekspor CSV" }).click()]);
    expect(download.suggestedFilename()).toMatch(/^laporan-affiliator-affiliators-.*\.csv$/);
    const stream = await download.createReadStream();
    const chunks: Buffer[] = [];
    for await (const chunk of stream) chunks.push(chunk as Buffer);
    const csv = Buffer.concat(chunks).toString("utf8");
    expect(csv.startsWith("\uFEFFnama,email,status")).toBeTruthy();
    expect(csv).toContain(ctx.affiliatorEmail);
  });

  test("promoting a school admin warns that it cannot be undone and starts with its school", async ({ page, context, request }) => {
    const suffix = `${Date.now()}`;
    const email = `promote-${suffix}@example.test`;
    const created = await api<{ id: string }>(request, "post", "/admin/system/accounts", ctx.adminToken, {
      email, name: `E2E Promote ${suffix}`, role: "admin_school", password: AFFILIATOR_PASSWORD, school_id: ctx.schoolB.id,
    });
    expect(created.res.status(), JSON.stringify(created.body)).toBe(201);

    await loginRealAdmin(context, request);
    await page.goto("/admin/affiliators");
    await page.getByRole("tab", { name: "Promote" }).click();
    await expect(page.getByRole("note")).toContainText("tidak bisa diturunkan kembali menjadi school admin");

    await page.getByRole("searchbox", { name: "Cari nama, email, atau sekolah" }).fill(email);
    const row = page.locator("tr", { hasText: email });
    await expect(row).toContainText(ctx.schoolB.name);
    await row.getByRole("button", { name: "Jadikan affiliator" }).click();

    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("alert")).toContainText("Tidak bisa diturunkan kembali");
    await expect(dialog.getByRole("alert")).toContainText("tidak bisa turun lagi menjadi school admin");
    await expect(dialog.getByRole("list", { name: "Sekolah terpilih" })).toContainText(ctx.schoolB.name);

    const promoted = page.waitForResponse((r) => r.request().method() === "POST" && r.url().includes(`/affiliators/${created.body.id}/promote`));
    await dialog.getByRole("button", { name: "Jadikan affiliator" }).click();
    expect((await promoted).ok()).toBeTruthy();

    // Gone from the promotable list, and the account is now an affiliator of that school.
    await page.getByRole("searchbox", { name: "Cari nama, email, atau sekolah" }).fill(email);
    await expect(page.locator("tr", { hasText: email })).toHaveCount(0);
    const login = await api<{ access_token: string }>(request, "post", "/auth/login", null, { identifier: email, password: AFFILIATOR_PASSWORD });
    const me = await api<{ schools?: { id: string }[] }>(request, "get", "/affiliates/me", login.body.access_token);
    expect(me.res.status()).toBe(200);
    expect(me.body.schools?.map((x) => x.id)).toEqual([ctx.schoolB.id]);
  });

  test("creates many affiliators from a CSV with one school per row, generating blank passwords", async ({ page, context, request }) => {
    const suffix = `${Date.now()}`;
    const probe = await api(request, "post", "/admin/system/affiliators/bulk/presign?filename=probe.csv", ctx.adminToken);
    test.skip(!probe.res.ok(), "needs the object storage used for uploads (MinIO in the local stack)");

    const given = `bulk-given-${suffix}@example.test`;
    const generated = `bulk-gen-${suffix}@example.test`;
    const broken = `bulk-bad-${suffix}@example.test`;
    const csv = [
      "name,email,password,school_npsn,school_code",
      `E2E Bulk Given,${given},Given#2026pw,,${ctx.schoolA.code}`,
      // One school per row: the same email on two rows is one affiliator with two schools.
      `E2E Bulk Generated,${generated},,,${ctx.schoolA.code}`,
      `E2E Bulk Generated,${generated},,,${ctx.schoolB.code}`,
      `E2E Bulk Broken,${broken},,,NO_SUCH_SCHOOL`,
    ].join("\n");

    await loginRealAdmin(context, request);
    await page.goto("/admin/affiliators");
    await page.getByRole("button", { name: "Tambah affiliator" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("radio", { name: "Banyak akun (CSV)" }).click();
    await expect(dialog.getByLabel("Pilih berkas CSV")).toBeVisible();

    await dialog.getByLabel("Pilih berkas CSV").setInputFiles({ name: "affiliators.csv", mimeType: "text/csv", buffer: Buffer.from(csv) });
    await dialog.getByRole("button", { name: "Unggah" }).click();
    await expect(dialog.getByText("Impor selesai.")).toBeVisible({ timeout: 60_000 });

    const resultHref = await dialog.getByRole("link", { name: /Unduh hasil impor/ }).getAttribute("href");
    const report = await (await request.get(resultHref!)).text();
    const lines = report.trim().split("\n");
    expect(lines[0]).toBe("row,name,email,school,status,temp_password,error");
    const forEmail = (email: string) => lines.filter((l) => l.includes(email));
    expect(forEmail(given)).toHaveLength(1);
    expect(forEmail(given)[0]).toContain("success");
    expect(forEmail(broken)[0]).toContain("failed");
    expect(forEmail(broken)[0]).toContain("school not found by code");
    // Two lines for the generated account, both created; the password is on the first only.
    const genLines = forEmail(generated).map((l) => l.split(","));
    expect(genLines).toHaveLength(2);
    expect(genLines.map((l) => l[4])).toEqual(["success", "success"]);
    expect(genLines.map((l) => l[3])).toEqual([ctx.schoolA.name, ctx.schoolB.name]);
    const tempPassword = genLines[0][5];
    expect(tempPassword.length).toBeGreaterThanOrEqual(8);
    expect(genLines[1][5]).toBe("");

    for (const [email, password] of [[given, "Given#2026pw"], [generated, tempPassword]]) {
      const login = await api<{ access_token: string }>(request, "post", "/auth/login", null, { identifier: email, password });
      expect(login.res.ok(), `${email} can sign in`).toBeTruthy();
    }
    const me = await api<{ schools?: unknown[] }>(request, "get", "/affiliates/me", (await api<{ access_token: string }>(request, "post", "/auth/login", null, { identifier: generated, password: tempPassword })).body.access_token);
    expect(me.body.schools).toHaveLength(2);
    const none = await api<{ data: unknown[] }>(request, "get", `/admin/system/affiliators?q=${encodeURIComponent(broken)}`, ctx.adminToken);
    expect(none.body.data).toHaveLength(0);
  });

  test("the super admin sees which schools the affiliator has and exports the list", async ({ page, context, request }) => {
    await loginRealAdmin(context, request);
    await page.goto("/admin/affiliators");

    await page.getByRole("tab", { name: "Daftar sekolah" }).click();
    await page.getByRole("searchbox", { name: "Cari affiliator atau sekolah" }).fill(ctx.affiliatorEmail);
    const row = page.locator("tr", { hasText: ctx.schoolA.name });
    await expect(row).toBeVisible();
    await expect(row).toContainText(ctx.affiliatorEmail);

    const [download] = await Promise.all([page.waitForEvent("download"), page.getByRole("button", { name: "Ekspor CSV" }).click()]);
    expect(download.suggestedFilename()).toBe("sekolah-affiliator.csv");
    const chunks: Buffer[] = [];
    for await (const chunk of await download.createReadStream()) chunks.push(chunk as Buffer);
    const csv = Buffer.concat(chunks).toString("utf8");
    expect(csv.startsWith("\uFEFFaffiliator,email")).toBeTruthy();
    expect(csv).toContain(ctx.schoolA.name);
  });

  test("the super admin deactivates the affiliator from the list, and only that action is offered", async ({ page, context, request }) => {
    await loginRealAdmin(context, request);
    await page.goto("/admin/affiliators");

    await page.getByRole("searchbox", { name: "Cari nama atau email" }).fill(ctx.affiliatorEmail);
    const row = page.locator("tr", { hasText: ctx.affiliatorEmail });
    await expect(row).toBeVisible();
    // Exact: the row's own "Nonaktifkan" button contains the text "Nonaktif".
    await expect(row.getByText("Aktif", { exact: true })).toBeVisible();
    await expect(row.getByRole("button")).toHaveText(["Nonaktifkan"]);
    await expect(page.getByTestId("pager-range")).toContainText("dari");

    page.once("dialog", (d) => d.accept());
    const patched = page.waitForResponse((r) => r.request().method() === "PATCH" && r.url().includes(`/accounts/${ctx.affiliatorId}/status`));
    await row.getByRole("button", { name: "Nonaktifkan" }).click();
    expect((await patched).ok(), "the status change must reach the server").toBeTruthy();
    await expect(row.getByText("Nonaktif", { exact: true })).toBeVisible();
    await expect(row.getByRole("button")).toHaveText(["Aktifkan"]);
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
