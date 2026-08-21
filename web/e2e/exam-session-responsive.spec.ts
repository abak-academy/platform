import {
  test,
  expect,
  type BrowserContext,
  type Locator,
  type Page,
} from "@playwright/test";
import type { SessionQuestion, SessionState, SessionTest } from "../lib/types";
import { seedSession } from "./helpers/session";

test.use({ baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3100" });

const STUDENT = {
  id: "e2e-responsive-student",
  email: "responsive@example.test",
  name: "Responsive E2E Student",
  role: "student",
};
const IMAGE_DATA_URI =
  "data:image/svg+xml,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20width='1200'%20height='600'%20viewBox='0%200%201200%20600'%3E%3Crect%20width='1200'%20height='600'%20fill='%23dbeafe'/%3E%3Ctext%20x='40'%20y='80'%20font-size='48'%20fill='%231e3a8a'%3EResponsive%20fixture%201200x600%3C/text%3E%3C/svg%3E";

function imageHeavyBody(index: number) {
  if (index !== 1) return `<p>Pertanyaan gambar ${index}</p>`;
  return `<p>Pertanyaan gambar ${index}</p><img src="${IMAGE_DATA_URI}" alt="fixture 1200px" />`;
}

function mcqQuestion(
  testId: string,
  index: number,
  body = imageHeavyBody(index),
): SessionQuestion {
  return {
    id: `${testId}-q-${index}`,
    test_id: testId,
    format: "mcq",
    body,
    sort_order: index,
    options: [
      { key: "A", text: "Jawaban A", sort_order: 1 },
      { key: "B", text: "Jawaban B", sort_order: 2 },
    ],
  };
}

const imageQuestions = Array.from({ length: 40 }, (_, index) =>
  mcqQuestion("image-test", index + 1),
);

const imageSession: SessionState = {
  session_id: "responsive-image-session",
  registration_id: "responsive-image-registration",
  status: "in_progress",
  started_at: "2026-08-21T00:00:00Z",
  remaining_seconds: 3600,
  timer_mode: "overall",
  duration_minutes: 60,
  answers: [],
  current_position: 0,
  tests: [
    {
      id: "image-test",
      title: "Ujian Gambar",
      subject: "Matematika",
      questions: imageQuestions,
    },
  ],
};

const longOption = "Pilihan panjang ".repeat(25);
const unbrokenOption = "X".repeat(90);
const longTextSession: SessionState = {
  ...imageSession,
  session_id: "responsive-long-text-session",
  tests: [
    {
      id: "long-text-test",
      title: "Ujian Teks Panjang",
      subject: "Bahasa",
      questions: [
        {
          id: "long-text-q-1",
          test_id: "long-text-test",
          format: "mcq",
          body: "Pilih opsi dengan teks sangat panjang.",
          sort_order: 1,
          options: [
            { key: "A", text: longOption, sort_order: 1 },
            { key: "B", text: unbrokenOption, sort_order: 2 },
          ],
        },
      ],
    },
  ],
};

const sectionTests: SessionTest[] = Array.from({ length: 6 }, (_, index) => {
  const number = index + 1;
  return {
    id: `section-${number}`,
    title: `Bagian UTBK ${number} dengan judul yang sengaja panjang`,
    subject: "TPS",
    status: number === 1 ? "active" : "pending",
    remaining_seconds: number === 1 ? 1800 : 0,
    duration_minutes: 30,
    audio_url:
      number === 1
        ? "data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA="
        : null,
    questions: [
      mcqQuestion(`section-${number}`, 1, `<p>Pertanyaan bagian ${number}</p>`),
    ],
  };
});

const sectionedSession: SessionState = {
  ...imageSession,
  session_id: "responsive-sectioned-session",
  mode: "utbk",
  timer_mode: "per_test",
  active_test_id: "section-1",
  remaining_seconds: 1800,
  duration_minutes: null,
  tests: sectionTests,
};

async function mockSession(page: Page, session: SessionState) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" }),
  );
  await page.route("**/api/v1/exam/sessions/*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(session),
    });
  });
}

