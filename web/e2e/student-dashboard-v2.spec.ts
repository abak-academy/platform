import { test, expect, type BrowserContext, type Page } from "@playwright/test";
import { appendFileSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { seedSession } from "./helpers/session";

const OUT = process.env.E2E_EVIDENCE_DIR ?? "test-results/student-dashboard-v2";
mkdirSync(OUT, { recursive: true });
const NOTES = `${OUT}/notes.log`;
const note = (s: string) => appendFileSync(NOTES, s + "\n");

const HOUR = 3600_000;
const DAY = 24 * HOUR;
const iso = (offsetMs: number) => new Date(Date.now() + offsetMs).toISOString();

const PENDING_ID = "ord-pending-aaaa0001";
const PROC_ID = "ord-processing-bbbb0002";
const PAID_ID = "ord-paid-cccc0003";
const RESI = "JNE0012345678";

const physicalItem = (orderId: string) => ({
  id: `${orderId}-i1`, order_id: orderId, product_id: "prod-book-1", product_type: "book",
  name: "Buku Latihan Olimpiade Matematika", unit_price: 85000, qty: 2, jumlah: 170000, weight_grams: 400,
});
const courseItem = (orderId: string) => ({
  id: `${orderId}-i2`, order_id: orderId, product_id: "prod-course-1", product_type: "course",
  name: "Kursus Aljabar Dasar", unit_price: 150000, qty: 1, jumlah: 150000,
});

const baseOrder = {
  student_id: "stu-1", student_name: "Siswa Uji", student_school: "SMA Contoh 1", student_grade: 10,
  subtotal: 0, discount: 0, shipping_cost: 0, total: 0,
};

const ORDERS = [
  { ...baseOrder, id: PENDING_ID, status: "payment_pending", subtotal: 150000, total: 150000, items: [courseItem(PENDING_ID)],
    created_at: iso(-2 * HOUR), payment_expires_at: iso(22 * HOUR), checked_out_at: iso(-2 * HOUR) },
  { ...baseOrder, id: PROC_ID, status: "processing", subtotal: 170000, shipping_cost: 18000, total: 188000,
    items: [physicalItem(PROC_ID)], created_at: iso(-2 * DAY), paid_at: iso(-2 * DAY + HOUR), payment_method: "bank_transfer",
    tracking_number: RESI, selected_courier: "JNE", selected_service: "REG",
    shipping_address: { penerima: "Siswa Uji", alamat: "Jl. Contoh No. 1", kode_pos: "12345" } },
  { ...baseOrder, id: PAID_ID, status: "completed", subtotal: 320000, discount: 20000, shipping_cost: 0, total: 300000,
    items: [courseItem(PAID_ID), { ...courseItem(PAID_ID), id: `${PAID_ID}-i3`, name: "Tryout Paket Premium", product_type: "exam", jumlah: 170000, unit_price: 170000 }],
    created_at: iso(-10 * DAY), paid_at: iso(-10 * DAY + HOUR), completed_at: iso(-9 * DAY), payment_method: "gopay" },
];

const reg = (o: Record<string, unknown>) => ({
  student_id: "stu-1", token: "tok", card_key: null, checked_in_at: null, attempts_used: 0, status: "registered",
  created_at: iso(-5 * DAY), is_free: false, requires_checkin: false, check_in_window_minutes: null,
  duration_minutes: 90, session_id: null, max_attempts: 1, navigation_policy: "free", timer_mode: "overall",
  section_count: 3, question_count: 40, sections_duration_minutes: null, scheduled_at: null, scheduled_end_at: null, ...o,
});

const REGS = [
  reg({ id: "reg-live", exam_id: "exam-live", exam_title: "Olimpiade Matematika Penyisihan", scheduled_at: iso(-30 * 60_000), scheduled_end_at: iso(2 * HOUR) }),
  reg({ id: "reg-up", exam_id: "exam-reg-1", exam_title: "Olimpiade Sains Babak Awal", requires_checkin: true, check_in_window_minutes: 15, scheduled_at: iso(2 * DAY), scheduled_end_at: iso(2 * DAY + 2 * HOUR) }),
  reg({ id: "reg-sub", exam_id: "exam-sub", exam_title: "Kompetisi Bahasa Inggris Ronde 1", status: "submitted", attempts_used: 1, session_id: "sess-sub-1",
    scheduled_at: iso(-6 * DAY), scheduled_end_at: iso(-6 * DAY + 2 * HOUR),
    external_results: [{ round_id: "r1", round_label: "Ronde Nasional", score: 87.5, rank: 12, award: "Medali Perak", certificate_url: "https://example.test/cert.pdf", published_at: iso(-3 * DAY) }] }),
  reg({ id: "reg-sub2", exam_id: "exam-sub2", exam_title: "Tryout Fisika Mingguan", status: "submitted", attempts_used: 1, session_id: "sess-sub-2",
    scheduled_at: iso(-4 * DAY), scheduled_end_at: iso(-4 * DAY + 2 * HOUR) }),
  reg({ id: "reg-exp", exam_id: "exam-exp", exam_title: "Tryout Kimia Terlewat", scheduled_at: iso(-3 * DAY), scheduled_end_at: iso(-3 * DAY + 2 * HOUR) }),
];

const edition = (n: number, examIds: string[], phase: "open" | "live", title: string) => ({
  product_id: `prod-ed-${n}`, exam_ids: examIds, title, stages: ["SMP", "SMA"], grade_min: 7, grade_max: 12, price: 75000 * n,
  image_url: "", phase, scheduled_at: iso(5 * DAY), scheduled_end_at: iso(5 * DAY + 2 * HOUR), available_until: iso(4 * DAY),
});
const evt = (slug: string, name: string, tone: string, illustration: string, eds: unknown[]) => ({
  slug, name, short_name: name.slice(0, 4), scope: "national", subjects: ["Matematika"], tone, illustration, logo_url: "",
  official_url: null, full_name: name, tagline: "Kompetisi untuk pelajar", levels_label: "SMP/SMA", story: [], rounds: [],
  categories: [], facts: [], schedule: [], open_editions: eds,
});
const EVENTS = [
  evt("omp", "Olimpiade Matematika Pelajar", "brand", "math", [
    edition(1, ["exam-reg-1"], "open", "Babak Awal Terdaftar"),
    edition(2, ["exam-a2"], "open", "Babak Penyisihan 2026"),
    edition(3, ["exam-a3"], "live", "Babak Final 2026"),
  ]),
  evt("osn", "Kompetisi Sains Nusantara", "teal", "science", [edition(4, ["exam-b4"], "open", "Seleksi Sains Wilayah")]),
];

const PRODUCTS = [
  { id: "prod-book-1", type: "book", name: "Buku Latihan Olimpiade Matematika", price: 85000, stock: 20, status: "published", image_url: "" },
  { id: "prod-merch-1", type: "merchandise", name: "Kaos Abak Academy", price: 120000, stock: 5, status: "published", image_url: "" },
  { id: "prod-medal-1", type: "medal", name: "Medali Juara Kustom", price: 45000, stock: 12, status: "published", image_url: "" },
  { id: "prod-exam-1", type: "exam", name: "Tryout Eksklusif", price: 50000, status: "published", image_url: "" },
  { id: "prod-course-2", type: "course", name: "Kursus Geometri", price: 130000, course_ids: ["course-3"], status: "published", image_url: "" },
];

const DASHBOARD = {
  greeting: "",
  enrolled_courses: [
    { id: "course-1", title: "Aljabar Dasar", progress: 0.4, total_lessons: 10, done_lessons: 4 },
    { id: "course-2", title: "Fisika Mekanika", progress: 0.75, total_lessons: 8, done_lessons: 6 },
  ],
  pending_order: { id: PENDING_ID, product: "Kursus Aljabar Dasar", amount: 150000 },
  study_summary: { visited_lectures: 10, total_lectures: 18, enrolled_courses_count: 2, completed_courses: 0, total_minutes: 540 },
  ranking: { position: null, points: null, leaderboard: [] },
  exam_progress: [],
  popular_lessons: [],
};

const USER = { id: "stu-1", role: "student", name: "Siswa Uji", email: "siswa@example.test", username: "siswauji", auth_provider: "password", grade: 10, school_id: "sch-1", phone: "081200000000" };

const unstubbed = new Set<string>();
const consoleErrors: string[] = [];
const json = (route: import("@playwright/test").Route, body: unknown, status = 200) =>
  route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });

