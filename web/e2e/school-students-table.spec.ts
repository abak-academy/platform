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
  // useSchools() (GET /schools) and useProvinces() (GET /provinces) both
  // expect an array body, not the catch-all's "{}" — {}.find / {}.map isn't a
  // function, which crashed the page before it ever rendered the table.
  await page.route("**/api/v1/schools*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
  });
  await page.route("**/api/v1/provinces*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
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
//
// 200 (borrowed from exam-tables-overflow.spec.ts) is too high here: at
// 390px this page nests one more padded layer than /admin/exam/* does (its
// own `max-w-6xl px-4` wrapper on top of AppShell's collapsed sidebar rail
// and `.md-card-outlined` card padding), which legitimately measures 188px
// wide, not collapsed. 100 still fails a near-0px collapse while clearing
// that legitimate width — same conclusion school-reports-table.spec.ts
// reached for the same /admin/school/* layout.
async function assertTableIsTheScroller(page: Page, testId: string, overflows: boolean) {
  const scroller = page.locator(`[data-testid="${testId}"] .overflow-x-auto`);
  await expect(scroller).toHaveCount(1);
  const { scrollWidth, clientWidth } = await scroller.evaluate((el) => ({
    scrollWidth: el.scrollWidth,
    clientWidth: el.clientWidth,
  }));
  expect(clientWidth).toBeGreaterThan(100);
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
