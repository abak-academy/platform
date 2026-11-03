import { test, expect, type BrowserContext, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

const VIEWPORTS = [360, 390, 768, 1024, 1440];

const LONG_WA = `https://chat.whatsapp.com/${"A".repeat(220)}`;
const LONG_NOTE = `${"Catatan".repeat(40)}\n${"Z".repeat(180)}`;
const LONG_RULES_URL = `https://example.com/${"panduan".repeat(40)}`;
const SECTION_ID = "5f1d1c2e-3b4a-4c6d-8e9f-0a1b2c3d4e5f";

const ADMIN_EXAM = {
  id: "exam-2",
  title: "Tryout Event Info",
  scheduled_at: "2026-11-07T08:00:00Z",
  scheduled_end_at: "2026-11-07T10:00:00Z",
  format_version: 2,
  navigation_policy: "sequential",
  timer_mode: "per_test",
  duration_minutes: null,
  status: "draft",
  has_sessions: true,
  max_attempts: 1,
  result_config: "hidden",
  card_notes: [],
  whatsapp_group_url: LONG_WA,
  additional_rules_note: LONG_NOTE,
  additional_rules_url: LONG_RULES_URL,
  tests: [
    {
      id: "entry-1",
      exam_id: "exam-2",
      test_id: SECTION_ID,
      sort_order: 0,
      question_presentation: "one_per_page",
      material_question_positions: [],
      test: { id: SECTION_ID, title: "Literasi Membaca", subject: "Bahasa", duration_minutes: 30, question_count: 20 },
    },
  ],
};

const STUDENT_EXAM = {
  id: "exam-2",
  title: "Tryout Event Info",
  scheduled_at: new Date(Date.now() - 60 * 60_000).toISOString(),
  scheduled_end_at: new Date(Date.now() + 2 * 24 * 60 * 60_000).toISOString(),
  requires_checkin: false,
  check_in_window_minutes: null,
  timer_mode: "per_test",
  navigation_policy: "sequential",
  max_attempts: 1,
  duration_minutes: null,
  result_config: "hidden",
  card_enabled: false,
  card_notes: [],
  section_count: 1,
  question_count: 20,
  sections_duration_minutes: 30,
  sections: [{ test_id: SECTION_ID, title: "Literasi Membaca", duration_minutes: 30, question_count: 20, question_presentation: "one_per_page" }],
  whatsapp_group_url: LONG_WA,
  additional_rules_note: LONG_NOTE,
  additional_rules_url: LONG_RULES_URL,
};

const STUDENT_DETAIL = {
  id: "reg-1",
  student_id: "s-1",
  exam_id: "exam-2",
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
  exam: STUDENT_EXAM,
};

async function prepare(context: BrowserContext, page: Page, role: string) {
  await seedSession(context, { token: "fixture", refreshToken: "", user: { id: "u-1", role, name: "Browser User" } });
  await context.addInitScript(() => {
    window.localStorage.setItem("abak-ui", JSON.stringify({ state: { theme: "light", lang: "id" }, version: 0 }));
  });
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url()).pathname;
    if (url.endsWith("/exam/registrations/reg-1")) return route.fulfill({ json: STUDENT_DETAIL });
    if (url.endsWith("/admin/exams/exam-2")) return route.fulfill({ json: ADMIN_EXAM });
    return route.fulfill({ json: { data: [] } });
  });
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

for (const width of VIEWPORTS) {
  test(`admin settings modal, summary and rules card have no horizontal overflow at ${width}px`, async ({ page, context }) => {
    await prepare(context, page, "admin_exam");
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/admin/exam/packages/exam-2");

    const card = page.getByRole("region", { name: "Grup WhatsApp kompetisi" });
    await expect(card).toBeVisible();
    expect((await card.boundingBox())!.width).toBeGreaterThan(200);
    await expectNoHorizontalOverflow(page);

    await page.getByRole("button", { name: /^Tes & section \(\d+\)$/ }).click();
    const rules = page.getByRole("region", { name: "Aturan pengerjaan" });
    await expect(rules).toBeVisible();
    expect((await rules.boundingBox())!.width).toBeGreaterThan(200);
    await expectNoHorizontalOverflow(page);

    await rules.getByRole("button", { name: "Edit" }).click();
    await expect(rules.getByLabel("Keterangan tambahan")).toBeEnabled();
    await expectNoHorizontalOverflow(page);

    await page.getByRole("button", { name: "Pengaturan", exact: true }).click();
    const field = page.getByLabel(/link grup whatsapp kompetisi/i);
    await expect(field).toHaveValue(LONG_WA);
    const dialog = page.getByRole("dialog");
    const box = (await dialog.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(-1);
    expect(box.x + box.width).toBeLessThanOrEqual(width + 1);
    expect((await field.boundingBox())!.width).toBeGreaterThan(100);
    await expectNoHorizontalOverflow(page);
  });

  test(`student briefing with long invitation and rules has no horizontal overflow at ${width}px`, async ({ page, context }) => {
    await prepare(context, page, "student");
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/exam/reg-1");

    const join = page.getByRole("link", { name: /gabung grup whatsapp/i });
    await expect(join).toBeVisible();
    await expect(join).toHaveAttribute("rel", "noopener noreferrer");
    const rulesLink = page.getByRole("link", { name: /baca aturan lengkap/i });
    await expect(rulesLink).toBeVisible();
    expect((await page.getByText(/^CatatanCatatan/).boundingBox())!.width).toBeGreaterThan(100);
    await expectNoHorizontalOverflow(page);
  });
}