interface Opts { theme?: "light" | "dark"; lang?: "id" | "en"; events?: unknown[]; regs?: unknown[]; orders?: unknown[]; dashboard?: unknown }

async function setup(page: Page, context: BrowserContext, o: Opts = {}) {
  await seedSession(context, { token: "tok-student", refreshToken: "ref", user: USER });
  await context.addInitScript(
    (ui) => { if (!window.localStorage.getItem("abak-ui")) window.localStorage.setItem("abak-ui", JSON.stringify({ state: ui, version: 0 })); },
    { theme: o.theme ?? "light", lang: o.lang ?? "id" },
  );
  const tag = test.info().title;
  page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(`[${tag}] ${m.text()}`); });
  page.on("pageerror", (e) => consoleErrors.push(`[${tag}] PAGEERROR ${e.message}`));
  const orders = o.orders ?? ORDERS;
  await page.route("**/api/v1/**", async (route) => {
    const req = route.request();
    const path = new URL(req.url()).pathname.replace(/^.*\/api\/v1/, "");
    const m = req.method();
    if (path === "/auth/me" || path === "/students/profile") return json(route, USER);
    if (path === "/students/dashboard") return json(route, o.dashboard ?? DASHBOARD);
    if (path === "/exam/registrations") return json(route, { data: o.regs ?? REGS });
    if (path === "/orders" && m === "GET") return json(route, { data: orders, next_cursor: "" });
    if (path === "/orders" && m === "POST") return json(route, { ...baseOrder, id: "cart-1", status: "cart", items: [{ ...courseItem("cart-1") }], subtotal: 150000, total: 150000 });
    const od = path.match(/^\/orders\/([^/]+)$/);
    if (od && m === "GET") {
      const found = (orders as { id: string }[]).find((x) => x.id === od[1]);
      return found ? json(route, found) : json(route, { error: "not found" }, 404);
    }
    if (path.startsWith("/event-series")) return json(route, { data: o.events ?? EVENTS });
    if (path === "/products") return json(route, { data: PRODUCTS, next_cursor: "" });
    const rs = path.match(/^\/exam\/sessions\/([^/]+)\/result$/);
    if (rs) return json(route, rs[1] === "sess-sub-1"
      ? { state: "result", result_config: "score_only", score: 80, correct_count: 32, wrong_count: 8, empty_count: 0, rank: 5 }
      : { state: "grading" });
    if (path === "/config/payment-client-key") return json(route, { client_key: "" });
    if (path === "/courses" && m === "GET") return json(route, { data: [
      { id: "cs-1", student_id: "stu-1", course_id: "course-1", status: "active", enrolled_at: iso(-20 * DAY), completed_lessons: {} },
      { id: "cs-2", student_id: "stu-1", course_id: "course-2", status: "active", enrolled_at: iso(-10 * DAY), completed_lessons: {} },
    ] });
    const cd = path.match(/^\/courses\/([^/]+)$/);
    if (cd) return json(route, { id: cd[1], title: cd[1] === "course-1" ? "Aljabar Dasar" : "Fisika Mekanika", level: "SMA", subject: "Matematika", instructor_name: "Pengajar Uji",
      sections: [{ id: "sec1", course_id: cd[1], title: "Bagian 1", lessons: [{ id: "l1", section_id: "sec1", title: "Pengantar" }, { id: "l2", section_id: "sec1", title: "Latihan" }] }] });
    if (path.startsWith("/files/")) return route.fulfill({ status: 404, body: "" });
    if (path.startsWith("/promo-codes/active")) return json(route, { data: [] });
    if (path.startsWith("/provinces") || path.startsWith("/schools")) return json(route, []);
    unstubbed.add(`${m} ${path}`);
    return json(route, {});
  });
}

