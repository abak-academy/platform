import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

const ADMIN_EXAM = {
  id: "e2e-admin-exam-id",
  email: "admin.exam@example.test",
  name: "Admin Exam",
  role: "admin_exam",
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
  const requested = new Set<string>();
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
    if (method === "POST" && (path === "/api/v1/admin/tests/test-1/question-bundles/kunci" || path === "/api/v1/admin/tests/test-1/question-bundles/naskah")) {
      const body = req.postDataJSON() as { template?: { document?: string } };
      expect(body.template?.document).toContain("{{tests_html}}");
      requested.add(path);
      return route.fulfill({
        status: 202,
        contentType: "application/json",
        body: JSON.stringify({
          test_id: "test-1",
          variant: path.endsWith("/kunci") ? "kunci" : "naskah",
          status: "queued",
        }),
      });
    }
    if (method === "GET" && path.startsWith("/api/v1/admin/tests/test-1/question-bundles/") && !path.endsWith("/download")) {
      const key = path.endsWith("/kunci");
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          test_id: "test-1",
          variant: key ? "kunci" : "naskah",
          status: requested.has(path) ? "ready" : "idle",
          generated_at: requested.has(path) ? "2026-08-25T01:00:01Z" : undefined,
        }),
      });
    }
    if (method === "GET" && path.startsWith("/api/v1/admin/tests/test-1/question-bundles/") && path.endsWith("/download")) {
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
  const keyControls = page.getByTestId("question-bundle-kunci");
  await keyControls.getByRole("button", { name: "Buat PDF" }).click();
  await expect(page.getByTestId("question-bundle-kunci-status")).toContainText("siap");
  await keyControls.getByRole("button", { name: "Unduh" }).click();

  expect(requests).toContain("POST /api/v1/admin/tests/test-1/question-bundles/kunci");
  expect(requests).toContain("GET /api/v1/admin/tests/test-1/question-bundles/kunci/download");
});

test("exam packages never expose aggregate question bundle controls", async ({ page, context }) => {
  await seedSession(context, { token: "token", refreshToken: "refresh", user: ADMIN_EXAM });
  const requests = await mockQuestionBundleBackend(page);
  await page.goto("/admin/exam/packages/exam-1");

  await expect(page.getByTestId("exam-question-bundle-controls")).toHaveCount(0);
  expect(requests.some((request) => request.includes("/admin/exams/exam-1/question-bundles/"))).toBe(false);
});