async function openExam(
  page: Page,
  context: BrowserContext,
  session: SessionState,
  viewport: { width: number; height: number },
) {
  await seedSession(context, {
    token: "e2e-fake",
    refreshToken: "",
    user: STUDENT,
  });
  await context.addInitScript(() => {
    Object.defineProperty(Element.prototype, "requestFullscreen", {
      configurable: true,
      value: () =>
        Promise.reject(
          new Error(
            "fullscreen is intentionally unavailable in the layout harness",
          ),
        ),
    });
    Object.defineProperty(document, "exitFullscreen", {
      configurable: true,
      value: () =>
        Promise.reject(
          new Error(
            "fullscreen exit is intentionally unavailable in the layout harness",
          ),
        ),
    });
  });
  await mockSession(page, session);
  await page.setViewportSize(viewport);
  await page.goto(`/exam/sessions/${session.session_id}`);
  await page.getByTestId("enter-fullscreen").click();
  await expect(
    page.getByTestId("exam-overlay"),
    "exam shell must render after the fullscreen gate",
  ).toBeVisible();
}

async function assertVisibleBox(locator: Locator, label: string) {
  await expect(
    locator,
    `${label} must be visible before geometry is asserted`,
  ).toBeVisible();
  const box = await locator.boundingBox();
  expect(
    box,
    `${label} must have a layout box before geometry is asserted`,
  ).not.toBeNull();
  return box!;
}

async function assertVisibleAndWide(locator: Locator, label: string) {
  const box = await assertVisibleBox(locator, label);
  expect(
    box!.width,
    `${label} must be wider than 100px so a collapsed element cannot pass vacuously`,
  ).toBeGreaterThan(100);
  return box;
}

async function assertNoPageOverflow(page: Page, guard: Locator, label: string) {
  await assertVisibleAndWide(guard, label);
  const metrics = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    innerWidth: window.innerWidth,
    examBodyScrollWidth:
      document.querySelector('[data-testid="exam-body"]')?.scrollWidth ?? 0,
    examBodyClientWidth:
      document.querySelector('[data-testid="exam-body"]')?.clientWidth ?? 0,
  }));
  expect(
    metrics.scrollWidth,
    `${label} must not create horizontal page overflow`,
  ).toBeLessThanOrEqual(metrics.innerWidth + 1);
  expect(
    metrics.examBodyScrollWidth,
    `${label} must not create horizontal overflow inside the exam body scroll container`,
  ).toBeLessThanOrEqual(metrics.examBodyClientWidth + 1);
}

async function assertNoIntersection(
  first: Locator,
  second: Locator,
  label: string,
) {
  const a = await assertVisibleAndWide(first, `${label} first element`);
  const b = await assertVisibleAndWide(second, `${label} second element`);
  const overlap =
    Math.max(0, Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x)) *
    Math.max(0, Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y));
  expect(
    overlap,
    `${label} must not overlap; a panel that covers the question card is a regression`,
  ).toBeLessThanOrEqual(1);
}

async function assertWithinViewport(
  locator: Locator,
  viewport: { width: number; height: number },
  label: string,
) {
  const box = await assertVisibleBox(locator, label);
  expect(
    box.x,
    `${label} must stay inside the viewport left edge`,
  ).toBeGreaterThanOrEqual(-1);
  expect(
    box.y,
    `${label} must stay inside the viewport top edge`,
  ).toBeGreaterThanOrEqual(-1);
  expect(
    box.x + box.width,
    `${label} must stay inside the viewport right edge`,
  ).toBeLessThanOrEqual(viewport.width + 1);
  expect(
    box.y + box.height,
    `${label} must stay inside the viewport bottom edge`,
  ).toBeLessThanOrEqual(viewport.height + 1);
}

async function capture(page: Page, name: string) {
  await page.screenshot({
    path: test.info().outputPath(`${name}.png`),
    fullPage: true,
  });
}

