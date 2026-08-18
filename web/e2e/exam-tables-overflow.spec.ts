import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

// FR-13: jsdom has no layout, so vitest can't see whether a wide table scrolls
// inside its own wrapper or blows out the page. Fake session per e2e/question-editor.spec.ts.

const FAKE_ADMIN_USER = {
  id: "e2e-fake-admin-id",
  email: "e2e-fake-admin@example.test",
  name: "E2E Fake Admin",
  role: "admin_exam",
};

const VIEWPORTS = [
  { width: 390, height: 844 },
  { width: 1280, height: 800 },
];

function longName(prefix: string, i: number): string {
  return `${prefix} Sangat Panjang Sekali Nomor Urut ${i} Kabupaten Provinsi`;
}

const MONITOR_ROWS = Array.from({ length: 20 }, (_, i) => ({
  registration_id: `reg-${i}`,
  student_id: `stu-${i}`,
  student_name: longName("Siswa", i),
  school_name: longName("SMA Negeri Sangat Panjang", i),
  status: "in_progress",
  answers_saved: 10,
  total_questions: 40,
  checked_in_at: "2026-08-01T01:00:00Z",
  last_saved_at: "2026-08-01T01:30:00Z",
  violation_count: 0,
  session_id: `sess-${i}`,
  admin_submitted: false,
  extended_until: null,
  active_section_title: "Tes Potensi Skolastik Bagian Yang Sangat Panjang Sekali",
  active_section_remaining_seconds: 1200,
}));

const QUESTION_ROWS = Array.from({ length: 20 }, (_, i) => ({
  question: {
    id: `q-${i}`,
    format: "mcq",
    body: `Pertanyaan sangat panjang nomor ${i} yang membahas topik matematika lanjutan dengan banyak detail supaya sel tabel ini melebar sekali`,
    difficulty: "medium",
    point_correct: 1,
    point_wrong: 0,
    sort_order: i,
    topic_id: "topic-1",
    topic: "Topik Sangat Panjang Sekali Kategori Matematika Lanjutan",
    question_number: 1000 + i,
  },
  options: [],
  attached_count: i,
}));

async function mockCommonBackend(page: Page) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" })
  );
}

async function mockMonitorBackend(page: Page) {
  await mockCommonBackend(page);
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

async function mockQuestionsBackend(page: Page) {
  await mockCommonBackend(page);
  await page.route("**/api/v1/admin/topics*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: [
          {
            id: "topic-1",
            name: "Topik Sangat Panjang Sekali Kategori Matematika Lanjutan",
            subject: "math",
          },
        ],
      }),
    });
  });
  await page.route("**/api/v1/admin/questions*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: QUESTION_ROWS, next_cursor: "" }),
    });
  });
}

async function pageDoesNotScrollSideways(page: Page): Promise<boolean> {
  return page.evaluate(
    () => document.documentElement.scrollWidth <= document.documentElement.clientWidth
  );
}

// A collapsed 0px column satisfies both scroll assertions trivially, so the
// visible width is asserted first — that is what caught a real regression.
async function scrollerOverflows(page: Page, testId: string): Promise<boolean> {
  const scroller = page.locator(`[data-testid="${testId}"] .overflow-x-auto`);
  const { scrollWidth, clientWidth } = await scroller.evaluate((el) => ({
    scrollWidth: el.scrollWidth,
    clientWidth: el.clientWidth,
  }));
  expect(clientWidth).toBeGreaterThan(200);
  return scrollWidth > clientWidth;
}

for (const viewport of VIEWPORTS) {
  test(`exam/monitor table overflows inside its wrapper, not the page — ${viewport.width}x${viewport.height}`, async ({
    page,
    context,
  }) => {
    await seedSession(context, {
      token: "e2e-fake-token",
      refreshToken: "e2e-fake-refresh",
      user: FAKE_ADMIN_USER,
    });
    await mockMonitorBackend(page);
    await page.setViewportSize(viewport);
    await page.goto("/admin/exam/monitor");
    await expect(page.getByText(MONITOR_ROWS[0].student_name)).toBeVisible();

    expect(await pageDoesNotScrollSideways(page)).toBe(true);

    if (viewport.width === 390) {
      expect(await scrollerOverflows(page, "exam-monitor-table")).toBe(true);
    }
  });

  test(`exam/questions table overflows inside its wrapper, not the page — ${viewport.width}x${viewport.height}`, async ({
    page,
    context,
  }) => {
    await seedSession(context, {
      token: "e2e-fake-token",
      refreshToken: "e2e-fake-refresh",
      user: FAKE_ADMIN_USER,
    });
    await mockQuestionsBackend(page);
    await page.setViewportSize(viewport);
    await page.goto("/admin/exam/questions");
    await expect(page.getByText(String(QUESTION_ROWS[0].question.question_number))).toBeVisible();

    expect(await pageDoesNotScrollSideways(page)).toBe(true);

    if (viewport.width === 390) {
      expect(await scrollerOverflows(page, "exam-questions-table")).toBe(true);
    }
  });
}

