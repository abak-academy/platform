import { test, expect, type Page } from "@playwright/test";
import { appendFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const evidence = process.env.VERIFY_EVIDENCE_DIR!;
const api = process.env.VERIFY_API_URL!;

test.beforeEach(async ({ page }, info) => {
  const log = (line: string) => appendFileSync(join(evidence, `${info.title.split(":")[0]}.browser.log`), line + "\n");
  page.on("console", (m) => log(`console.${m.type()} ${m.text()}`));
  page.on("pageerror", (e) => log(`pageerror ${e.message}`));
  page.on("requestfailed", (r) => log(`requestfailed ${r.method()} ${r.url()} ${r.failure()?.errorText}`));
  page.on("response", (r) => r.url().startsWith(api) && log(`response ${r.status()} ${r.request().method()} ${r.url()}`));
});

async function login(page: Page, path: string, email: string, password: string) {
  await page.goto(path);
  await page.getByLabel(/^(Email atau Username|Email or Username)$/).fill(email);
  await page.getByLabel(/^(Kata Sandi|Password)$/).fill(password);
  await page.getByRole("button", { name: /^(Masuk|Sign in)$/ }).click();
  await page.waitForURL((u) => !u.pathname.endsWith("/login"), { timeout: 30_000 });
}

const admin = (page: Page) => login(page, "/admin/login", process.env.VERIFY_ADMIN_EMAIL!, process.env.VERIFY_ADMIN_PASSWORD!);

// Screenshot setup only, not a proof step: the theme lives in the persisted UI store.
async function setTheme(page: Page, theme: "light" | "dark") {
  await page.evaluate((t) => {
    localStorage.setItem("abak-ui", JSON.stringify({ state: { theme: t, lang: "id" }, version: 0 }));
  }, theme);
}

async function shoot(page: Page, name: string) {
  await page.waitForLoadState("networkidle");
  await page.screenshot({ path: join(evidence, `${name}.png`), fullPage: true });
}

// A fixture dashboard so the donut, feed and lists render with data; the stack's DB is empty.
const FIXTURE = {
  period: { from: "2026-10-27", to: "2026-11-25", bucket: "day" },
  kpi: {
    revenue: { value: 12500000, prev: 10000000 },
    revenue_digital: { value: 8000000, prev: 5000000 },
    revenue_physical: { value: 4500000, prev: 5000000 },
    order_count: { value: 84, prev: 90 },
    new_students: { value: 31, prev: 25 },
    schools: { value: 12 },
    students_total: { value: 640 },
  },
  series: Array.from({ length: 30 }, (_, i) => ({
    date: `2026-${i < 5 ? "10" : "11"}-${String(i < 5 ? 27 + i : i - 4).padStart(2, "0")}`,
    revenue: 300000 + ((i * 97) % 11) * 40000,
    revenue_digital: 200000 + ((i * 53) % 7) * 30000,
    revenue_physical: 100000 + ((i * 31) % 5) * 20000,
    order_count: 2 + (i % 4),
    new_students: i % 3,
    exam_students: 10 + (i % 6),
    buying_students: 4 + (i % 3),
  })),
  attention: { needs_confirm: 3, ready_to_ship: 5, shipment_failed: 1, active_sessions: 2, essays_to_grade: 4, essays_exam_id: "00000000-0000-0000-0000-000000000001" },
  top_products: [
    { product_id: "p1", name: "Paket Olimpiade Matematika Nasional", product_type: "exam", is_physical: false, product_revenue: 4200000, qty_sold: 42 },
    { product_id: "p2", name: "Buku Latihan Soal OSN SD", product_type: "book", is_physical: true, product_revenue: 2100000, qty_sold: 21 },
    { product_id: "p3", name: "Medali Juara", product_type: "merchandise", is_physical: true, product_revenue: 900000, qty_sold: 9 },
  ],
  upcoming_exams: [
    { id: "e1", title: "Olimpiade Sains Nasional — Babak 1", scheduled_at: "2026-12-02T08:00:00+07:00", registrant_count: 128 },
  ],
};

for (const width of [1366, 390]) {
  for (const theme of ["light", "dark"] as const) {
    test(`admin-pages-${width}-${theme}: admin screens render in v2`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await admin(page);
      await setTheme(page, theme);
      for (const [route, name] of [
        ["/admin", "admin"],
        ["/admin/orders", "orders"],
        ["/admin/exam/tests", "exam-tests"],
        ["/admin/exam/packages", "exam-packages"],
      ] as const) {
        await page.goto(route);
        await shoot(page, `${name}-${width}-${theme}`);
      }
      await page.route(`${api}/admin/dashboard*`, (r) => r.fulfill({ json: FIXTURE }));
      await page.goto("/admin");
      await expect(page.getByRole("img", { name: /Digital 64%/ })).toBeVisible({ timeout: 20_000 });
      await shoot(page, `admin-fixture-${width}-${theme}`);
    });
  }
}

