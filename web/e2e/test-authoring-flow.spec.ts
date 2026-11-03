import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

const VIEWPORTS = [
  { width: 360, height: 740 },
  { width: 390, height: 844 },
  { width: 768, height: 900 },
  { width: 1024, height: 800 },
  { width: 1440, height: 900 },
];

const UNBROKEN = "https://example.test/" + "segmen-panjang-tanpa-spasi-".repeat(8);
const WIDE_IMAGE =
  "data:image/svg+xml;utf8," +
  encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="2400" height="300"><rect width="2400" height="300" fill="#ccd"/></svg>');
const MATERIAL = `<p>Bacaan ${UNBROKEN}</p><table><tr><td>${"sel ".repeat(40)}</td><td>b</td></tr></table><img src="${WIDE_IMAGE}" alt="gambar lebar">`;

const TOPICS = [
  { id: "topic-1", name: "Aljabar", subject: "Matematika" },
  { id: "topic-2", name: "Reading", subject: "Bahasa Inggris" },
];

const TEST = {
  id: "t1",
  title: "Tryout dengan judul yang cukup panjang untuk uji tata letak di layar kecil",
  subject: "Matematika",
  topic: "Aljabar",
  duration_minutes: 90,
  question_count: 2,
  audio_url: "https://cdn.test/a.mp3",
  shared_material_html: MATERIAL,
  instructions_html: "<p>Kerjakan dengan teliti</p>",
};

function question(id: string, body: string, format = "mcq", extra: Record<string, unknown> = {}) {
  return {
    question: { id, format, body, sort_order: 1, point_correct: 1, point_wrong: 0, topic: "Aljabar", ...extra },
    options: format === "mcq"
      ? [
          { question_id: id, key: "a", text: "Jakarta", is_correct: true, sort_order: 1 },
          { question_id: id, key: "b", text: UNBROKEN, is_correct: false, sort_order: 2 },
        ]
      : [],
    attached_count: 0,
  };
}

const ATTACHED = [question("q1", `<p>Soal terpasang ${UNBROKEN}</p>`), question("q2", "<p>Soal esai</p>", "essay")];
const BANK = [
  question("b1", `<p>${"Teks bank yang sangat panjang ".repeat(12)}${UNBROKEN}</p>`),
  question("b2", "<p>Soal bank pendek</p>", "essay"),
];

interface Calls {
  tests: URL[];
  mutations: string[];
}

async function setup(page: Page, context: Parameters<typeof seedSession>[0], opts: { listFails?: { value: boolean } } = {}) {
  await seedSession(context, {
    token: "e2e-fake-token",
    refreshToken: "e2e-fake-refresh",
    user: { id: "e2e-admin", email: "e2e-admin@example.test", name: "E2E Admin", role: "admin_exam" },
  });
  const calls: Calls = { tests: [], mutations: [] };
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace(/^.*\/api\/v1/, "");
    if (request.method() !== "GET") calls.mutations.push(`${request.method()} ${path}`);
    if (path === "/admin/topics") return route.fulfill({ json: { data: TOPICS } });
    if (path === "/admin/tests" && request.method() === "GET") {
      calls.tests.push(url);
      if (opts.listFails?.value) return route.fulfill({ status: 500, json: { error: "gagal memuat tes" } });
      const cursor = url.searchParams.get("cursor");
      const q = url.searchParams.get("q");
      const rows = q ? [] : [{ ...TEST, id: cursor ? "t2" : "t1" }];
      return route.fulfill({ json: { data: rows, total: q ? 0 : 2, next_cursor: cursor || q ? undefined : "c2" } });
    }
    if (path === "/admin/tests/t1") return route.fulfill({ json: { test: TEST, questions: ATTACHED } });
    if (path === "/admin/tests/t1/questions") return route.fulfill({ json: { data: ATTACHED } });
    if (path === "/admin/questions") return route.fulfill({ json: { data: BANK, next_cursor: "more" } });
    return route.fulfill({ json: { data: [] } });
  });
  return calls;
}

async function noSidewaysScroll(page: Page) {
  const overflowing = await page.evaluate(() => {
    const limit = document.documentElement.clientWidth;
    if (document.documentElement.scrollWidth <= limit) return [];
    return Array.from(document.body.querySelectorAll<HTMLElement>("*"))
      .filter((el) => el.getBoundingClientRect().right > limit + 1)
      .slice(0, 5)
      .map((el) => `${el.tagName}.${String(el.className).slice(0, 60)}`);
  });
  if (overflowing.length) console.log("overflow:", overflowing.join(" | "));
  return overflowing.length === 0;
}

async function dialogFits(page: Page) {
  return page.evaluate(() => {
    const el = document.querySelector('[role="dialog"]') as HTMLElement | null;
    return !!el && el.clientWidth > 0 && el.scrollWidth <= el.clientWidth;
  });
}

