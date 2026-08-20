import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

// FR-11..17: subject/topic/search toolbar on /admin/exam/tests. Modelled on
// exam-tables-overflow.spec.ts — jsdom can't see layout, so overflow needs a real browser.

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

const TOPICS = [
  { id: "topic-1", name: "Aljabar", subject: "Matematika" },
  { id: "topic-2", name: "Reading", subject: "Bahasa Inggris" },
];

const TESTS = [
  {
    id: "t1",
    title: "Tryout UTBK Saintek",
    subject: "Matematika",
    topic: "Aljabar",
    duration_minutes: 90,
    question_count: 10,
  },
  {
    id: "t2",
    title: "Tryout Bahasa Inggris",
    subject: "Bahasa Inggris",
    topic: "Reading",
    duration_minutes: 60,
    question_count: 8,
  },
];

async function mockCommonBackend(page: Page) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" })
  );
}

async function mockTestsBackend(page: Page) {
  await mockCommonBackend(page);
  await page.route("**/api/v1/admin/topics*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: TOPICS }),
    });
  });
  await page.route("**/api/v1/admin/tests*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: TESTS }),
    });
  });
}

async function pageDoesNotScrollSideways(page: Page): Promise<boolean> {
  return page.evaluate(
    () => document.documentElement.scrollWidth <= document.documentElement.clientWidth
  );
}

for (const viewport of VIEWPORTS) {
  test(`exam/tests toolbar fits the viewport and every control is usable — ${viewport.width}x${viewport.height}`, async ({
    page,
    context,
  }) => {
    await seedSession(context, {
      token: "e2e-fake-token",
      refreshToken: "e2e-fake-refresh",
      user: FAKE_ADMIN_USER,
    });
    await mockTestsBackend(page);
    await page.setViewportSize(viewport);
    await page.goto("/admin/exam/tests");
    await expect(page.getByText(TESTS[0].title)).toBeVisible();

    const toolbar = page.getByTestId("exam-tests-toolbar");
    await expect(toolbar).toBeVisible();

    const subjectSelect = page.getByTestId("tests-subject-filter");
    const topicSelect = page.getByTestId("tests-topic-filter");
    const searchInput = page.getByTestId("tests-search-input");

    // A collapsed 0px control would satisfy the overflow check below
    // vacuously, so its visible width is asserted first.
    for (const control of [subjectSelect, topicSelect, searchInput]) {
      const width = await control.evaluate((el) => el.clientWidth);
      expect(width).toBeGreaterThan(0);
    }

    expect(await pageDoesNotScrollSideways(page)).toBe(true);

    const requestPromise = page.waitForRequest(
      (req) => req.url().includes("/admin/tests") && req.url().includes("q=")
    );
    await searchInput.fill("tryout");
    const request = await requestPromise;
    expect(request.url()).toContain("q=");
  });
}