test("E-1 mobile image-heavy exam has no horizontal overflow or overlaying nav", async ({
  page,
  context,
}) => {
  const viewport = { width: 390, height: 844 };
  await openExam(page, context, imageSession, viewport);
  const overlay = page.getByTestId("exam-overlay");
  const card = page.locator('[data-slot="card"]').first();
  const rail = page.getByTestId("exam-nav-rail");
  const topBar = page.getByTestId("exam-top-bar");
  const image = page.locator("[data-rich-content] img");

  await assertNoPageOverflow(page, overlay, "E-1 exam shell");
  await assertNoIntersection(
    card,
    rail,
    "E-1 collapsed mobile nav and question card",
  );
  await assertVisibleAndWide(topBar, "E-1 top bar");
  const topBarMetrics = await topBar.evaluate((el) => ({
    scrollWidth: el.scrollWidth,
    clientWidth: el.clientWidth,
  }));
  expect(
    topBarMetrics.scrollWidth,
    "E-1 top bar must wrap instead of overflowing",
  ).toBeLessThanOrEqual(topBarMetrics.clientWidth + 1);
  const imageBox = await assertVisibleAndWide(
    image,
    "E-1 authored 1200px image",
  );
  const imageMetrics = await image.evaluate((el) => ({
    naturalWidth: (el as HTMLImageElement).naturalWidth,
    parentWidth: el.parentElement!.getBoundingClientRect().width,
  }));
  expect(
    imageMetrics.naturalWidth,
    "E-1 fixture must remain intrinsically 1200px wide",
  ).toBeGreaterThanOrEqual(1200);
  const imageMaxWidth = await image.evaluate(
    (el) => getComputedStyle(el).maxWidth,
  );
  expect(
    imageMaxWidth,
    "E-1 authored images must retain the rich-content 100% max-width clamp",
  ).toBe("100%");
  expect(
    imageBox.width,
    "E-1 authored image must clamp to its rich-content container",
  ).toBeLessThanOrEqual(imageMetrics.parentWidth + 1);
  await capture(page, "e1-mobile-390x844");
});

test("E-2 mobile timer and Submit stay visible and Submit remains clickable", async ({
  page,
  context,
}) => {
  const viewport = { width: 390, height: 844 };
  await openExam(page, context, imageSession, viewport);
  const timer = page.getByTestId("exam-top-bar").getByText(/^\d{2}:\d{2}$/);
  const submit = page.getByTestId("exam-top-bar").getByRole("button");
  await assertWithinViewport(timer, viewport, "E-2 timer");
  await assertWithinViewport(submit, viewport, "E-2 Submit");
  const submitClickable = await submit.evaluate((button) => {
    const rect = button.getBoundingClientRect();
    const target = document.elementFromPoint(
      rect.left + rect.width / 2,
      rect.top + rect.height / 2,
    );
    return target !== null && button.contains(target);
  });
  expect(
    submitClickable,
    "E-2 Submit centre must resolve to the Submit button or its descendant",
  ).toBeTruthy();
  await capture(page, "e2-mobile-390x844");
});

test("E-3 mobile nav expands in flow and selecting a number collapses it", async ({
  page,
  context,
}) => {
  await openExam(page, context, imageSession, { width: 390, height: 844 });
  const toggle = page.getByTestId("exam-nav-toggle");
  const panel = page.locator("#exam-nav-panel");
  const card = page.locator('[data-slot="card"]').first();
  await expect(
    toggle,
    "E-3 navigation must start collapsed on mobile",
  ).toHaveAttribute("aria-expanded", "false");
  await expect(
    panel,
    "E-3 mounted panel must be CSS-hidden before expansion",
  ).toBeHidden();
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await expect(
    panel,
    "E-3 panel must become visible when expanded",
  ).toBeVisible();
  await assertNoIntersection(
    card,
    panel,
    "E-3 expanded nav panel and question card",
  );
  await expect(
    panel.getByText("Terjawab", { exact: true }),
    "E-3 expanded panel must retain the full legend",
  ).toBeVisible();
  await page.getByTestId("session-nav-11").click();
  await expect(
    toggle,
    "E-3 selecting question 12 must collapse the panel",
  ).toHaveAttribute("aria-expanded", "false");
  await expect(
    page.getByTestId("save-indicator"),
    "E-3 navigation must put the save indicator into the unsaved state before autosave flushes",
  ).toHaveText("Belum tersimpan");
  await expect(page.getByTestId("exam-title")).toHaveText("Ujian Gambar");
  const titleMetrics = await page.getByTestId("exam-title").evaluate((el) => ({
    clientWidth: el.clientWidth,
    scrollWidth: el.scrollWidth,
  }));
  expect(
    titleMetrics.scrollWidth,
    "E-3 mobile title must not truncate to a tiny fragment when the unsaved label is visible",
  ).toBeLessThanOrEqual(titleMetrics.clientWidth + 1);
  await expect(
    page.getByText("Pertanyaan gambar 12", { exact: true }),
    "E-3 selecting question 12 must show its question content",
  ).toBeVisible();
  const questionBox = await assertVisibleBox(
    page.getByText("Pertanyaan gambar 12", { exact: true }),
    "E-3 selected question 12 stem",
  );
  expect(
    questionBox.y,
    "E-3 selecting a nav cell must scroll to the selected question, not strand the viewport on the collapsed nav",
  ).toBeGreaterThanOrEqual(0);
  expect(
    questionBox.y,
    "E-3 selected question stem must land inside the viewport",
  ).toBeLessThanOrEqual(844);
  await capture(page, "e3-mobile-390x844");
});

