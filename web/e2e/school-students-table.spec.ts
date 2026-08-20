import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

// FR-33: jsdom has no layout, so vitest can't see whether the wide students
// table scrolls inside its own wrapper or blows out the page. Modelled on
// e2e/exam-tables-overflow.spec.ts.

const FAKE_ADMIN_USER = {
  id: "e2e-fake-admin-id",
  email: "e2e-fake-admin@example.test",
  name: "E2E Fake Admin",
  role: "admin_school",
  school_id: "s1",
};

const VIEWPORTS = [
  { width: 390, height: 844 },
  { width: 1280, height: 800 },
];

function longName(prefix: string, i: number): string {
  return `${prefix} Sangat Panjang Sekali Nomor Urut ${i} Kabupaten Provinsi`;
}

const STUDENT_ROWS = Array.from({ length: 20 }, (_, i) => ({
  id: `st-${i}`,
  name: longName("Siswa", i),
  username: `siswa${i}`,
  jenjang: "SMA",
  email: `siswa${i}@example.test`,
  status: i % 2 === 0 ? "active" : "deactivated",
  grade: 10,
  school_name: longName("SMA Negeri Sangat Panjang", i),
  created_at: "2026-08-01T01:00:00Z",
}));

async function mockCommonBackend(page: Page) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" })
  );
}

async function mockStudentsBackend(page: Page) {
  await mockCommonBackend(page);
  await page.route("**/api/v1/admin/students*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: STUDENT_ROWS, next_cursor: "" }),
    });
  });
}

async function pageDoesNotScrollSideways(page: Page): Promise<boolean> {
  return page.evaluate(
    () => document.documentElement.scrollWidth <= document.documentElement.clientWidth
  );
}

// The wrapper must exist at every viewport, or a "fix" that deletes it passes
// wherever the table happens to fit. A collapsed 0px column satisfies the
// overflow comparison trivially, so the visible width is asserted first.
async function assertTableIsTheScroller(page: Page, testId: string, overflows: boolean) {
  const scroller = page.locator(`[data-testid="${testId}"] .overflow-x-auto`);
  await expect(scroller).toHaveCount(1);
  const { scrollWidth, clientWidth } = await scroller.evaluate((el) => ({
    scrollWidth: el.scrollWidth,
    clientWidth: el.clientWidth,
  }));
  expect(clientWidth).toBeGreaterThan(200);
  if (overflows) expect(scrollWidth).toBeGreaterThan(clientWidth);
}

for (const viewport of VIEWPORTS) {
  test(`school/students table overflows inside its wrapper, not the page — ${viewport.width}x${viewport.height}`, async ({
    page,
    context,
  }) => {
    await seedSession(context, {
      token: "e2e-fake-token",
      refreshToken: "e2e-fake-refresh",
      user: FAKE_ADMIN_USER,
    });
    await mockStudentsBackend(page);
    await page.setViewportSize(viewport);
    await page.goto("/admin/school/students");
    await expect(page.getByText(STUDENT_ROWS[0].name)).toBeVisible();

    expect(await pageDoesNotScrollSideways(page)).toBe(true);
    await assertTableIsTheScroller(page, "school-students-table", viewport.width === 390);
  });
}