const shot = (page: Page, name: string, fullPage = true) => page.screenshot({ path: `${OUT}/${name}.png`, fullPage });
const box = async (page: Page, sel: string) => {
  const b = await page.locator(sel).first().boundingBox();
  expect(b, `bbox of ${sel}`).not.toBeNull();
  return b!;
};
const waitHome = async (page: Page) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible({ timeout: 120_000 });
  await expect(page.getByRole("region", { name: /carousel|event/i }).first()).toBeVisible({ timeout: 30_000 });
  await page.waitForTimeout(800);
};

test.use({ channel: "chrome", viewport: { width: 1366, height: 900 }, permissions: ["clipboard-read", "clipboard-write"] });
test.setTimeout(240_000);

test.afterAll(() => {
  writeFileSync(`${OUT}/console-errors.json`, JSON.stringify([...new Set(consoleErrors)], null, 1));
  writeFileSync(`${OUT}/unstubbed.json`, JSON.stringify([...unstubbed], null, 1));
});

for (const theme of ["light", "dark"] as const) {
  test(`1 pages at 1366 ${theme}`, async ({ page, context }) => {
    await setup(page, context, { theme });
    await waitHome(page);
    expect(await page.evaluate(() => document.documentElement.classList.contains("dark") || document.documentElement.dataset.theme === "dark")).toBe(theme === "dark");
    await shot(page, `home-1366-${theme}`);

    const nav = page.locator("aside nav, nav").first();
    for (const label of ["Beranda", "Kompetisi", "Kursus Saya", "Tagihan"]) {
      await expect(page.locator("aside").getByRole("link", { name: new RegExp(`^${label}`) }).first()).toBeVisible();
    }
    const links = await page.locator("aside nav a").allInnerTexts();
    note(`[1 ${theme}] sidebar links: ${JSON.stringify(links)}`);
    expect(links.length).toBe(4);
    const header = page.locator("header").first();
    const headerButtons = await header.getByRole("button").allInnerTexts();
    const headerLabels = await header.locator("button").evaluateAll((els) => els.map((e) => e.getAttribute("aria-label")));
    note(`[1 ${theme}] header buttons labels: ${JSON.stringify(headerLabels)} visible text ${JSON.stringify(headerButtons)}`);
    for (const l of headerLabels) expect(l ?? "").not.toMatch(/mode|notif|cart|keranjang/i);
    void nav;

    // badges (Kompetisi: live count, Tagihan: pending)
    const kom = await page.locator("aside").getByRole("link", { name: /^Kompetisi/ }).innerText();
    const tag = await page.locator("aside").getByRole("link", { name: /^Tagihan/ }).innerText();
    note(`[1 ${theme}] Kompetisi link text=${JSON.stringify(kom)} Tagihan=${JSON.stringify(tag)}`);

    for (const [p, name, wait] of [
      ["/exam", "exam", /Kompetisi|Ujian/i],
      ["/courses", "courses", /Kursus Saya/],
      ["/orders", "orders", /Tagihan|Pesanan/i],
      [`/orders/${PAID_ID}`, "order-detail", /Ringkasan/i],
      ["/cart", "cart", /Keranjang|Cart/i],
    ] as const) {
      await page.goto(p);
      await expect(page.getByText(wait).first()).toBeVisible({ timeout: 90_000 });
      await page.waitForTimeout(800);
      await shot(page, `${name}-1366-${theme}`);
      expect(await page.locator("aside nav a").count()).toBe(4);
    }
  });
}