test("E-4 mobile long option text wraps and nav cells remain 40px", async ({
  page,
  context,
}) => {
  await openExam(page, context, longTextSession, { width: 390, height: 844 });
  const optionRow = page.locator('[data-slot="card"] label').nth(1);
  await assertVisibleAndWide(optionRow, "E-4 long option row");
  const optionMetrics = await optionRow.evaluate((el) => ({
    scrollWidth: el.scrollWidth,
    clientWidth: el.clientWidth,
  }));
  expect(
    optionMetrics.scrollWidth,
    "E-4 a 90-character unbroken option token must wrap inside its row",
  ).toBeLessThanOrEqual(optionMetrics.clientWidth + 1);
  await page.getByTestId("exam-nav-toggle").click();
  const navCell = page.getByTestId("session-nav-0");
  const navBox = await assertVisibleBox(
    navCell,
    "E-4 expanded mobile nav cell",
  );
  expect(
    navBox.height,
    "E-4 mobile nav cell must be at least 40px tall",
  ).toBeGreaterThanOrEqual(40);
  expect(
    navBox.width,
    "E-4 mobile nav cell must be at least 40px wide",
  ).toBeGreaterThanOrEqual(40);
  await capture(page, "e4-mobile-390x844");
});

test("E-5 tablet portrait remains stacked without overflow", async ({
  page,
  context,
}) => {
  const viewport = { width: 768, height: 1024 };
  await openExam(page, context, imageSession, viewport);
  const overlay = page.getByTestId("exam-overlay");
  const rail = page.getByTestId("exam-nav-rail");
  const card = page.locator('[data-slot="card"]').first();
  const timer = page.getByTestId("exam-top-bar").getByText(/^\d{2}:\d{2}$/);
  const submit = page.getByTestId("exam-top-bar").getByRole("button");
  const railBox = await assertVisibleAndWide(rail, "E-5 tablet nav rail");
  expect(
    railBox.width,
    "E-5 tablet portrait must stack the nav below the question rather than keep a 280px side rail",
  ).toBeGreaterThan(600);
  await assertNoPageOverflow(page, overlay, "E-5 tablet exam shell");
  await assertNoIntersection(card, rail, "E-5 tablet nav and question card");
  await assertWithinViewport(timer, viewport, "E-5 tablet timer");
  await assertWithinViewport(submit, viewport, "E-5 tablet Submit");
  await capture(page, "e5-tablet-768x1024");
});

test("E-6 tablet section rail scrolls internally and audio fits", async ({
  page,
  context,
}) => {
  await openExam(page, context, sectionedSession, { width: 768, height: 1024 });
  const rail = page.getByTestId("section-rail");
  const audio = page.getByTestId("section-audio-player");
  await assertVisibleAndWide(rail, "E-6 section rail");
  const railMetrics = await rail.evaluate((el) => ({
    scrollWidth: el.scrollWidth,
    clientWidth: el.clientWidth,
  }));
  expect(
    railMetrics.scrollWidth,
    "E-6 five-plus sections must overflow inside the section rail",
  ).toBeGreaterThan(railMetrics.clientWidth);
  await assertNoPageOverflow(
    page,
    page.getByTestId("exam-overlay"),
    "E-6 sectioned tablet shell",
  );
  await assertVisibleAndWide(audio, "E-6 section audio player");
  const audioBox = await audio.boundingBox();
  expect(
    audioBox!.x + audioBox!.width,
    "E-6 section audio player must fit its pane",
  ).toBeLessThanOrEqual(769);
  await capture(page, "e6-sectioned-768x1024");
});

