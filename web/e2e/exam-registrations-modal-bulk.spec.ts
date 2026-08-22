import { test, expect, type Page, type BrowserContext } from "@playwright/test";
import { seedSession } from "./helpers/session";

const EXAM_ID = "e2e-exam";
const EXAM_TITLE = "E2E Registration Exam";

const SUPER_ADMIN = {
  id: "e2e-super-admin",
  email: "super@example.test",
  name: "E2E Super Admin",
  role: "super_admin",
};

const ADMIN_SCHOOL = {
  id: "e2e-admin-school",
  email: "school@example.test",
  name: "E2E School Admin",
  role: "admin_school",
  school_id: "school-1",
};

const ADMIN_EXAM = {
  id: "e2e-admin-exam",
  email: "exam@example.test",
  name: "E2E Exam Admin",
  role: "admin_exam",
};

const ROSTER_ROWS = [
  {
    registration_id: "reg-1",
    student_id: "student-existing",
    student_name: "Existing Student",
    student_username: "existing1",
    participant_number: 1,
    participant_no: "250822-000001",
    status: "registered",
    checked_in_at: null,
    token: "TOKEN-EXISTING",
  },
];

const PICKER_STUDENTS = [
  {
    id: "student-1",
    name: "Andi Saputra",
    username: "andi123",
    jenjang: "sma",
    grade: 11,
    school_id: "school-1",
    school_name: "SMA E2E",
    status: "active",
  },
  {
    id: "student-2",
    name: "Budi Santoso",
    username: "budi456",
    jenjang: "sma",
    grade: 11,
    school_id: "school-1",
    school_name: "SMA E2E",
    status: "active",
  },
];

function examDetail() {
  return {
    id: EXAM_ID,
    title: EXAM_TITLE,
    scheduled_at: "2026-08-22T01:00:00Z",
    scheduled_end_at: "2026-08-22T03:00:00Z",
    timer_mode: "overall",
    duration_minutes: 120,
    mode: "utbk",
    is_free: false,
    requires_checkin: true,
    allow_leaderboard: true,
    randomize: false,
    status: "published",
    certificate_enabled: false,
    card_enabled: true,
    tests: [],
  };
}

async function seedAdmin(context: BrowserContext, user: Record<string, unknown>) {
  await seedSession(context, {
    token: "e2e-fake-token",
    refreshToken: "e2e-fake-refresh",
    user,
  });
}

async function mockExamRegistrationsBackend(page: Page) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" }),
  );

  await page.route(`**/api/v1/admin/exams/${EXAM_ID}`, (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(examDetail()),
    });
  });

  await page.route(`**/api/v1/admin/exams/${EXAM_ID}/registrations*`, (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: ROSTER_ROWS, next_cursor: "" }),
    });
  });

  await page.route("**/api/v1/admin/exam-grants/students/search*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: PICKER_STUDENTS, next_cursor: "" }),
    });
  });

  await page.route("**/api/v1/admin/students*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: PICKER_STUDENTS, next_cursor: "" }),
    });
  });

  await page.route("**/api/v1/admin/schools/options*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: [{ id: "school-1", name: "SMA E2E" }] }),
    });
  });

  await page.route("**/api/v1/admin/exam-grants", (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        granted_count: 2,
        granted_students: [
          { id: "student-1", name: "Andi Saputra", username: "andi123" },
          { id: "student-2", name: "Budi Santoso", username: "budi456" },
        ],
      }),
    });
  });

  await page.route("**/api/v1/admin/bulk-exam-orders/preview", (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ net_new_count: 2, excluded: [], unit_price: 75000, total: 150000 }),
    });
  });

  await page.route("**/api/v1/admin/bulk-exam-orders", (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ id: "order-1", status: "pending", total: 150000 }),
    });
  });

  await page.route("**/api/v1/admin/exam-grants/bulk/presign*", (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        url: "http://localhost:3000/__fake-exam-grant-upload",
        key: `exam-grant-bulk/${EXAM_ID}/upload.csv`,
      }),
    });
  });

  await page.route("**/__fake-exam-grant-upload", (route) => {
    if (route.request().method() !== "PUT") return route.fallback();
    return route.fulfill({ status: 200, body: "" });
  });

  await page.route("**/api/v1/admin/exam-grants/bulk", (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    return route.fulfill({
      status: 202,
      contentType: "application/json",
      body: JSON.stringify({ job_id: "job-1" }),
    });
  });

  await page.route("**/api/v1/admin/jobs/job-1", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        id: "job-1",
        type: "exam_grant_bulk",
        status: "succeeded",
        progress: 100,
        result_url: "http://localhost:3000/e2e-result.csv",
      }),
    });
  });
}

async function openRegistrationsTab(page: Page) {
  await page.goto(`/admin/exam/packages/${EXAM_ID}`);
  await expect(page.getByRole("heading", { name: EXAM_TITLE })).toBeVisible();
  await page.getByRole("button", { name: /Pendaftaran|Registrations/i }).click();
  await expect(page.getByText("Existing Student")).toBeVisible();
}

