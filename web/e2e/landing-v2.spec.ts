import { test, expect, type Browser, type BrowserContext, type Page, type Route } from "@playwright/test";
import { seedSession } from "./helpers/session";

const DAY = 24 * 3600_000;
const iso = (offsetMs: number) => new Date(Date.now() + offsetMs).toISOString();
const json = (route: Route, body: unknown, status = 200) =>
  route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });

const edition = (n: number, phase: "open" | "live", title: string) => ({
  product_id: `prod-ed-${n}`, exam_ids: [`exam-${n}`], title, stages: ["SMP", "SMA"], grade_min: 7, grade_max: 12, price: 75000 * n,
  image_url: "", phase, scheduled_at: iso(5 * DAY), scheduled_end_at: iso(5 * DAY + 7200_000), available_until: iso(4 * DAY),
});
const evt = (slug: string, name: string, tone: string, illustration: string, eds: unknown[]) => ({
  slug, name, short_name: name.slice(0, 4), scope: "national", subjects: ["Matematika"], tone, illustration, logo_url: "",
  official_url: null, full_name: name, tagline: "Kompetisi untuk pelajar", levels_label: "SMP/SMA", story: [], rounds: [],
  categories: [], facts: [], schedule: [], open_editions: eds,
});
const EVENTS = [
  evt("omp", "Olimpiade Matematika Pelajar", "brand", "math", [edition(1, "open", "Babak Awal"), edition(2, "open", "Babak Penyisihan")]),
  evt("osn", "Kompetisi Sains Nusantara", "teal", "science", []),
];

const BOOK = { id: "prod-book-1", type: "book", name: "Buku Latihan Olimpiade", price: 85000, stock: 5, status: "published", image_url: "", weight_grams: 400 };
const PRODUCTS = [
  BOOK,
  { id: "prod-merch-1", type: "merchandise", name: "Kaos Abak", price: 120000, stock: 5, status: "published", image_url: "" },
  { id: "prod-course-1", type: "course", name: "Kursus Aljabar", price: 150000, course_ids: ["course-1"], status: "published", image_url: "" },
];
const GROUPINGS = [{ id: "grp-1", name: "Olimpiade", order: 1, status: "published" }];

const USER = { id: "stu-1", role: "student", name: "Siswa Uji", email: "siswa@example.test", username: "siswauji", auth_provider: "password", grade: 10, school_id: "sch-1", phone: "081200000000" };
const PENDING_ORDER = {
  id: "ord-pending-1", student_id: "stu-1", student_name: "Siswa Uji", student_school: "SMA Contoh 1", student_grade: 10,
  status: "payment_pending", subtotal: 150000, discount: 0, shipping_cost: 0, total: 150000, created_at: iso(-3600_000),
  payment_expires_at: iso(20 * 3600_000), checked_out_at: iso(-3600_000),
  items: [{ id: "i1", order_id: "ord-pending-1", product_id: "prod-course-1", product_type: "course", name: "Kursus Aljabar", unit_price: 150000, qty: 1, jumlah: 150000 }],
};

const requested: string[] = [];

async function mockApi(page: Page, o: { orders?: unknown[] } = {}) {
  page.on("request", (r) => {
    const u = new URL(r.url());
    if (u.pathname.includes("/api/v1/")) requested.push(`${r.method()} ${u.pathname.replace(/^.*\/api\/v1/, "")}${u.search}`);
  });
  await page.route("**/api/v1/**", async (route) => {
    const req = route.request();
    const path = new URL(req.url()).pathname.replace(/^.*\/api\/v1/, "");
    const m = req.method();
    if (path === "/auth/me" || path === "/students/profile") return json(route, USER);
    if (path === "/exam/registrations") return json(route, { data: [] });
    if (path === "/orders" && m === "GET") return json(route, { data: o.orders ?? [], next_cursor: "" });
    if (path.startsWith("/event-series")) return json(route, { data: EVENTS });
    if (path.startsWith("/testimonials")) return json(route, { data: [] });
    if (path === "/products") return json(route, { data: PRODUCTS, next_cursor: "" });
    const pd = path.match(/^\/products\/([^/]+)$/);
    if (pd) {
      const p = PRODUCTS.find((x) => x.id === pd[1]);
      return p ? json(route, p) : json(route, { error: "not found" }, 404);
    }
    if (path === "/product-groupings") return json(route, { data: GROUPINGS });
    if (path.startsWith("/files/")) return route.fulfill({ status: 404, body: "" });
    if (path.startsWith("/promo-codes/active")) return json(route, { data: [] });
    return json(route, {});
  });
}