test("E-7 desktop retains a 280px nav rail without page overflow", async ({
  page,
  context,
}) => {
  await openExam(page, context, imageSession, { width: 1280, height: 800 });
  const rail = page.getByTestId("exam-nav-rail");
  const railBox = await assertVisibleAndWide(rail, "E-7 desktop nav rail");
  expect(
    railBox.width,
    "E-7 desktop nav rail must remain 280px wide",
  ).toBeGreaterThanOrEqual(279);
  expect(
    railBox.width,
    "E-7 desktop nav rail must remain 280px wide",
  ).toBeLessThanOrEqual(281);
  await assertNoPageOverflow(
    page,
    page.getByTestId("exam-overlay"),
    "E-7 desktop exam shell",
  );
  await page.getByTestId("exam-question-pane").evaluate((el) => {
    el.scrollTop = el.scrollHeight;
  });
  await page.getByTestId("session-nav-11").click();
  const questionBox = await assertVisibleBox(
    page.getByText("Pertanyaan gambar 12", { exact: true }),
    "E-7 selected question 12 stem",
  );
  expect(
    questionBox.y,
    "E-7 desktop nav selection must reset the question-pane scroll position",
  ).toBeGreaterThanOrEqual(0);
  expect(
    questionBox.y,
    "E-7 desktop selected question stem must land inside the viewport",
  ).toBeLessThanOrEqual(800);
  await capture(page, "e7-desktop-1280x800");
});

test("E-8 1024px tablet landscape uses the desktop rail boundary", async ({
  page,
  context,
}) => {
  await openExam(page, context, imageSession, { width: 1024, height: 768 });
  const rail = page.getByTestId("exam-nav-rail");
  const railBox = await assertVisibleAndWide(rail, "E-8 1024px nav rail");
  expect(
    railBox.width,
    "E-8 1024px must use the desktop 280px rail, not the stacked layout",
  ).toBeGreaterThanOrEqual(279);
  expect(
    railBox.width,
    "E-8 1024px must use the desktop 280px rail, not the stacked layout",
  ).toBeLessThanOrEqual(281);
  await capture(page, "e8-landscape-1024x768");
});

test("E-9 expanded mobile panel never covers the question card at the bottom", async ({
  page,
  context,
}) => {
  await openExam(page, context, imageSession, { width: 390, height: 844 });
  const toggle = page.getByTestId("exam-nav-toggle");
  const panel = page.locator("#exam-nav-panel");
  const card = page.locator('[data-slot="card"]').first();
  await expect(
    toggle,
    "E-9 mobile nav toggle must exist before expanded-panel overlap is measured",
  ).toBeVisible();
  await toggle.click();
  await expect(panel).toBeVisible();
  await page.getByTestId("exam-body").evaluate((el) => {
    el.scrollTop = el.scrollHeight;
  });
  await assertNoIntersection(
    card,
    panel,
    "E-9 bottom-scrolled expanded nav panel and question card",
  );
  await capture(page, "e9-mobile-bottom-390x844");
});

test("E-10 sectioned mobile keeps its timer visible and has no Submit", async ({
  page,
  context,
}) => {
  const viewport = { width: 390, height: 844 };
  await openExam(page, context, sectionedSession, viewport);
  const topBar = page.getByTestId("exam-top-bar");
  const timer = topBar.getByText(/^\d{2}:\d{2}$/);
  await expect(timer, "E-10 sectioned mode must use the active test timer").toHaveText("30:00");
  await assertWithinViewport(timer, viewport, "E-10 sectioned timer");
  await expect(
    topBar.getByRole("button"),
    "E-10 sectioned mode must not render Submit",
  ).toHaveCount(0);
  await capture(page, "e10-sectioned-mobile-390x844");
});