test("2 user menu and EN", async ({ page, context }) => {
  await setup(page, context);
  await waitHome(page);
  const trigger = page.getByRole("button", { name: "Menu akun" });
  await expect(page.getByTestId("user-menu-dot")).toHaveText("1");
  await trigger.click();
  const menu = page.getByRole("menu");
  await expect(menu).toBeVisible();
  const items = await menu.getByRole("menuitem").allInnerTexts();
  note(`[2] menu items: ${JSON.stringify(items)}`);
  await shot(page, "usermenu-1366-light", false);
  const idx = (re: RegExp) => items.findIndex((s) => re.test(s));
  expect([idx(/Keranjang/), idx(/Mode gelap/), idx(/Bahasa/), idx(/Profil & akun/), idx(/Keluar/)]).toEqual([0, 1, 2, 3, 4]);
  expect(items.some((s) => /Notifikasi/i.test(s))).toBe(false);
  await menu.getByRole("button", { name: "EN" }).click();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("heading", { level: 1 })).toContainText(/Good|Hello|Welcome|Hi/i, { timeout: 10_000 });
  await page.waitForTimeout(600);
  await shot(page, "home-1366-en");
  await page.getByRole("button", { name: "Account menu" }).click();
  note(`[2] EN menu items: ${JSON.stringify(await page.getByRole("menu").getByRole("menuitem").allInnerTexts())}`);
  await shot(page, "usermenu-1366-en", false);
});