const signIn = (context: BrowserContext) =>
  seedSession(context, { token: "tok-student", refreshToken: "ref", user: USER });

test.use({ channel: "chrome", viewport: { width: 1366, height: 900 } });
test.setTimeout(120_000);
test.beforeEach(() => { requested.length = 0; });

const gotoReady = async (page: Page, path: string) => {
  await page.goto(path);
  await expect(page.locator("footer")).toBeAttached({ timeout: 60_000 });
};

test("AC2 prerendered / has landing copy without JavaScript", async ({ browser }: { browser: Browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  const page = await context.newPage();
  await page.goto("/");
  const html = await page.content();
  const text = await page.evaluate(() => document.body.innerText);
  for (const copy of [
    "Dari ruang kelas ke panggung",
    "Program Abak",
    "Setiap anak punya kapasitas, potensi, dan bakat yang unik.",
    "Masih ragu? Kami jawab.",
    "Medali berikutnya bisa jadi milikmu.",
  ]) expect(html, copy).toContain(copy);
  expect(text).not.toContain("Memuat…");
  await context.close();
});

const HERO_SAMPLER = `
  window.__heroFrames = { frames: 0, visible: 0 };
  const tick = () => {
    const h = document.querySelector("[data-hero]");
    window.__heroFrames.frames++;
    if (h) {
      const cs = getComputedStyle(h);
      const r = h.getBoundingClientRect();
      if (cs.display !== "none" && cs.visibility !== "hidden" && r.width > 0 && r.height > 0) window.__heroFrames.visible++;
    }
    requestAnimationFrame(tick);
  };
  requestAnimationFrame(tick);
`;

test("AC3 stored student session never paints the landing hero on /", async ({ page, context }) => {
  await signIn(context);
  await context.addInitScript(HERO_SAMPLER);
  await mockApi(page);
  await page.goto("/");
  await page.waitForTimeout(2500);
  const s = await page.evaluate(() => (window as unknown as { __heroFrames: { frames: number; visible: number } }).__heroFrames);
  expect(s.frames).toBeGreaterThan(30);
  expect(s.visible).toBe(0);
});

test("AC3 control: the sampler does see the hero for a guest", async ({ page, context }) => {
  await context.addInitScript(HERO_SAMPLER);
  await mockApi(page);
  await page.goto("/");
  await expect(page.locator("[data-hero]")).toBeVisible({ timeout: 60_000 });
  await page.waitForTimeout(500);
  const s = await page.evaluate(() => (window as unknown as { __heroFrames: { visible: number } }).__heroFrames);
  expect(s.visible).toBeGreaterThan(5);
});

const NAV_PAGES = ["/event", "/catalog", "/program"];

for (const path of NAV_PAGES) {
  test(`AC4 student sees the public nav and content on ${path}`, async ({ page, context }) => {
    await signIn(context);
    await mockApi(page, { orders: [PENDING_ORDER] });
    await gotoReady(page, path);
    const nav = page.getByRole("navigation", { name: "Utama" });
    await expect(nav).toBeVisible();
    const dash = nav.getByRole("link", { name: /Dashboard/ });
    await expect(dash).toBeVisible();
    await expect(dash).toContainText("1 perlu perhatian");
    await expect(page.locator("main h1").first()).toBeVisible();
    await expect(page.locator("footer")).toBeVisible();
    await expect(nav.getByRole("link", { name: "Masuk" })).toHaveCount(0);
  });
}

test("AC4 student with nothing pending gets a Dashboard button without a count", async ({ page, context }) => {
  await signIn(context);
  await mockApi(page);
  await gotoReady(page, "/event");
  const dash = page.getByRole("navigation", { name: "Utama" }).getByRole("link", { name: /Dashboard/ });
  await expect(dash).toBeVisible();
  await expect(dash).not.toContainText("perlu perhatian");
});

for (const lang of ["id", "en"] as const) {
  test(`AC4 guest nav in ${lang}: links, auth buttons, no "Katalog"`, async ({ page, context }) => {
    await context.addInitScript((l) => window.localStorage.setItem("abak-ui", JSON.stringify({ state: { theme: "light", lang: l }, version: 0 })), lang);
    await mockApi(page);
    const labels = lang === "id"
      ? { links: ["Event", "Program", "Toko"], login: "Masuk", signup: "Daftar gratis" }
      : { links: ["Events", "Programs", "Shop"], login: "Log in", signup: "Sign up free" };
    for (const path of ["/", "/event", "/program", "/catalog"]) {
      await gotoReady(page, path);
      const nav = page.getByRole("navigation").first();
      for (const l of labels.links) await expect(nav.getByRole("link", { name: l, exact: true })).toBeVisible();
      await expect(nav.getByRole("link", { name: labels.login, exact: true })).toBeVisible();
      await expect(nav.getByRole("link", { name: labels.signup, exact: true })).toBeVisible();
      await expect(nav.getByRole("link", { name: /Dashboard/ })).toHaveCount(0);
      expect(await page.evaluate(() => document.body.innerText), path).not.toMatch(/katalog/i);
    }
  });
}

test("AC5 nav state: top, pill, hide on scroll down, show on 5px up and on focus", async ({ page }) => {
  await mockApi(page);
  await gotoReady(page, "/");
  const header = page.locator("header[data-state]");
  await expect(header).toHaveAttribute("data-state", "top");
  await expect(header).toHaveAttribute("data-hidden", "false");

  const scrollTo = async (y: number) => {
    await page.evaluate((v) => window.scrollTo(0, v), y);
    await page.waitForTimeout(150);
  };
  await scrollTo(9);
  await expect(header).toHaveAttribute("data-state", "pill");
  await expect(header).toHaveAttribute("data-hidden", "false");

  const heroH = await page.locator("[data-hero]").evaluate((e) => (e as HTMLElement).offsetHeight);
  await scrollTo(heroH + 400);
  await expect(header).toHaveAttribute("data-hidden", "true");
  await scrollTo(heroH + 395);
  await expect(header).toHaveAttribute("data-hidden", "false");

  await scrollTo(heroH + 900);
  await expect(header).toHaveAttribute("data-hidden", "true");
  await header.getByRole("link", { name: "Program", exact: true }).focus();
  await expect(header).toHaveAttribute("data-hidden", "false");

  await scrollTo(0);
  await expect(header).toHaveAttribute("data-state", "top");
});

const REDUCED = { reducedMotion: "reduce" as const };
const COARSE = { hasTouch: true, isMobile: true, viewport: { width: 390, height: 844 } };

for (const [name, opts] of [["reduced motion", REDUCED], ["coarse pointer", COARSE]] as const) {
  test(`AC16 ${name}: no swarm, no hidden reveals, no parallax, content visible`, async ({ browser }) => {
    const context = await browser.newContext(opts);
    const page = await context.newPage();
    await mockApi(page);
    await gotoReady(page, "/");
    if (name === "coarse pointer") expect(await page.evaluate(() => matchMedia("(pointer: coarse)").matches)).toBe(true);
    await page.mouse.move(200, 200);
    await page.evaluate(() => window.scrollTo(0, 300));
    await page.waitForTimeout(1200);
    await expect(page.locator("#pm-swarm")).toHaveCount(0);
    await expect(page.locator(".pm-reveal:not(.pm-in)")).toHaveCount(0);
    const photoTransform = await page.locator("[data-hero-photo]").evaluate((e) => getComputedStyle(e).transform);
    expect(photoTransform === "none" || photoTransform === "matrix(1, 0, 0, 1, 0, 0)").toBe(true);
    const sections = await page.locator("main section").evaluateAll((els) => els.map((e) => getComputedStyle(e).opacity));
    expect(sections.length).toBeGreaterThan(5);
    expect(sections.every((o) => o === "1")).toBe(true);
    await context.close();
  });
}

test("AC16 control: desktop with a fine pointer does get the swarm and reveal classes", async ({ page }) => {
  await mockApi(page);
  await gotoReady(page, "/");
  await page.mouse.move(300, 300);
  await expect(page.locator("#pm-swarm")).toHaveCount(1);
  expect(await page.locator("section.pm-reveal").count()).toBeGreaterThan(0);
});

test("AC17 contextmenu is prevented on public pages only", async ({ page, context }) => {
  await mockApi(page);
  const prevented = () =>
    page.evaluate(() => {
      const e = new MouseEvent("contextmenu", { bubbles: true, cancelable: true });
      (document.querySelector("main") ?? document.body).dispatchEvent(e);
      return e.defaultPrevented;
    });
  await gotoReady(page, "/event");
  expect(await prevented()).toBe(true);
  await page.goto("/login");
  await expect(page.locator("main, form").first()).toBeVisible();
  expect(await prevented()).toBe(false);

  await signIn(context);
  await page.goto("/orders");
  await page.waitForTimeout(1500);
  expect(await page.evaluate(() => {
    const e = new MouseEvent("contextmenu", { bubbles: true, cancelable: true });
    document.body.dispatchEvent(e);
    return e.defaultPrevented;
  })).toBe(false);
});

// Content box of the last block: the 80px spacing is its pb-20, so padding counts as the gap.
const gapToFooter = (page: Page, countPadding: boolean) =>
  page.evaluate((withPadding) => {
    const main = document.querySelector("main")!;
    const blocks = [...main.querySelectorAll("section"), main.lastElementChild!] as HTMLElement[];
    const last = blocks.reduce((a, b) => (b.getBoundingClientRect().bottom > a.getBoundingClientRect().bottom ? b : a));
    const cs = getComputedStyle(last);
    const pad = withPadding ? parseFloat(cs.paddingBottom) + parseFloat(cs.borderBottomWidth) : 0;
    return document.querySelector("footer")!.getBoundingClientRect().top - last.getBoundingClientRect().bottom + pad;
  }, countPadding);

for (const path of ["/event", "/program", "/catalog", "/cart"]) {
  test(`AC18 80px between the last section and the footer on ${path}`, async ({ page }) => {
    await mockApi(page);
    await gotoReady(page, path);
    await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
    await page.waitForTimeout(1500);
    expect(Math.abs((await gapToFooter(page, true)) - 80)).toBeLessThanOrEqual(1);
  });
}

test("AC18 landing CTA band touches the footer", async ({ page }) => {
  await mockApi(page);
  await gotoReady(page, "/");
  await expect(page.getByRole("heading", { name: "Medali berikutnya bisa jadi milikmu." })).toBeAttached();
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
  await page.waitForTimeout(1500);
  expect(Math.abs(await gapToFooter(page, false))).toBeLessThanOrEqual(1);
});

test("AC19 switching to EN sets html lang and requests ?lang=en", async ({ page }) => {
  await mockApi(page);
  await gotoReady(page, "/");
  await expect(page.locator("html")).toHaveAttribute("lang", "id");
  expect(requested).toContain("GET /event-series?lang=id");
  await page.getByRole("button", { name: "EN", exact: true }).first().click();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect.poll(() => requested.includes("GET /event-series?lang=en")).toBe(true);
  expect(requested).toContain("GET /testimonials?lang=en");
  await expect(page.getByRole("heading", { name: "The next medal could be yours." })).toBeAttached();
});

test("guest journey: Toko, product, add, cart, Masuk untuk bayar goes to login with next", async ({ page }) => {
  await mockApi(page);
  await gotoReady(page, "/");
  await page.getByRole("navigation").first().getByRole("link", { name: "Toko", exact: true }).click();
  await expect(page).toHaveURL(/\/catalog$/);
  await page.getByRole("link", { name: /Buku Latihan Olimpiade/ }).first().click();
  await expect(page).toHaveURL(/\/catalog\/prod-book-1$/);
  await page.getByRole("button", { name: "Tambah ke Keranjang" }).click();
  await expect(page.getByText("Ditambahkan ke keranjang")).toBeVisible();
  await page.getByRole("button", { name: "Lihat keranjang" }).click();
  await expect(page).toHaveURL(/\/cart$/);
  await expect(page.getByText("Buku Latihan Olimpiade")).toBeVisible();
  const pay = page.getByRole("link", { name: "Masuk untuk bayar" });
  await expect(pay).toBeVisible();
  await pay.click();
  await expect(page).toHaveURL(/\/login\?next=(\/cart|%2Fcart)$/);
  expect(requested.filter((r) => r.startsWith("POST /orders"))).toEqual([]);
});
