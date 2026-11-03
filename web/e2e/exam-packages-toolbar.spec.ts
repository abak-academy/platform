import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

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

const PACKAGE_ROWS = [
  {
    id: "exam-1",
    title: "Ujian E2E Sangat Panjang Sekali Nomor Urut Kabupaten Provinsi",
    scheduled_at: "2026-08-01T00:00:00Z",
    has_published_product: true,
    is_free: true,
    requires_checkin: true,
    allow_leaderboard: true,
    randomize: false,
    format_version: 2,
    navigation_policy: "free",
    timer_mode: "overall",
    duration_minutes: 120,
    grace_window_minutes: 5,
    status: "published",
    registration_count: 0,
    section_count: 2,
    question_count: 40,
    sections_duration_minutes: 90,
  },
];

async function mockCommonBackend(page: Page) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" })
  );
}

async function mockPackagesBackend(page: Page) {
  await mockCommonBackend(page);
  await page.route("**/api/v1/admin/exams*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: PACKAGE_ROWS }),
    });
  });
}

async function pageDoesNotScrollSideways(page: Page): Promise<boolean> {
  return page.evaluate(
    () => document.documentElement.scrollWidth <= document.documentElement.clientWidth
  );
}

for (const viewport of VIEWPORTS) {
  test(`exam/packages toolbar and table fit without page overflow — ${viewport.width}x${viewport.height}`, async ({
    page,
    context,
  }) => {
    await seedSession(context, {
      token: "e2e-fake-token",
      refreshToken: "e2e-fake-refresh",
      user: FAKE_ADMIN_USER,
    });
    await mockPackagesBackend(page);
    await page.setViewportSize(viewport);
    await page.goto("/admin/exam/packages");
    await expect(page.getByText(PACKAGE_ROWS[0].title)).toBeVisible();

    // Two "Cari…" inputs exist: the admin shell's global search is type="search"
    // (role searchbox); the page's own filter is a plain textbox.
    const search = page.getByRole("textbox", { name: "Cari…" });
    await expect(page.locator('select[data-slot="select"]')).toHaveCount(0);
    await expect(search).toBeVisible();
    expect(await search.evaluate((el) => el.clientWidth)).toBeGreaterThan(0);

    expect(await pageDoesNotScrollSideways(page)).toBe(true);

    const requestPromise = page.waitForRequest((req) => req.url().includes("q="));
    await search.fill("ujian");
    const request = await requestPromise;
    expect(request.url()).toContain("q=");

    const row = page.locator('[data-testid="package-row"]').first();
    expect(await row.evaluate((el) => el.clientWidth)).toBeGreaterThan(200);
    expect(await pageDoesNotScrollSideways(page)).toBe(true);
  });
}