test("3 mobile 390 drawer", async ({ page, context }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await setup(page, context);
  await waitHome(page);
  await shot(page, "home-390-light");
  const headerLabels = await page.locator("header").first().locator("button").evaluateAll((els) => els.map((e) => e.getAttribute("aria-label")));
  note(`[3] header buttons at 390: ${JSON.stringify(headerLabels)}`);
  expect(headerLabels).toEqual(["Toggle sidebar"]);
  const hscroll = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  note(`[3] horizontal overflow px at 390 home: ${hscroll}`);
  expect(hscroll).toBe(0);
  await page.getByRole("button", { name: "Toggle sidebar" }).click();
  await page.waitForTimeout(500);
  await shot(page, "drawer-390-light", false);
  await page.getByRole("link", { name: /^Tagihan/ }).first().click();
  await expect(page).toHaveURL(/\/orders$/);
  await page.waitForTimeout(700);
  const x = (await page.locator("aside").first().boundingBox())?.x ?? 0;
  const w = (await page.locator("aside").first().boundingBox())?.width ?? 0;
  note(`[3] aside after nav: x=${x} w=${w}`);
  expect(x + w).toBeLessThanOrEqual(1);
  await shot(page, "orders-390-after-drawer-nav", false);
});

test("4 home layout ratios and stacking", async ({ page, context }) => {
  await setup(page, context);
  await waitHome(page);
  const car = page.getByRole("region", { name: /carousel|event/i }).first();
  const rowA = await car.boundingBox();
  const eventsCard = page.locator("section, div").filter({ has: page.getByRole("heading", { name: "Event & kompetisi" }) }).last();
  const ev = await page.getByRole("heading", { name: "Event & kompetisi" }).locator("xpath=ancestor::*[contains(@class,'rounded')][1]").boundingBox();
  void eventsCard;
  const ratioA = rowA!.width / (rowA!.width + ev!.width);
  note(`[4] RowA carousel w=${rowA!.width} events w=${ev!.width} ratio=${ratioA.toFixed(3)}`);
  expect(Math.abs(ratioA - 0.65)).toBeLessThan(0.03);

  const learn = await page.getByRole("heading", { name: /Lanjut belajar/i }).locator("xpath=ancestor::*[contains(@class,'rounded')][1]").boundingBox();
  const study = await page.getByRole("heading", { name: /jam belajar/i }).locator("xpath=ancestor::*[contains(@class,'rounded')][1]").boundingBox();
  const ratioB = learn!.width / (learn!.width + study!.width);
  note(`[4] RowB learn w=${learn!.width} study w=${study!.width} ratio=${ratioB.toFixed(3)}`);
  expect(Math.abs(ratioB - 0.4)).toBeLessThan(0.03);

  await page.setViewportSize({ width: 1100, height: 900 });
  await page.waitForTimeout(500);
  const car2 = await car.boundingBox();
  const ev2 = await page.getByRole("heading", { name: "Event & kompetisi" }).locator("xpath=ancestor::*[contains(@class,'rounded')][1]").boundingBox();
  const learn2 = await page.getByRole("heading", { name: /Lanjut belajar/i }).locator("xpath=ancestor::*[contains(@class,'rounded')][1]").boundingBox();
  const study2 = await page.getByRole("heading", { name: /jam belajar/i }).locator("xpath=ancestor::*[contains(@class,'rounded')][1]").boundingBox();
  note(`[4] 1100: car(x=${car2!.x},y=${car2!.y}) ev(x=${ev2!.x},y=${ev2!.y}) learn(x=${learn2!.x},y=${learn2!.y}) study(x=${study2!.x},y=${study2!.y})`);
  expect(Math.abs(car2!.x - ev2!.x)).toBeLessThan(2);
  expect(ev2!.y).toBeGreaterThan(car2!.y + 50);
  expect(Math.abs(learn2!.x - study2!.x)).toBeLessThan(2);
  expect(study2!.y).toBeGreaterThan(learn2!.y + 50);
  await shot(page, "home-1100-light");
});

