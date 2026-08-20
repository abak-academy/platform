import { test, expect, type Page, type BrowserContext } from "@playwright/test";
import { seedSession } from "./helpers/session";

// FR-29/30: jsdom can't compute styles, so the M3-token → ink/brand-token
// swap needs a real browser to prove the resulting colour and contrast.

const FAKE_ADMIN_USER = {
  id: "e2e-fake-admin-id",
  email: "e2e-fake-admin@example.test",
  name: "E2E Fake Admin",
  role: "admin_exam",
};

const MONITOR_ROWS = [
  {
    registration_id: "reg-0",
    student_id: "stu-0",
    student_name: "Siswa Palet",
    school_name: "SMA Negeri Palet",
    status: "in_progress",
    answers_saved: 10,
    total_questions: 40,
    checked_in_at: "2026-08-01T01:00:00Z",
    last_saved_at: "2026-08-01T01:30:00Z",
    violation_count: 0,
    session_id: "sess-0",
    admin_submitted: false,
    extended_until: null,
  },
];

async function mockMonitorBackend(page: Page) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" })
  );
  await page.route("**/api/v1/admin/exams*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: [
          {
            id: "exam-1",
            title: "Ujian E2E",
            scheduled_at: "2026-08-01T00:00:00Z",
            has_published_product: true,
            is_free: true,
            requires_checkin: true,
            allow_leaderboard: true,
            randomize: false,
            timer_mode: "overall",
            duration_minutes: 120,
            grace_window_minutes: 5,
            status: "active",
          },
        ],
      }),
    });
  });
  await page.route("**/api/v1/admin/sessions/monitor*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        exam: {
          id: "exam-1",
          title: "Ujian E2E",
          scheduled_at: "2026-08-01T00:00:00Z",
          duration_minutes: 120,
          grace_window_minutes: 5,
          status: "published",
        },
        rows: MONITOR_ROWS,
        violations_recent: [],
      }),
    });
  });
}

// Mirrors the zustand persist shape stores/ui.ts writes under "abak-ui".
async function seedTheme(context: BrowserContext, theme: "light" | "dark") {
  await context.addInitScript((t) => {
    window.localStorage.setItem(
      "abak-ui",
      JSON.stringify({ state: { theme: t, lang: "id" }, version: 0 })
    );
  }, theme);
}

function relLuminance(r: number, g: number, b: number): number {
  const chan = (c: number) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * chan(r) + 0.7152 * chan(g) + 0.0722 * chan(b);
}

function contrastRatio(fg: [number, number, number], bg: [number, number, number]): number {
  const l1 = relLuminance(...fg);
  const l2 = relLuminance(...bg);
  const [lighter, darker] = l1 > l2 ? [l1, l2] : [l2, l1];
  return (lighter + 0.05) / (darker + 0.05);
}

function parseRgb(rgb: string): [number, number, number] {
  const m = rgb.match(/(\d+),\s*(\d+),\s*(\d+)/);
  if (!m) throw new Error(`unparseable rgb: ${rgb}`);
  return [Number(m[1]), Number(m[2]), Number(m[3])];
}

async function readCellAndCardColors(page: Page): Promise<{ cell: string; card: string }> {
  const cell = page.locator('[data-testid="exam-monitor-table"] td', { hasText: "SMA Negeri Palet" }).locator("span").first();
  const card = page.locator('[data-testid="exam-monitor-table"]');
  const cellColor = await cell.evaluate((el) => getComputedStyle(el).color);
  const cardColor = await card.evaluate((el) => getComputedStyle(el).backgroundColor);
  return { cell: cellColor, card: cardColor };
}

test("monitor secondary text resolves to ink-600 in light mode with AA contrast", async ({ page, context }) => {
  await seedSession(context, {
    token: "e2e-fake-token",
    refreshToken: "e2e-fake-refresh",
    user: FAKE_ADMIN_USER,
  });
  await seedTheme(context, "light");
  await mockMonitorBackend(page);
  await page.goto("/admin/exam/monitor");
  await expect(page.getByText("Siswa Palet")).toBeVisible();

  expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).not.toBe("dark");

  const { cell, card } = await readCellAndCardColors(page);
  expect(cell).toBe("rgb(86, 92, 132)");

  expect(contrastRatio(parseRgb(cell), parseRgb(card))).toBeGreaterThanOrEqual(4.5);
});

test("monitor secondary text resolves to ink-600 in dark mode with AA contrast", async ({ page, context }) => {
  await seedSession(context, {
    token: "e2e-fake-token",
    refreshToken: "e2e-fake-refresh",
    user: FAKE_ADMIN_USER,
  });
  await seedTheme(context, "dark");
  await mockMonitorBackend(page);
  await page.goto("/admin/exam/monitor");
  await expect(page.getByText("Siswa Palet")).toBeVisible();

  // Assert the seeding actually took before trusting any colour read below —
  // otherwise this half silently re-runs the light case and proves nothing.
  expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("dark");

  const { cell, card } = await readCellAndCardColors(page);
  expect(cell).toBe("rgb(186, 192, 220)");

  expect(contrastRatio(parseRgb(cell), parseRgb(card))).toBeGreaterThanOrEqual(4.5);
});

test("light and dark ink-600 resolve to different computed colours", async ({ browser }) => {
  const lightContext = await browser.newContext();
  const darkContext = await browser.newContext();
  try {
    await seedSession(lightContext, { token: "t", refreshToken: "r", user: FAKE_ADMIN_USER });
    await seedTheme(lightContext, "light");
    const lightPage = await lightContext.newPage();
    await mockMonitorBackend(lightPage);
    await lightPage.goto("/admin/exam/monitor");
    await expect(lightPage.getByText("Siswa Palet")).toBeVisible();
    const { cell: lightCell } = await readCellAndCardColors(lightPage);

    await seedSession(darkContext, { token: "t", refreshToken: "r", user: FAKE_ADMIN_USER });
    await seedTheme(darkContext, "dark");
    const darkPage = await darkContext.newPage();
    await mockMonitorBackend(darkPage);
    await darkPage.goto("/admin/exam/monitor");
    await expect(darkPage.getByText("Siswa Palet")).toBeVisible();
    expect(await darkPage.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("dark");
    const { cell: darkCell } = await readCellAndCardColors(darkPage);

    expect(lightCell).not.toBe(darkCell);
  } finally {
    await lightContext.close();
    await darkContext.close();
  }
});
