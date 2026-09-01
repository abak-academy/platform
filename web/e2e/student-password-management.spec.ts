import { test, expect, type Page } from "@playwright/test";
import { seedSession } from "./helpers/session";

const STUDENTS = [
  {
    id: "st1",
    name: "Budi Santoso",
    username: "budi",
    jenjang: "SMA",
    email: "budi@example.test",
    status: "active",
    grade: 10,
    school_name: "SMAN 1 Jakarta",
    created_at: "2026-08-01T01:00:00Z",
  },
];

const SCHOOLS = {
  data: [{ id: "s1", name: "SMAN 1 Jakarta", school_types: ["SMA"] }],
  next_cursor: "",
};

async function mockBackend(page: Page, captures: { register?: unknown; reset?: unknown } = {}) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" }),
  );
  await page.route("**/api/v1/admin/students/*/password", async (route) => {
    captures.reset = route.request().postDataJSON();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ message: "password updated" }),
    });
  });
  await page.route("**/api/v1/admin/students*", async (route) => {
    if (route.request().method() === "GET") {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: STUDENTS, next_cursor: "" }),
      });
    }
    if (route.request().method() === "POST") {
      captures.register = route.request().postDataJSON();
      return route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          id: "st-new",
          name: "Manual Password",
          username: "manual",
          jenjang: "SMA",
          status: "active",
          created_at: "2026-08-01T01:00:00Z",
        }),
      });
    }
    return route.fallback();
  });
  await page.route("**/api/v1/admin/schools/options*", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(SCHOOLS) }),
  );
  await page.route("**/api/v1/schools*", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(SCHOOLS.data) }),
  );
  await page.route("**/api/v1/provinces*", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "[]" }),
  );
}

test("super_admin can create and set explicit student passwords without rendering submitted secrets", async ({
  page,
  context,
}) => {
  await seedSession(context, {
    token: "super-token",
    refreshToken: "refresh",
    user: { id: "super", role: "super_admin", name: "Super Admin" },
  });
  const captures: { register?: unknown; reset?: unknown } = {};
  await mockBackend(page, captures);

  await page.goto("/admin/school/students");
  await expect(page.getByText("Budi Santoso")).toBeVisible();

  await page.getByRole("button", { name: /daftarkan siswa/i }).click();
  await page.getByPlaceholder("Nama Lengkap").fill("Manual Password");
  await page.getByPlaceholder("Password").fill("chosenPass123");
  await page.getByRole("combobox").nth(2).click();
  await page.getByRole("option", { name: "SMA" }).click();
  await page.getByRole("dialog").getByRole("button", { name: /daftarkan siswa/i }).click();
  const credentialDialog = page.getByRole("dialog");
  await expect(credentialDialog).toBeVisible();
  await expect(credentialDialog.getByText("Kredensial Siswa")).toBeVisible();
  await expect(credentialDialog.getByText("manual")).toBeVisible();
  await expect(credentialDialog.getByText("Password Sementara")).toHaveCount(0);
  expect(captures.register).toEqual({
    name: "Manual Password",
    jenjang: "SMA",
    password: "chosenPass123",
  });
  await expect(page.getByText("chosenPass123")).toHaveCount(0);
  await credentialDialog.getByRole("button", { name: "Batal" }).click();
  await expect(credentialDialog).toHaveCount(0);

  await page.locator("tr", { hasText: "Budi Santoso" }).getByRole("button").click();
  await page.getByText("Set Password").click();
  await page.getByPlaceholder("Password Baru").fill("newPass123");
  await page.getByPlaceholder("Konfirmasi Password").fill("newPass123");
  await page.getByRole("dialog").getByRole("button", { name: "Set Password" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(captures.reset).toEqual({ new_password: "newPass123" });
  await expect(page.getByText("newPass123")).toHaveCount(0);
});

test("admin_school has no explicit password controls or bulk password guidance", async ({
  page,
  context,
}) => {
  await seedSession(context, {
    token: "school-token",
    refreshToken: "refresh",
    user: { id: "school-admin", role: "admin_school", school_id: "s1", name: "School Admin" },
  });
  await mockBackend(page);

  await page.goto("/admin/school/students");
  await expect(page.getByText("Budi Santoso")).toBeVisible();

  await page.getByRole("button", { name: /daftarkan siswa/i }).click();
  await expect(page.getByPlaceholder("Password")).toHaveCount(0);
  await page.getByRole("button", { name: "Batal" }).click();

  await page.locator("tr", { hasText: "Budi Santoso" }).getByRole("button").click();
  await expect(page.getByText("Terbitkan Ulang Kredensial")).toBeVisible();
  await expect(page.getByText("Set Password")).toHaveCount(0);

  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: /pendaftaran siswa massal/i }).click();
  await expect(page.getByText("bulk_format_student_password")).toHaveCount(0);
  await expect(page.getByText("password")).toHaveCount(0);
});