for (const viewport of VIEWPORTS) {
  test(`tes list and detail fit the viewport — ${viewport.width}x${viewport.height}`, async ({ page, context }) => {
    await setup(page, context);
    await page.setViewportSize(viewport);

    await page.goto("/admin/exam/tests");
    await expect(page.getByTestId("test-row").first()).toBeVisible();
    expect(await noSidewaysScroll(page)).toBe(true);

    await page.goto("/admin/exam/tests/t1");
    await expect(page.getByRole("tab", { name: "Soal" })).toBeVisible();
    await expect(page.getByText(/Soal terpasang/)).toBeVisible();
    expect(await noSidewaysScroll(page)).toBe(true);
    const sidebar = page.getByTestId("test-material-sidebar");
    await expect(sidebar.getByRole("img", { name: "gambar lebar" })).toBeVisible();
    expect(await sidebar.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true);

    await page.getByRole("tab", { name: /materi & instruksi/i }).click();
    expect(await noSidewaysScroll(page)).toBe(true);
    await page.getByRole("tab", { name: /informasi tes/i }).click();
    await expect(page.getByLabel(/judul/i)).toBeVisible();
    expect(await noSidewaysScroll(page)).toBe(true);

    await page.getByRole("tab", { name: "Soal" }).click();
    await page.getByRole("button", { name: /dari bank/i }).first().click();
    await expect(page.getByText(/Teks bank yang sangat panjang/).first()).toBeVisible();
    await expect(page.getByRole("button", { name: "Lihat selengkapnya" }).first()).toBeVisible();
    expect(await dialogFits(page)).toBe(true);
    expect(await noSidewaysScroll(page)).toBe(true);
    await page.keyboard.press("Escape");

    await page.getByRole("button", { name: /pratinjau tes/i }).click();
    await expect(page.getByText(/tidak ada sesi atau percobaan/i)).toBeVisible();
    expect(await dialogFits(page)).toBe(true);
    expect(await noSidewaysScroll(page)).toBe(true);
  });
}

test("picker: expand stays inside the row and does not select; selection counts survive", async ({ page, context }) => {
  await setup(page, context);
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/admin/exam/tests/t1");
  await page.getByRole("button", { name: /dari bank/i }).first().click();

  const dialog = page.getByRole("dialog");
  const body = dialog.getByText(/Teks bank yang sangat panjang/).first();
  const clamped = await body.evaluate((el) => el.scrollHeight > el.clientHeight);
  expect(clamped).toBe(true);
  await dialog.getByRole("button", { name: "Lihat selengkapnya" }).first().click();
  expect(await body.evaluate((el) => el.scrollHeight <= el.clientHeight + 1)).toBe(true);
  await expect(dialog.getByRole("button", { name: /tambahkan 0 soal/i })).toBeDisabled();

  await dialog.getByText("Soal bank pendek").click();
  await expect(dialog.getByText("1 dipilih")).toBeVisible();
});

test("list: search restarts at page 1 and a failed load can be retried", async ({ page, context }) => {
  const failing = { value: true };
  const calls = await setup(page, context, { listFails: failing });
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/admin/exam/tests");
  await expect(page.getByText("gagal memuat tes")).toBeVisible();

  failing.value = false;
  await page.getByRole("button", { name: /coba lagi/i }).click();
  await expect(page.getByTestId("test-row").first()).toBeVisible();

  await page.getByRole("button", { name: /muat lebih banyak/i }).click();
  await expect.poll(() => calls.tests.some((u) => u.searchParams.get("cursor") === "c2")).toBe(true);

  const before = calls.tests.length;
  await page.getByTestId("tests-search-input").fill("zzz");
  await expect.poll(() => calls.tests.slice(before).some((u) => u.searchParams.get("q") === "zzz")).toBe(true);
  expect(calls.tests.slice(before).every((u) => u.searchParams.get("cursor") === null)).toBe(true);
  await expect(page.getByText(/belum ada tes/i)).toBeVisible();
});

test("detail: unsaved draft prompts before leaving, then saves material and instructions in one PATCH", async ({ page, context }) => {
  const calls = await setup(page, context);
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/admin/exam/tests/t1");

  await page.getByRole("tab", { name: /informasi tes/i }).click();
  await page.getByLabel(/judul/i).fill("Judul baru");

  const messages: string[] = [];
  page.once("dialog", (d) => {
    messages.push(d.message());
    void d.dismiss();
  });
  await page.getByTestId("tests-back-link").click();
  await expect.poll(() => messages.length).toBe(1);
  expect(messages[0]).toMatch(/buang perubahan/i);
  await expect(page).toHaveURL(/\/admin\/exam\/tests\/t1$/);

  await page.getByRole("button", { name: /simpan pengaturan/i }).click();
  await expect.poll(() => calls.mutations.includes("PATCH /admin/tests/t1")).toBe(true);
});
