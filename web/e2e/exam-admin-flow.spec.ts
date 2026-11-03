import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

const VIEWPORTS = [
  { width: 390, height: 844 },
  { width: 1024, height: 800 },
  { width: 1440, height: 900 },
];

const SECTION_ID = "5f1d1c2e-3b4a-4c6d-8e9f-0a1b2c3d4e5f";

const LIST_ROWS = [
  {
    id: "exam-2",
    title: "Tryout TKA Paket A Dengan Judul Yang Cukup Panjang Untuk Menguji Tabel",
    scheduled_at: "2026-11-07T08:00:00Z",
    scheduled_end_at: "2026-11-07T10:00:00Z",
    format_version: 2,
    navigation_policy: "sequential",
    timer_mode: "per_test",
    duration_minutes: null,
    status: "draft",
    registration_count: 12,
    section_count: 2,
    question_count: 35,
    sections_duration_minutes: 70,
  },
  {
    id: "exam-1",
    title: "Tryout Lama",
    scheduled_at: null,
    format_version: 1,
    mode: "standard",
    timer_mode: "overall",
    duration_minutes: 60,
    status: "published",
    registration_count: 28,
    section_count: 0,
    question_count: 0,
    sections_duration_minutes: null,
  },
];

const DETAIL = {
  ...LIST_ROWS[0],
  has_sessions: true,
  requires_checkin: true,
  check_in_window_minutes: 15,
  grace_window_minutes: 5,
  result_config: "score_only",
  result_release_at: "2026-11-08T08:00:00Z",
  max_attempts: 1,
  card_notes: [],
  tests: [
    {
      id: "entry-1",
      exam_id: "exam-2",
      test_id: SECTION_ID,
      sort_order: 0,
      question_presentation: "all_on_page",
      material_question_positions: [2, 4],
      test: { id: SECTION_ID, title: "Literasi Membaca", subject: "Bahasa", duration_minutes: 30, question_count: 20 },
    },
  ],
};

async function login(page: Page, context: Parameters<typeof seedSession>[0]) {
  await seedSession(context, {
    token: "e2e-fake-token",
    refreshToken: "e2e-fake-refresh",
    user: { id: "e2e-admin", email: "e2e-admin@example.test", name: "E2E Admin", role: "admin_exam" },
  });
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/admin/exams")) {
      await route.fulfill({ json: { data: LIST_ROWS } });
    } else if (path.endsWith("/admin/exams/exam-2")) {
      await route.fulfill({ json: DETAIL });
    } else {
      await route.fulfill({ json: { data: [] } });
    }
  });
}

const pageDoesNotScrollSideways = (page: Page) =>
  page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth);

for (const viewport of VIEWPORTS) {
  test(`exam list shows composition without document overflow — ${viewport.width}px`, async ({ page, context }) => {
    await login(page, context);
    await page.setViewportSize(viewport);
    await page.goto("/admin/exam/packages");

    await expect(page.getByRole("columnheader")).toHaveCount(5);
    await expect(page.getByText("2 section · 35 soal")).toBeVisible();
    await expect(page.getByText("Total 70 menit")).toBeVisible();
    await expect(page.getByText("Format sebelumnya")).toBeVisible();
    expect(await pageDoesNotScrollSideways(page)).toBe(true);
  });

  test(`exam detail overview and sections fit without document overflow — ${viewport.width}px`, async ({ page, context }) => {
    await login(page, context);
    await page.setViewportSize(viewport);
    await page.goto("/admin/exam/packages/exam-2");

    await expect(page.getByRole("button", { name: "Pengaturan", exact: true })).toBeVisible();
    await expect(page.getByText("Jendela Check-in")).toBeVisible();
    expect(await pageDoesNotScrollSideways(page)).toBe(true);

    await page.getByRole("button", { name: "Tes & section", exact: true }).click();
    const select = page.getByRole("combobox", { name: "Tampilan soal Literasi Membaca" });
    await expect(select).toBeDisabled();
    await expect(page.getByText(/terkunci karena ujian ini sudah memiliki sesi/i)).toBeVisible();
    await expect(page.getByText(/#1 · Literasi Membaca: soal 2, 4 punya materi sendiri/)).toBeVisible();
    const box = await select.boundingBox();
    expect(box?.width ?? 0).toBeGreaterThanOrEqual(220);
    expect(await pageDoesNotScrollSideways(page)).toBe(true);
  });
}