test("5 carousel stable height, inert, autoplay, reduced motion", async ({ page, context }) => {
  await setup(page, context);
  await waitHome(page);
  const car = page.getByRole("region", { name: /carousel|event/i }).first();
  const dots = car.locator("button[aria-label^='Event ']");
  const n = await dots.count();
  note(`[5] slide dots: ${n}`);
  expect(n).toBeGreaterThanOrEqual(2);
  const measure = async () => ({
    h: (await car.boundingBox())!.height,
    dots: await Promise.all(Array.from({ length: n }, async (_, i) => { const b = (await dots.nth(i).boundingBox())!; return [b.x, b.y, b.width, b.height]; })),
  });
  await page.mouse.move(5, 5);
  const before = await measure();
  await shot(page, "carousel-slide1");
  await page.mouse.move(5, 5);
  // hover pauses autoplay, so jump with a click and re-park the pointer
  await dots.nth(1).click();
  await page.mouse.move(5, 5);
  await page.waitForTimeout(1000);
  const after = await measure();
  note(`[5] height before=${before.h} after=${after.h}`);
  note(`[5] dots before=${JSON.stringify(before.dots.map((d) => d.slice(0, 2)))} after=${JSON.stringify(after.dots.map((d) => d.slice(0, 2)))}`);
  await shot(page, "carousel-slide2");
  expect(after.h).toBe(before.h);
  expect(after.dots.map((d) => [d[0], d[1]])).toEqual(before.dots.map((d) => [d[0], d[1]]));
  const inertState = await car.locator("[data-slide]").evaluateAll((els) => els.map((e) => (e as HTMLElement).inert));
  note(`[5] inert flags after click dot 2: ${JSON.stringify(inertState)}`);
  expect(inertState.filter((x) => !x).length).toBe(1);
  expect(inertState[1]).toBe(false);

  const active = () => dots.evaluateAll((els) => els.findIndex((e) => e.getAttribute("aria-current") === "true"));
  await page.mouse.move(5, 5);
  await page.waitForTimeout(150);
  const a0 = await active();
  await page.waitForTimeout(6800);
  const a1 = await active();
  note(`[5] autoplay: active ${a0} -> ${a1} after 6.8s`);
  expect(a1).not.toBe(a0);

  await car.hover();
  const h0 = await active();
  await page.waitForTimeout(7000);
  const h1 = await active();
  note(`[5] hover pause: ${h0} -> ${h1} after 7s hover`);
  expect(h1).toBe(h0);
});

test("5b reduced motion no autoplay", async ({ page, context }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await setup(page, context);
  await waitHome(page);
  const car = page.getByRole("region", { name: /carousel|event/i }).first();
  const dots = car.locator("button[aria-label^='Event ']");
  await page.mouse.move(5, 5);
  const active = () => dots.evaluateAll((els) => els.findIndex((e) => e.getAttribute("aria-current") === "true"));
  const a0 = await active();
  await page.waitForTimeout(7500);
  const a1 = await active();
  note(`[5b] reduced-motion: ${a0} -> ${a1} after 7.5s`);
  expect(a1).toBe(a0);
});

test("6 registered edition absent, shipping strip, resi copy, fallback", async ({ page, context }) => {
  await setup(page, context);
  await waitHome(page);
  const car = page.getByRole("region", { name: /carousel|event/i }).first();
  const text = await car.innerText();
  const labels = await car.locator("button[aria-label^='Event ']").evaluateAll((els) => els.map((e) => e.getAttribute("aria-label")));
  note(`[6] dots: ${JSON.stringify(labels)}`);
  expect(text).not.toContain("Babak Awal Terdaftar");
  expect(text).toContain("Babak Penyisihan 2026");
  expect(labels.length).toBe(3);
  const strip = page.getByRole("list", { name: /pengiriman|progress|status/i }).first();
  const steps = page.locator("li[data-reached]");
  expect(await steps.count()).toBe(4);
  note(`[6] ship steps reached: ${JSON.stringify(await steps.evaluateAll((e) => e.map((x) => x.getAttribute("data-reached"))))}`);
  void strip;
  await page.getByRole("button", { name: "Salin resi" }).click();
  const clip = await page.evaluate(() => navigator.clipboard.readText());
  note(`[6] clipboard=${clip}`);
  expect(clip).toBe(RESI);
  await shot(page, "shipping-strip-1366", false);

  const noReg = REGS.filter((r) => (r as { id?: string }).id !== "reg-up");
  await page.unroute("**/api/v1/**");
  await setup(page, context, { events: [], regs: noReg });
  await page.reload();
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible({ timeout: 60_000 });
  await page.waitForTimeout(900);
  const car2 = page.getByRole("region", { name: /carousel|event/i }).first();
  expect(await car2.locator("button[aria-label^='Event ']").count()).toBe(0);
  await shot(page, "carousel-fallback-1366");
});

