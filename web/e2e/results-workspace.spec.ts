import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

const SUPER_ADMIN = {
  id: "e2e-super-admin-id",
  email: "e2e-super-admin@example.test",
  name: "E2E Super Admin",
  role: "super_admin",
};

const ADMIN_SCHOOL = {
  id: "e2e-school-admin-id",
  email: "e2e-school-admin@example.test",
  name: "E2E School Admin",
  role: "admin_school",
  school_id: "school-1",
};

const EXAM = {
  id: "exam-1",
  title: "Results Workspace E2E",
  scheduled_at: "2026-08-01T08:00:00Z",
  timer_mode: "overall",
  duration_minutes: 90,
  is_free: false,
  requires_checkin: true,
  allow_leaderboard: true,
  randomize: false,
  status: "published",
  certificate_enabled: false,
  tests: [],
};

const ASSESSMENT_PAGE_1 = {
  summary: {
    total_registered: 2,
    completed_participants: 1,
    completion_rate: 0.5,
    average_score: 87.5,
    max_possible_score: 100,
    distribution: [
      { label: "0-20", count: 0 },
      { label: "21-40", count: 0 },
      { label: "41-60", count: 0 },
      { label: "61-80", count: 0 },
      { label: "81-100", count: 1 },
    ],
    violation_attempts: 1,
    violation_events: 2,
  },
  data: [
    {
      registration_id: "reg-1",
      student_id: "student-1",
      student_name: "Budi Results",
      username: "budi.results",
      school_id: "school-1",
      school_name: "SMA E2E",
      rank: 1,
      score: 87.5,
      attempts_count: 2,
      latest_session_id: "sess-1",
      latest_attempt_number: 2,
      latest_submitted_at: "2026-08-01T10:00:00Z",
      latest_violations: 2,
    },
  ],
  next_cursor: "cursor-page-2",
};

const ASSESSMENT_PAGE_2 = {
  ...ASSESSMENT_PAGE_1,
  data: [
    {
      registration_id: "reg-2",
      student_id: "student-2",
      student_name: "Siti Juara",
      username: "siti.start",
      school_id: "school-1",
      school_name: "SMA E2E",
      rank: 2,
      score: 76,
      attempts_count: 1,
      latest_session_id: "sess-2",
      latest_attempt_number: 1,
      latest_submitted_at: "2026-08-01T10:05:00Z",
      latest_violations: 0,
    },
  ],
  next_cursor: "",
};

async function mockBackend(page: Page) {
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    const method = route.request().method();

    if (method === "GET" && path === "/api/v1/admin/exams/exam-1") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(EXAM) });
    }
    if (method === "GET" && path === "/api/v1/schools") {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([{ id: "school-1", name: "SMA E2E" }]),
      });
    }
    if (method === "GET" && path === "/api/v1/admin/exams/exam-1/results-workspace") {
      const body = url.searchParams.get("cursor") ? ASSESSMENT_PAGE_2 : ASSESSMENT_PAGE_1;
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
    }
    if (method === "GET" && path === "/api/v1/admin/exams/exam-1/results-workspace/reg-1/attempts") {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: [
            {
              session_id: "sess-1",
              attempt_number: 2,
              status: "submitted",
              submitted_at: "2026-08-01T10:00:00Z",
              score: 87.5,
              violations: 2,
              result_available: true,
              is_latest: true,
            },
            {
              session_id: "sess-older",
              attempt_number: 1,
              status: "submitted",
              submitted_at: "2026-08-01T09:00:00Z",
              score: 75,
              violations: 0,
              result_available: false,
              is_latest: false,
            },
          ],
        }),
      });
    }
    if (method === "GET" && path === "/api/v1/admin/exams/exam-1/results-workspace/sessions/sess-1") {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          session_id: "sess-1",
          student_name: "Budi Results",
          score: 87.5,
          correct_count: 8,
          wrong_count: 1,
          empty_count: 1,
          breakdown: [{ test_id: "test-1", title: "Matematika", earned: 87.5, max: 100 }],
          pembahasan: [
            {
              question_id: "question-1",
              body: "2 + 2 = ?",
              format: "mcq",
              your_answer: "B",
              correct_answer: "B",
              is_correct: true,
              explanation: "Empat adalah jawaban yang benar.",
            },
          ],
        }),
      });
    }
    if (method === "GET" && path === "/api/v1/admin/results/export") {
      return route.fulfill({ status: 200, contentType: "text/csv", body: "student,score\nBudi,87.5\n" });
    }
    if (method === "GET" && path === "/api/v1/admin/exams/exam-1/registrations") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: [] }) });
    }

    return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
  });
}

test("super_admin unified Results workspace is ranked and supports large detail modal", async ({ page, context }) => {
  const resultsWorkspaceRequests: string[] = [];
  page.on("request", (req) => {
    if (req.url().includes("/results-workspace")) resultsWorkspaceRequests.push(req.url());
  });
  await seedSession(context, {
    token: "e2e-fake-token",
    refreshToken: "e2e-fake-refresh",
    user: SUPER_ADMIN,
  });
  await mockBackend(page);

  await page.goto("/admin/exam/packages/exam-1");
  await expect(page.getByRole("heading", { name: EXAM.title })).toBeVisible();
  await expect(page.getByRole("button", { name: "Hasil" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Leaderboard" })).toHaveCount(0);

  await page.getByRole("button", { name: "Hasil" }).click();
  await expect(page.getByText("Budi Results")).toBeVisible();
  await expect(page.getByText("50%")).toBeVisible();
  await expect(page.getByText("87.5%").first()).toBeVisible();
  await expect(page.getByText("87.5 / 100").first()).toBeVisible();
  await expect(page.getByText("Distribusi Skor")).toBeVisible();
  await expect(page.getByText("81-100%")).toBeVisible();

  await page.getByPlaceholder("Nama atau username siswa").fill("budi");
  await expect
    .poll(() => resultsWorkspaceRequests.some((url) => url.includes("q=budi")))
    .toBe(true);

  await page.getByRole("button", { name: "Lihat" }).first().click();
  await expect(page.getByRole("heading", { name: "Detail Hasil" })).toBeVisible();
  await expect(page.getByText("Percobaan 2")).toBeVisible();
  await expect(page.getByText("Matematika")).toBeVisible();
  await expect(page.getByText("2 + 2 = ?")).toHaveCount(0);
  await page.getByRole("button", { name: /Matematika/ }).click();
  await expect(page.getByText("2 + 2 = ?")).toBeVisible();
  await expect(page.getByText("Empat adalah jawaban yang benar.")).toBeVisible();
});

test("admin_school keeps scoped Results tab and never calls results workspace endpoint", async ({ page, context }) => {
  const resultsWorkspaceRequests: string[] = [];
  page.on("request", (req) => {
    if (req.url().includes("/results-workspace")) resultsWorkspaceRequests.push(req.url());
  });
  await seedSession(context, {
    token: "e2e-fake-token",
    refreshToken: "e2e-fake-refresh",
    user: ADMIN_SCHOOL,
  });
  await mockBackend(page);

  await page.goto("/admin/exam/packages/exam-1");
  await expect(page.getByRole("heading", { name: EXAM.title })).toBeVisible();
  await expect(page.getByRole("button", { name: "Hasil" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Asesmen" })).toHaveCount(0);
  await expect.poll(() => resultsWorkspaceRequests.length).toBe(0);
});
