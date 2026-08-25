import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

const ADMIN_EXAM = {
  id: "e2e-admin-exam-id",
  email: "admin.exam@example.test",
  name: "Admin Exam",
  role: "admin_exam",
};

const ADMIN_SCHOOL = {
  id: "e2e-admin-school-id",
  email: "admin.school@example.test",
  name: "Admin School",
  role: "admin_school",
};

const TEST_DETAIL = {
  test: {
    id: "test-1",
    title: "TPS Bundle",
    subject: "TPS",
    topic: "Penalaran",
    duration_minutes: 60,
  },
  questions: [
    {
      question: {
        id: "question-1",
        format: "mcq",
        body: "<p>2 + 2 =?</p>",
        sort_order: 1,
        point_correct: 4,
        point_wrong: 1,
      },
      options: [
        { question_id: "question-1", key: "a", text: "3", is_correct: false, sort_order: 1 },
        { question_id: "question-1", key: "b", text: "4", is_correct: true, sort_order: 2 },
      ],
    },
  ],
};

const EXAM_DETAIL = {
  id: "exam-1",
  title: "Tryout Bundle",
  scheduled_at: "2026-08-01T08:00:00Z",
  timer_mode: "overall",
  duration_minutes: 60,
  is_free: false,
  requires_checkin: false,
  allow_leaderboard: false,
  randomize: false,
  status: "draft",
  mode: "standard",
  certificate_enabled: false,
  card_enabled: false,
  result_config: "hidden",
  tests: [
    {
      id: "exam-test-1",
      exam_id: "exam-1",
      test_id: "test-1",
      sort_order: 1,
      test: { id: "test-1", title: "TPS Bundle", subject: "TPS", topic: "Penalaran", duration_minutes: 60, question_count: 1 },
    },
  ],
};

async function mockQuestionBundleBackend(page: Page) {
  const requests: string[] = [];
  await page.route("**/api/v1/**", async (route) => {
    const req = route.request();
    const url = new URL(req.url());
    const path = url.pathname;
    const method = req.method();
    requests.push(`${method} ${path}`);

    if (method === "GET" && path === "/api/v1/admin/tests/test-1") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(TEST_DETAIL) });
    }
    if (method === "GET" && path === "/api/v1/admin/tests/test-1/questions") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: TEST_DETAIL.questions }) });
    }
    if (method === "GET" && path === "/api/v1/admin/topics") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: [{ id: "topic-1", name: "Penalaran", subject: "TPS" }] }) });
    }
    if (method === "GET" && path === "/api/v1/admin/exams/exam-1") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(EXAM_DETAIL) });
    }
    if (method === "GET" && path === "/api/v1/admin/tests") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: [TEST_DETAIL.test] }) });
    }
    if (method === "POST" && (path === "/api/v1/admin/tests/test-1/question-bundle" || path === "/api/v1/admin/exams/exam-1/question-bundle")) {
      const body = req.postDataJSON() as { include_answer_key?: boolean };
      return route.fulfill({
        status: 202,
        contentType: "application/json",
        body: JSON.stringify({
          id: body.include_answer_key ? "bundle-key" : "bundle-paper",
          scope_type: path.includes("/exams/") ? "exam" : "test",
          scope_id: path.includes("/exams/") ? "exam-1" : "test-1",
          variant: body.include_answer_key ? "kunci" : "naskah",
          status: "queued",
          created_at: "2026-08-25T01:00:00Z",
          updated_at: "2026-08-25T01:00:00Z",
        }),
      });
    }
    if (method === "GET" && (path === "/api/v1/admin/question-bundles/bundle-key" || path === "/api/v1/admin/question-bundles/bundle-paper")) {
      const key = path.endsWith("bundle-key");
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: key ? "bundle-key" : "bundle-paper",
          scope_type: "test",
          scope_id: "test-1",
          variant: key ? "kunci" : "naskah",
          status: "ready",
          created_at: "2026-08-25T01:00:00Z",
          updated_at: "2026-08-25T01:00:01Z",
          generated_at: "2026-08-25T01:00:01Z",
        }),
      });
    }
    if (method === "GET" && (path === "/api/v1/admin/question-bundles/bundle-key/download" || path === "/api/v1/admin/question-bundles/bundle-paper/download")) {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ url: "https://files.example/question-bundle.pdf" }) });
    }

    return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
  });
  return requests;
}

test("admin_exam can enqueue and download a test question bundle", async ({ page, context }) => {
  await seedSession(context, { token: "token", refreshToken: "refresh", user: ADMIN_EXAM });
  const requests = await mockQuestionBundleBackend(page);
  await page.goto("/admin/exam/tests/test-1");

  await expect(page.getByTestId("test-question-bundle-controls")).toBeVisible();
  await page.getByRole("button", { name: "Buat kunci" }).click();
  await expect(page.getByTestId("question-bundle-status")).toContainText("siap diunduh");
  await page.getByRole("button", { name: "Unduh PDF" }).click();

  expect(requests).toContain("POST /api/v1/admin/tests/test-1/question-bundle");
  expect(requests).toContain("GET /api/v1/admin/question-bundles/bundle-key/download");
});

test("admin_school does not see question bundle controls", async ({ page, context }) => {
  await seedSession(context, { token: "token", refreshToken: "refresh", user: ADMIN_SCHOOL });
  await mockQuestionBundleBackend(page);
  await page.goto("/admin/exam/packages/exam-1");

  await expect(page.getByTestId("exam-question-bundle-controls")).toHaveCount(0);
});