test("7-9 order detail, invoice, pdf, print", async ({ page, context }) => {
  await setup(page, context);
  await page.goto(`/orders/${PAID_ID}`);
  await expect(page.getByText("Ringkasan Pesanan")).toBeVisible({ timeout: 120_000 });
  const sum = await page.getByRole("heading", { name: "Ringkasan Pesanan" }).locator("xpath=ancestor::*[contains(@class,'rounded')][1]").boundingBox();
  const hist = await page.getByRole("heading", { name: "Riwayat status" }).locator("xpath=ancestor::*[contains(@class,'rounded')][1]").boundingBox();
  note(`[7] Ringkasan x=${sum!.x} w=${sum!.width} | Riwayat x=${hist!.x} w=${hist!.width}`);
  expect(sum!.width).toBeGreaterThan(hist!.width);
  expect(sum!.x).toBeLessThan(hist!.x);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(500);
  const sum2 = await page.getByRole("heading", { name: "Ringkasan Pesanan" }).locator("xpath=ancestor::*[contains(@class,'rounded')][1]").boundingBox();
  const hist2 = await page.getByRole("heading", { name: "Riwayat status" }).locator("xpath=ancestor::*[contains(@class,'rounded')][1]").boundingBox();
  note(`[7] 390: Ringkasan(x=${sum2!.x},y=${sum2!.y}) Riwayat(x=${hist2!.x},y=${hist2!.y})`);
  expect(Math.abs(sum2!.x - hist2!.x)).toBeLessThan(2);
  expect(hist2!.y).toBeGreaterThan(sum2!.y);
  await shot(page, "order-detail-390-light");
  await page.setViewportSize({ width: 1366, height: 900 });
  await page.waitForTimeout(400);

  // paid invoice: light then dark
  await page.getByRole("button", { name: "Lihat invoice" }).click();
  const paper = page.locator(".invoice-paper");
  await expect(paper).toBeVisible();
  await expect(paper).toContainText("LUNAS");
  await page.waitForTimeout(600);
  await shot(page, "invoice-paid-1366-light", false);
  const bgLight = await paper.evaluate((e) => getComputedStyle(e).backgroundColor);
  const pbox = await paper.boundingBox();
  note(`[7] paper light bg=${bgLight} box=${JSON.stringify(pbox)}`);

  // 8: download
  const reqs: string[] = [];
  page.on("request", (r) => reqs.push(r.url()));
  const [dl] = await Promise.all([
    page.waitForEvent("download", { timeout: 60_000 }),
    page.getByRole("button", { name: "Unduh PDF" }).click(),
  ]);
  const fname = dl.suggestedFilename();
  await dl.saveAs(`${OUT}/invoice.pdf`);
  note(`[8] filename=${fname}; requests during click: ${JSON.stringify(reqs.filter((u) => !u.startsWith("blob:")))}`);
  expect(fname).toBe(`Invoice-${PAID_ID.slice(-8)}.pdf`);
  const buf = readFileSync(`${OUT}/invoice.pdf`);
  const s = buf.toString("latin1");
  note(`[8] pdf bytes=${buf.length} head=${JSON.stringify(s.slice(0, 8))} Count=${/\/Count (\d+)/.exec(s)?.[1]} MediaBox=${/\/MediaBox \[([^\]]+)\]/.exec(s)?.[1]}`);
  expect(s.startsWith("%PDF")).toBe(true);
  expect(s).toContain("/Count 1");
  const mb = /\/MediaBox \[([^\]]+)\]/.exec(s)![1].split(/\s+/).map(Number);
  expect(Math.abs(mb[2] - 595.28)).toBeLessThan(1);
  expect(Math.abs(mb[3] - 841.89)).toBeLessThan(1);
  try { execFileSync("sips", ["-s", "format", "png", `${OUT}/invoice.pdf`, "--out", `${OUT}/invoice-pdf.png`]); } catch (e) { note(`[8] sips failed ${e}`); }
  await expect(page.getByRole("button", { name: "Unduh PDF" })).toBeEnabled();

  // 9: print media
  await page.emulateMedia({ media: "print" });
  await page.waitForTimeout(400);
  await shot(page, "invoice-print-media", false);
  const visibleBits = await page.evaluate(() => {
    const vis = (e: Element) => { const r = e.getBoundingClientRect(); const cs = getComputedStyle(e); return r.width > 0 && r.height > 0 && cs.display !== "none" && cs.visibility !== "hidden"; };
    return {
      paper: vis(document.querySelector(".invoice-paper")!),
      aside: !!document.querySelector("aside") && vis(document.querySelector("aside")!),
      header: !!document.querySelector("header:not(.invoice-paper header)") && Array.from(document.querySelectorAll("body > * header")).some((h) => !h.closest(".invoice-paper") && vis(h)),
      footerButtons: Array.from(document.querySelectorAll("button")).filter((b) => /Cetak|Unduh/.test(b.textContent ?? "") && vis(b)).length,
    };
  });
  note(`[9] print visibility: ${JSON.stringify(visibleBits)}`);
  const chain = await page.evaluate(() => {
    const out: string[] = [];
    for (let e: Element | null = document.querySelector(".invoice-paper"); e; e = e.parentElement) {
      const cs = getComputedStyle(e); const r = e.getBoundingClientRect();
      out.push(`${e.tagName}.${(e.getAttribute("class") ?? "").slice(0, 30)}[${e.getAttribute("data-slot") ?? ""}] disp=${cs.display} vis=${cs.visibility} op=${cs.opacity} pos=${cs.position} box=${Math.round(r.x)},${Math.round(r.y)},${Math.round(r.width)}x${Math.round(r.height)}`);
    }
    return out;
  });
  note(`[9] print ancestor chain:\n  ${chain.join("\n  ")}`);
  await page.pdf({ path: `${OUT}/print.pdf`, format: "A4", printBackground: true });
  const ps = readFileSync(`${OUT}/print.pdf`).toString("latin1");
  const pageObjs = (ps.match(/\/Type\s*\/Page[^s]/g) ?? []).length;
  note(`[9] print.pdf page objects=${pageObjs} Count=${/\/Count (\d+)/.exec(ps)?.[1]}`);
  try { execFileSync("sips", ["-s", "format", "png", `${OUT}/print.pdf`, "--out", `${OUT}/print-pdf.png`]); } catch (e) { note(`[9] sips failed ${e}`); }
  expect(pageObjs).toBe(1);
  await page.emulateMedia({ media: "screen" });
  await page.keyboard.press("Escape");

  // dark: paper must remain light
  await page.evaluate(() => {
    const s = JSON.parse(localStorage.getItem("abak-ui")!);
    s.state.theme = "dark";
    localStorage.setItem("abak-ui", JSON.stringify(s));
  });
  await page.reload();
  await expect(page.getByText("Ringkasan Pesanan")).toBeVisible({ timeout: 60_000 });
  expect(await page.evaluate(() => document.documentElement.classList.contains("dark") || document.documentElement.dataset.theme === "dark")).toBe(true);
  await page.getByRole("button", { name: "Lihat invoice" }).click();
  await expect(page.locator(".invoice-paper")).toBeVisible();
  await page.waitForTimeout(600);
  const bgDark = await page.locator(".invoice-paper").evaluate((e) => getComputedStyle(e).backgroundColor);
  const fgDark = await page.locator(".invoice-paper").evaluate((e) => getComputedStyle(e).color);
  note(`[7] paper dark bg=${bgDark} color=${fgDark}`);
  await shot(page, "invoice-paid-1366-dark", false);
  expect(bgDark).toBe(bgLight);
  await page.emulateMedia({ media: "print" });
  await page.waitForTimeout(300);
  await shot(page, "invoice-print-media-dark", false);
  await page.pdf({ path: `${OUT}/print-dark.pdf`, format: "A4", printBackground: true });
  const pd = readFileSync(`${OUT}/print-dark.pdf`).toString("latin1");
  note(`[9] print-dark page objects=${(pd.match(/\/Type\s*\/Page[^s]/g) ?? []).length}`);
  try { execFileSync("sips", ["-s", "format", "png", `${OUT}/print-dark.pdf`, "--out", `${OUT}/print-dark-pdf.png`]); } catch { /* */ }
  await page.emulateMedia({ media: "screen" });

  // pending order: BELUM DIBAYAR
  await page.goto(`/orders/${PENDING_ID}`);
  await page.getByRole("button", { name: "Lihat invoice" }).click();
  await expect(page.locator(".invoice-paper")).toContainText("BELUM DIBAYAR");
  await page.waitForTimeout(500);
  await shot(page, "invoice-pending-1366-dark", false);

  // 14: hidden for cancelled/expired
  for (const st of ["cancelled", "payment_expired"]) {
    await page.unroute("**/api/v1/**");
    await setup(page, context, { orders: [{ ...ORDERS[0], id: `ord-${st}-zzzz9999`, status: st }] });
    await page.goto(`/orders/ord-${st}-zzzz9999`);
    await page.waitForTimeout(2000);
    const n = await page.getByRole("button", { name: "Lihat invoice" }).count();
    note(`[14] ${st}: invoice button count=${n}`);
    expect(n).toBe(0);
  }
});
