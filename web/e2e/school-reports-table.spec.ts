import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

// FR-38/39: jsdom can't see layout overflow or real keyboard focus order.
// Fake session per exam-tables-overflow.spec.ts.

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

function longName(i: number): string {
  return `Siswa Sangat Panjang Sekali Nomor Urut ${i} Kabupaten Provinsi`;
}

const EXAM = {
  id: "exam-1",
  type: "exam",
  name: "Tryout E2E Sangat Panjang",
  price: 0,
  status: "published",
};

const RESULT_ROWS = Array.from({ length: 20 }, (_, i) => ({
  session_id: `sess-${i}`,
  student_name: longName(i),
  username: `user${i}`,
  score: 80 + i,
  submitted_at: "2026-08-01T01:00:00Z",
}));

const RESULT_DETAIL = {
  session_id: RESULT_ROWS[0].session_id,
  student_name: RESULT_ROWS[0].student_name,
  username: RESULT_ROWS[0].username,
  score: RESULT_ROWS[0].score,
  submitted_at: RESULT_ROWS[0].submitted_at,
  result_config: "score_only",
  correct_count: 10,
  wrong_count: 2,
  empty_count: 1,
};

async function mockBackend(page: Page) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" })
  );
  await page.route("**/api/v1/products*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: [EXAM] }),
    });
  });
  await page.route("**/api/v1/admin/results?*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: RESULT_ROWS }),
    });
  });
  await page.route("**/api/v1/admin/results/*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(RESULT_DETAIL),
    });
  });
}

async function pageDoesNotScrollSideways(page: Page): Promise<boolean> {
  return page.evaluate(
    () => document.documentElement.scrollWidth <= document.documentElement.clientWidth
  );
}

// A collapsed 0px column satisfies the overflow comparison vacuously, so the
// wrapper's visible width is asserted first.
async function assertTableIsTheScroller(page: Page, overflows: boolean) {
  const scroller = page.locator('[data-testid="school-reports-table"] .overflow-x-auto');
  await expect(scroller).toHaveCount(1);
  const { scrollWidth, clientWidth } = await scroller.evaluate((el) => ({
    scrollWidth: el.scrollWidth,
    clientWidth: el.clientWidth,
  }));
  expect(clientWidth).toBeGreaterThan(200);
  if (overflows) expect(scrollWidth).toBeGreaterThan(clientWidth);
}

async function selectExam(page: Page) {
  await page.getByRole("combobox").first().click();
  await page.getByRole("option", { name: EXAM.name, exact: true }).click();
}

for (const viewport of VIEWPORTS) {
  test(`school/reports table overflows inside its wrapper, not the page — ${viewport.width}x${viewport.height}`, async ({
    page,
    context,
  }) => {
    await seedSession(context, {
      token: "e2e-fake-token",
      refreshToken: "e2e-fake-refresh",
      user: FAKE_ADMIN_USER,
    });
    await mockBackend(page);
    await page.setViewportSize(viewport);
    await page.goto("/admin/school/reports");
    await selectExam(page);
    await expect(page.getByText(RESULT_ROWS[0].student_name)).toBeVisible();

    expect(await pageDoesNotScrollSideways(page)).toBe(true);
    await assertTableIsTheScroller(page, viewport.width === 390);
  });
}

test("view control opens the drill-down dialog via keyboard, native <button> semantics", async ({
  page,
  context,
}) => {
  await seedSession(context, {
    token: "e2e-fake-token",
    refreshToken: "e2e-fake-refresh",
    user: FAKE_ADMIN_USER,
  });
  await mockBackend(page);
  await page.setViewportSize(VIEWPORTS[1]);
  await page.goto("/admin/school/reports");
  await selectExam(page);
  await expect(page.getByText(RESULT_ROWS[0].student_name)).toBeVisible();

  const viewButton = page
    .locator("tr", { hasText: RESULT_ROWS[0].student_name })
    .getByRole("button");

  // Tab from the search box (the last focusable control before the table)
  // to prove the control is reachable in natural tab order, not just
  // programmatically focusable.
  await page.getByPlaceholder(/cari/i).click();
  await page.keyboard.press("Tab");
  await expect(viewButton).toBeFocused();
  await page.keyboard.press("Enter");

  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByText(RESULT_ROWS[0].student_name, { exact: false })).toBeVisible();
});