test("student-pages: student shell at 390 and 1366, light and dark", async ({ page }) => {
  await login(page, "/login", process.env.VERIFY_STUDENT_EMAIL!, process.env.VERIFY_STUDENT_PASSWORD!);
  for (const width of [1366, 390]) {
    for (const theme of ["light", "dark"] as const) {
      await page.setViewportSize({ width, height: 900 });
      await setTheme(page, theme);
      await page.goto("/");
      await expect(page.getByRole("button", { name: /Bahasa|Language/ })).toHaveCount(0);
      await shoot(page, `student-home-${width}-${theme}`);
    }
  }
});

test("sidebar-accordion: Akademik opens on /admin/exam/tests, one group open at a time", async ({ page }) => {
  await page.setViewportSize({ width: 1366, height: 900 });
  await admin(page);
  await page.goto("/admin/exam/tests");
  const academic = page.getByRole("button", { name: /^Akademik/ });
  const store = page.getByRole("button", { name: /^Toko/ });
  await expect(academic).toHaveAttribute("aria-expanded", "true");
  await expect(store).toHaveAttribute("aria-expanded", "false");
  await store.click();
  await expect(store).toHaveAttribute("aria-expanded", "true");
  await expect(academic).toHaveAttribute("aria-expanded", "false");
  await page.screenshot({ path: join(evidence, "sidebar-toko-open.png") });
  await store.click();
  await expect(store).toHaveAttribute("aria-expanded", "false");
  writeFileSync(join(evidence, "sidebar.aria.txt"), await page.getByRole("navigation").first().ariaSnapshot());
});

test("content-cap: page content is capped at 1480px and centred at 1920px", async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 900 });
  await admin(page);
  await page.goto("/admin/orders");
  await page.waitForLoadState("networkidle");
  const box = await page.locator("main > div").first().boundingBox();
  writeFileSync(join(evidence, "content-cap.json"), JSON.stringify(box));
  expect(box!.width).toBe(1480);
  expect(Math.round(box!.x - 264)).toBe(Math.round(1920 - box!.x - box!.width));
  await page.screenshot({ path: join(evidence, "content-cap-1920.png") });
});

for (const width of [1366, 390]) {
  test(`user-menu-${width}: menu opens upward, switches language, Escape returns focus, Keluar signs out`, async ({ page }) => {
    await page.setViewportSize({ width, height: 844 });
    await admin(page);
    await page.goto("/admin");
    if (width < 1024) {
      await page.getByRole("button", { name: /^(Toggle sidebar|Menu)$/ }).click();
      await expect(page.getByRole("dialog")).toBeVisible();
      await page.screenshot({ path: join(evidence, `drawer-${width}.png`) });
    }
    const scope = width < 1024 ? page.getByRole("dialog") : page;
    const card = scope.getByRole("button", { name: /^(Menu akun|Account menu)$/ }).last();
    const cardBox = await card.boundingBox();
    await card.click();
    const menu = page.getByRole("menu");
    await expect(menu).toBeVisible();
    const items = await menu.getByRole("menuitem").allInnerTexts();
    writeFileSync(join(evidence, `user-menu-${width}.items.json`), JSON.stringify(items, null, 2));
    const menuBox = await menu.boundingBox();
    expect(menuBox!.y + menuBox!.height).toBeLessThanOrEqual(cardBox!.y + 1);
    expect(menuBox!.y).toBeGreaterThanOrEqual(0);
    await page.screenshot({ path: join(evidence, `user-menu-${width}.png`) });
    writeFileSync(join(evidence, `user-menu-${width}.aria.txt`), await menu.ariaSnapshot());

    await menu.getByRole("button", { name: /^EN$/ }).click();
    await expect(menu).toBeVisible();
    await expect(menu.getByText(/Account & role/)).toBeVisible();
    await page.screenshot({ path: join(evidence, `user-menu-${width}-en.png`) });
    await menu.getByRole("button", { name: /^ID$/ }).click();
    await expect(menu.getByText(/Akun & peran/)).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(menu).toBeHidden();
    await expect(card).toBeFocused();

    await card.click();
    await page.getByRole("menuitem", { name: /^(Keluar|Log out|Logout)$/ }).click();
    await page.waitForURL(/\/admin\/login/, { timeout: 15_000 });
    writeFileSync(join(evidence, `user-menu-${width}.logout-url.txt`), page.url());
  });
}
