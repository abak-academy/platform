import { test, expect, type BrowserContext, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";
import type { RegistrationDetail, RegistrationExam, RegistrationListItem } from "../lib/types";

const LONG_TITLE =
  "Simulasi Ujian Seleksi Nasional Berbasis Komputer Tahap Akhir Gelombang Kedua Tahun Ajaran Dua Ribu Dua Puluh Enam Supercalifragilisticexpialidocious";

const OPENS_AT = new Date(Date.now() - 60 * 60_000).toISOString();
const CLOSES_AT = new Date(Date.now() + 2 * 24 * 60 * 60_000).toISOString();

const exam: RegistrationExam = {
  id: "exam-1",
  title: LONG_TITLE,
  scheduled_at: OPENS_AT,
  scheduled_end_at: CLOSES_AT,
  requires_checkin: false,
  check_in_window_minutes: null,
  timer_mode: "per_test",
  navigation_policy: "sequential",
  max_attempts: 3,
  duration_minutes: 120,
  result_config: "{}",
  card_enabled: true,
  card_notes: [],
  section_count: 2,
  question_count: 50,
  sections_duration_minutes: 100,
  sections: [
    { test_id: "t-1", title: "Penalaran Umum dan Kemampuan Memahami Bacaan Yang Sangat Panjang Sekali", duration_minutes: 60, question_count: 30, question_presentation: "one_per_page" },
    { test_id: "t-2", title: "Literasi Bahasa Inggris", duration_minutes: 40, question_count: 20, question_presentation: "all_on_page" },
  ],
};

const listItem: RegistrationListItem = {
  id: "reg-1",
  student_id: "s-1",
  exam_id: "exam-1",
  token: "ABC12345",
  card_key: null,
  checked_in_at: null,
  attempts_used: 0,
  status: "registered",
  created_at: "2026-06-01T00:00:00Z",
  exam_title: LONG_TITLE,
  scheduled_at: exam.scheduled_at,
  scheduled_end_at: exam.scheduled_end_at,
  is_free: false,
  requires_checkin: false,
  check_in_window_minutes: null,
  duration_minutes: 120,
  session_id: null,
  max_attempts: 3,
  navigation_policy: "sequential",
  timer_mode: "per_test",
  section_count: 2,
  question_count: 50,
  sections_duration_minutes: 100,
};

const detail: RegistrationDetail = {
  id: "reg-1",
  student_id: "s-1",
  exam_id: "exam-1",
  token: "ABC12345",
  card_key: null,
  checked_in_at: null,
  attempts_used: 0,
  status: "registered",
  created_at: "2026-06-01T00:00:00Z",
  participant_number: 1,
  participant_no: "260715-000001",
  subject: "TPS",
  platform: "exam.abakacademy.id",
  footer_note: "",
  tenant_name: "Abak Academy",
  contact: { phone: "", email: "", help_url: "", social_handle: "" },
  exam,
};

const VIEWPORTS = [360, 390, 768, 1024, 1440];

async function mockApi(page: Page) {
  await page.route("**/api/v1/**", async (route) => {
    const url = route.request().url();
    if (url.endsWith("/exam/registrations/reg-1")) return route.fulfill({ json: detail });
    if (url.endsWith("/exam/registrations")) return route.fulfill({ json: { data: [listItem] } });
    if (url.endsWith("/admin/exams/exam-1/preview")) return route.fulfill({ json: exam });
    return route.fulfill({ json: { data: [] } });
  });
}

async function prepare(context: BrowserContext, page: Page, role: string, lang: "id" | "en") {
  await seedSession(context, { token: "fixture", refreshToken: "", user: { id: "u-1", role, name: "Browser User" } });
  await context.addInitScript((l) => {
    window.localStorage.setItem("abak-ui", JSON.stringify({ state: { theme: "light", lang: l }, version: 0 }));
  }, lang);
  await mockApi(page);
}

async function expectNoHorizontalOverflow(page: Page) {
  const dims = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, viewport: window.innerWidth }));
  const offenders = await page.evaluate(() =>
    [...document.querySelectorAll("body *")]
      .filter((el) => el.getBoundingClientRect().right > window.innerWidth + 1)
      .slice(0, 5)
      .map((el) => `${el.tagName}:${(el.textContent ?? "").slice(0, 40)}:${Math.round(el.getBoundingClientRect().right)}`),
  );
  expect(dims.scroll, `elements past the viewport: ${offenders.join(" | ")}`).toBeLessThanOrEqual(dims.viewport + 1);
}

for (const lang of ["id", "en"] as const) {
  for (const width of VIEWPORTS) {
    test(`student card and detail have no horizontal overflow at ${width}px (${lang})`, async ({ page, context }) => {
      await prepare(context, page, "student", lang);
      await page.setViewportSize({ width, height: 900 });

      await page.goto("/exam");
      const card = page.getByRole("heading", { name: LONG_TITLE });
      await expect(card).toBeVisible();
      expect((await card.boundingBox())!.width).toBeGreaterThan(100);
      await expect(page.getByRole("link", { name: lang === "id" ? "Lihat aturan ujian" : "View exam rules" })).toBeVisible();
      await expectNoHorizontalOverflow(page);

      await page.goto("/exam/reg-1");
      const title = page.getByRole("heading", { level: 1, name: LONG_TITLE });
      await expect(title).toBeVisible();
      expect((await title.boundingBox())!.width).toBeGreaterThan(100);
      await expect(page.getByRole("checkbox")).toBeVisible();
      await expectNoHorizontalOverflow(page);
    });
  }
}

test("admin preview renders read-only without overflow on a narrow screen", async ({ page, context }) => {
  await prepare(context, page, "admin_exam", "id");
  await page.setViewportSize({ width: 360, height: 900 });

  await page.goto("/admin/exam/packages/exam-1/preview");
  await expect(page.getByRole("heading", { level: 1, name: LONG_TITLE })).toBeVisible();
  await expect(page.getByRole("button", { name: /pratinjau: mulai dinonaktifkan/i })).toBeDisabled();
  await expectNoHorizontalOverflow(page);
});
