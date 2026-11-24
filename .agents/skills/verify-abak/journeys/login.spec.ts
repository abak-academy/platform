import { test, expect, type Page } from "@playwright/test";
import { appendFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const evidence = process.env.VERIFY_EVIDENCE_DIR!;
const api = process.env.VERIFY_API_URL!;

test.beforeEach(async ({ page }, info) => {
  const log = (line: string) => appendFileSync(join(evidence, `${info.title.split(":")[0]}.browser.log`), line + "\n");
  page.on("console", (m) => log(`console.${m.type()} ${m.text()}`));
  page.on("pageerror", (e) => log(`pageerror ${e.message}`));
  page.on("requestfailed", (r) => log(`requestfailed ${r.method()} ${r.url()} ${r.failure()?.errorText}`));
  page.on("response", (r) => r.url().startsWith(api) && log(`response ${r.status()} ${r.request().method()} ${r.url()}`));
});

async function proveSession(page: Page, name: string, email: string, role: string) {
  const stored = await page.evaluate(() => window.localStorage.getItem("abak-auth"));
  expect(stored, "abak-auth must be written after login").not.toBeNull();
  const token = JSON.parse(stored!).state.token as string;
  const me = await page.request.get(`${api}/auth/me`, { headers: { Authorization: `Bearer ${token}` } });
  expect(me.status()).toBe(200);
  const body = await me.json();
  writeFileSync(join(evidence, `${name}.me.json`), JSON.stringify({ status: me.status(), body }, null, 2));
  expect(JSON.stringify(body)).toContain(email);
  expect(JSON.stringify(body)).toContain(role);
  writeFileSync(join(evidence, `${name}.aria.txt`), await page.locator("body").ariaSnapshot());
  await page.screenshot({ path: join(evidence, `${name}.after.png`), fullPage: true });
}

async function fillLogin(page: Page, email: string, password: string, name: string) {
  await page.getByLabel(/^(Email atau Username|Email or Username)$/).fill(email);
  await page.getByLabel(/^(Kata Sandi|Password)$/).fill(password);
  await page.screenshot({ path: join(evidence, `${name}.before.png`) });
  await page.getByRole("button", { name: /^(Masuk|Sign in)$/ }).click();
}

test("login-student: /login signs a student in and lands on the student home", async ({ page }) => {
  const email = process.env.VERIFY_STUDENT_EMAIL!;
  await page.goto("/login");
  await fillLogin(page, email, process.env.VERIFY_STUDENT_PASSWORD!, "login-student");
  await page.waitForURL((u) => u.pathname !== "/login", { timeout: 30_000 });
  writeFileSync(join(evidence, "login-student.url.txt"), page.url());
  await expect(page).not.toHaveURL(/\/login|\/admin/);
  await proveSession(page, "login-student", email, "student");
});

test("login-admin: /admin/login signs a super_admin in and lands on /admin", async ({ page }) => {
  const email = process.env.VERIFY_ADMIN_EMAIL!;
  await page.goto("/admin/login");
  await fillLogin(page, email, process.env.VERIFY_ADMIN_PASSWORD!, "login-admin");
  await page.waitForURL(/\/admin(\/(?!login).*)?$/, { timeout: 30_000 });
  writeFileSync(join(evidence, "login-admin.url.txt"), page.url());
  await proveSession(page, "login-admin", email, "super_admin");
});

test("login-wrong-password: a bad password stays on /login with an alert", async ({ page }) => {
  await page.goto("/login");
  await fillLogin(page, process.env.VERIFY_STUDENT_EMAIL!, "definitely-wrong", "login-wrong-password");
  await expect(page.getByRole("alert")).toBeVisible({ timeout: 15_000 });
  await expect(page).toHaveURL(/\/login$/);
  await page.screenshot({ path: join(evidence, "login-wrong-password.after.png") });
});