test("super_admin registrations flow is roster-first, manual grant modal works, and close resets state", async ({ page, context }) => {
  const errors: string[] = [];
  page.on("console", (msg) => {
    if (msg.type() === "error") errors.push(msg.text());
  });
  page.on("pageerror", (err) => errors.push(err.message));

  await seedAdmin(context, SUPER_ADMIN);
  await mockExamRegistrationsBackend(page);
  await openRegistrationsTab(page);

  await expect(page.getByTestId("open-grant-modal")).toBeVisible();
  await expect(page.getByText("Andi Saputra")).toHaveCount(0);

  await page.getByTestId("open-grant-modal").click();
  const dialog = page.getByRole("dialog", { name: /Beri Akses|Grant Access/i });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByTestId("grant-mode-toggle")).toBeVisible();

  await dialog.getByRole("checkbox", { name: "Andi Saputra" }).click();
  await dialog.getByRole("checkbox", { name: "Budi Santoso" }).click();
  await dialog.getByRole("button", { name: /Beri Akses|Grant Access/i }).last().click();

  await expect(dialog.getByText("Andi Saputra")).toBeVisible();
  await expect(dialog.getByText("@andi123")).toBeVisible();

  await dialog.getByRole("button", { name: "Close" }).click();
  await expect(dialog).toBeHidden();
  await page.getByTestId("open-grant-modal").click();
  const reopenedDialog = page.getByRole("dialog", { name: /Beri Akses|Grant Access/i });
  await expect(reopenedDialog.getByTestId("grant-mode-manual")).toBeVisible();
  await expect(reopenedDialog.getByText(/berhasil|success/i)).toHaveCount(0);
  await expect(reopenedDialog.getByRole("checkbox", { name: "Andi Saputra" })).toHaveAttribute(
    "aria-checked",
    "false",
  );

  expect(errors).toEqual([]);
});

test("super_admin CSV bulk grant mode uploads, polls job, and exposes result download", async ({ page, context }) => {
  const requests: string[] = [];
  await seedAdmin(context, SUPER_ADMIN);
  await mockExamRegistrationsBackend(page);
  page.on("request", (req) => requests.push(`${req.method()} ${req.url()}`));

  await openRegistrationsTab(page);
  await page.getByTestId("open-grant-modal").click();
  const dialog = page.getByRole("dialog", { name: /Beri Akses|Grant Access/i });
  await dialog.getByTestId("grant-mode-csv").click();

  await expect(dialog.getByTestId("csv-download-template")).toBeVisible();
  await dialog.getByTestId("csv-file-input").setInputFiles({
    name: "grant.csv",
    mimeType: "text/csv",
    buffer: Buffer.from("username\nandi123\nbudi456\n"),
  });
  await dialog.getByTestId("csv-upload-submit").click();

  await expect(dialog.getByText(/Selesai|Done/i)).toBeVisible();
  await expect(dialog.getByRole("link", { name: /Unduh Hasil|Download Result/i })).toHaveAttribute(
    "href",
    "http://localhost:3000/e2e-result.csv",
  );

  expect(requests.some((r) => r.includes("POST") && r.includes("/admin/exam-grants/bulk/presign"))).toBe(true);
  expect(requests.some((r) => r.includes("PUT") && r.includes("/__fake-exam-grant-upload"))).toBe(true);
  expect(requests.some((r) => r.includes("POST") && r.includes("/admin/exam-grants/bulk"))).toBe(true);
  expect(requests.some((r) => r.includes("GET") && r.includes("/admin/jobs/job-1"))).toBe(true);
});

test("admin_school keeps order flow inside modal through preview/create/SnapCheckout", async ({ page, context }) => {
  await seedAdmin(context, ADMIN_SCHOOL);
  await mockExamRegistrationsBackend(page);

  await openRegistrationsTab(page);
  await expect(page.getByTestId("open-order-modal")).toBeVisible();
  await expect(page.getByTestId("open-grant-modal")).toHaveCount(0);

  await page.getByTestId("open-order-modal").click();
  const dialog = page.getByRole("dialog", { name: /Pilih peserta|Pick participants/i });
  await dialog.getByRole("checkbox", { name: "Andi Saputra" }).click();
  await dialog.getByRole("checkbox", { name: "Budi Santoso" }).click();
  await dialog.getByRole("button", { name: /Pratinjau Pesanan|Preview Order/i }).click();

  await expect(dialog.getByText(/Rp\s*150\.000|150,000|150000/i)).toBeVisible();
  await dialog.getByRole("button", { name: /Buat Pesanan|Create Order/i }).click();

  await expect(dialog.getByText(/Pesanan massal berhasil dibuat|Bulk order created/i)).toBeVisible();
  await expect(dialog.getByRole("button", { name: /Bayar di Tab Baru/i })).toBeVisible();
});

test("admin_exam sees roster only and no write controls", async ({ page, context }) => {
  await seedAdmin(context, ADMIN_EXAM);
  await mockExamRegistrationsBackend(page);

  await openRegistrationsTab(page);
  await expect(page.getByText("Existing Student")).toBeVisible();
  await expect(page.getByText(/Ekspor CSV|Export CSV/i)).toBeVisible();
  await expect(page.getByTestId("open-grant-modal")).toHaveCount(0);
  await expect(page.getByTestId("open-order-modal")).toHaveCount(0);
  await expect(page.getByText("Andi Saputra")).toHaveCount(0);
});
