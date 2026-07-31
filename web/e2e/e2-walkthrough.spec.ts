import { test, expect, type BrowserContext, type Page } from "@playwright/test";

/**
 * Full-flow coverage for the bank-soal walkthrough batch (PR #68): numbered
 * pagination, the short/fill_blank format merge, min-4/max-8 options, the
 * Benar/Salah segmented toggle, and per-item points (migration 0050) surviving
 * the REAL save → database → reload round trip.
 *
 * Every case here follows question-editor.spec.ts's real-backend pattern —
 * these are wire/durability claims, so mocking the API would prove nothing.
 * Needs E2E_ADMIN_IDENTIFIER / E2E_ADMIN_PASSWORD (same contract as FB-24).
 */

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";

interface SeededSession {
  token: string;
  refreshToken: string;
  user: Record<string, unknown>;
}

async function seedSession(context: BrowserContext, session: SeededSession) {
  await context.addInitScript((s) => {
    window.localStorage.setItem(
      "abak-auth",
      JSON.stringify({ state: { token: s.token, refreshToken: s.refreshToken, user: s.user }, version: 0 })
    );
  }, session);
}

let cachedToken = "";

async function loginRealAdmin(context: BrowserContext, request: import("@playwright/test").APIRequestContext) {
  const identifier = process.env.E2E_ADMIN_IDENTIFIER;
  const password = process.env.E2E_ADMIN_PASSWORD;
  if (!identifier || !password) {
    throw new Error(
      "set E2E_ADMIN_IDENTIFIER and E2E_ADMIN_PASSWORD — this spec exercises the real backend."
    );
  }
  const loginRes = await request.post(`${API_BASE}/auth/login`, { data: { identifier, password } });
  if (!loginRes.ok()) {
    throw new Error(`login failed (${loginRes.status()}): ${await loginRes.text()}`);
  }
  const session = (await loginRes.json()) as {
    access_token: string;
    refresh_token?: string;
    user: Record<string, unknown>;
  };
  cachedToken = session.access_token;
  await seedSession(context, {
    token: session.access_token,
    refreshToken: session.refresh_token ?? "",
    user: session.user,
  });
}

// Reads the saved question back over the wire — the same list payload the
// edit dialog consumes, so a points value visible here is a points value the
// admin will see on reopen.
async function fetchSavedQuestion(
  request: import("@playwright/test").APIRequestContext,
  marker: string
) {
  const res = await request.get(
    `${API_BASE}/admin/questions?search=${encodeURIComponent(marker)}`,
    { headers: { Authorization: `Bearer ${cachedToken}` } }
  );
  expect(res.ok(), `list lookup for ${marker} should succeed`).toBeTruthy();
  const body = (await res.json()) as { data: Array<Record<string, any>> };
  expect(body.data.length, `exactly one question should match ${marker}`).toBe(1);
  return body.data[0];
}

async function openCreate(page: Page, format: string) {
  await page.goto("/admin/exam/questions");
  await page.getByRole("button", { name: "Buat", exact: true }).click();
  await page.locator("#question-format").selectOption(format);
  const topicOptionCount = await page.locator("#question-topic option").count();
  expect(topicOptionCount, "seed at least one exam topic locally").toBeGreaterThan(1);
  await page.locator("#question-topic").selectOption({ index: 1 });
}

async function typeBody(page: Page, text: string) {
  const body = page.locator("#question-body");
  await body.click();
  await page.keyboard.type(text);
}

async function save(page: Page) {
  await page.getByRole("button", { name: "Simpan soal" }).click();
  await expect(page.getByRole("dialog")).toBeHidden({ timeout: 10000 });
}

test.describe("bank-soal walkthrough batch — full flow", () => {
  test("numbered pagination reaches the second page and older questions", async ({
    page,
    context,
    request,
  }) => {
    await loginRealAdmin(context, request);
    await page.goto("/admin/exam/questions");

    // The summary only renders when total > page size — the local DB has 40+.
    const summary = page.getByText(/Hal\. 1 dari \d+/);
    await expect(summary).toBeVisible({ timeout: 10000 });

    const firstIdPage1 = await page.locator("tbody tr td:first-child").first().innerText();

    await page.getByRole("button", { name: "2", exact: true }).click();
    await expect(page.getByText(/Hal\. 2 dari \d+/)).toBeVisible();

    const firstIdPage2 = await page.locator("tbody tr td:first-child").first().innerText();
    expect(Number(firstIdPage2)).toBeLessThan(Number(firstIdPage1));
  });

  test("format merge — creation picker and list filter offer exactly the five live formats", async ({
    page,
    context,
    request,
  }) => {
    await loginRealAdmin(context, request);
    await page.goto("/admin/exam/questions");

    // list filter (first select on the page)
    const filterValues = await page
      .locator("select")
      .first()
      .locator("option")
      .evaluateAll((opts) => opts.map((o) => (o as HTMLOptionElement).value));
    expect(filterValues).toEqual(["all", "mcq", "multi_answer", "essay", "multi_blank", "true_false"]);

    await page.getByRole("button", { name: "Buat", exact: true }).click();
    const pickerValues = await page
      .locator("#question-format option")
      .evaluateAll((opts) => opts.map((o) => (o as HTMLOptionElement).value));
    expect(pickerValues).toEqual(["mcq", "multi_answer", "essay", "multi_blank", "true_false"]);
  });

  test("multi_answer — 4 options by default, 8 max, per-option points survive save and reload", async ({
    page,
    context,
    request,
  }) => {
    await loginRealAdmin(context, request);
    const marker = `e2ema${Date.now()}`;
    await openCreate(page, "multi_answer");

    // min-4 default, remove blocked
    await expect(page.locator('input[type="checkbox"]')).toHaveCount(4);
    await expect(page.getByRole("button", { name: "Hapus opsi" }).first()).toBeDisabled();

    // grows to exactly 8, then add locks
    const add = page.getByRole("button", { name: "Tambah opsi" });
    for (let i = 0; i < 4; i++) await add.click();
    await expect(page.locator('input[type="checkbox"]')).toHaveCount(8);
    await expect(add).toBeDisabled();

    await typeBody(page, `${marker} pilih dua`);

    // options a and b correct; a carries its own worth
    const checkboxes = page.locator('input[type="checkbox"]');
    await checkboxes.nth(0).check();
    await checkboxes.nth(1).check();

    const pointInputs = page.getByLabel("Poin", { exact: true });
    await expect(pointInputs).toHaveCount(8);
    // wrong options' inputs stay disabled — the worth relationship is visible
    await expect(pointInputs.nth(2)).toBeDisabled();
    await pointInputs.nth(0).fill("2");

    // every option needs text; fill all 8 via their contenteditable fields
    const optionFields = page.locator('[id^="option-text-"]');
    const count = await optionFields.count();
    for (let i = 0; i < count; i++) {
      await optionFields.nth(i).click();
      await page.keyboard.type(`opsi${i + 1}`);
    }

    await save(page);

    const q = await fetchSavedQuestion(request, marker);
    const options = q.options as Array<{ key: string; is_correct: boolean; points?: number }>;
    expect(options.length).toBe(8);
    expect(options.find((o) => o.key === "a")?.points).toBe(2);
    expect(options.find((o) => o.key === "b")?.points).toBeUndefined();
    expect(options.find((o) => o.key === "b")?.is_correct).toBe(true);
  });

  test("true_false — segmented toggle + per-statement points on one flow, wire round trip", async ({
    page,
    context,
    request,
  }) => {
    await loginRealAdmin(context, request);
    const marker = `e2etf${Date.now()}`;
    await openCreate(page, "true_false");
    await typeBody(page, `${marker} benar salah`);

    const statementInputs = page.getByLabel("Isi pernyataan");
    await statementInputs.nth(0).fill("Pernyataan pertama");
    await statementInputs.nth(1).fill("Pernyataan kedua");

    // statement 1 -> Benar via the segmented toggle (not a checkbox)
    const trueButtons = page.getByRole("button", { name: "Benar", exact: true });
    await trueButtons.nth(0).click();
    await expect(trueButtons.nth(0)).toHaveAttribute("aria-pressed", "true");

    // statement 1 worth 5, statement 2 inherits
    await page.getByLabel("Poin", { exact: true }).nth(0).fill("5");

    await save(page);

    const q = await fetchSavedQuestion(request, marker);
    const statements = (q.question?.statements ?? q.statements) as Array<{
      index: number;
      is_true: boolean;
      points?: number;
    }>;
    expect(statements.length).toBe(2);
    expect(statements.find((s) => s.index === 1)?.is_true).toBe(true);
    expect(statements.find((s) => s.index === 1)?.points).toBe(5);
    expect(statements.find((s) => s.index === 2)?.points).toBeUndefined();
  });

  test("multi_blank — per-blank points persist and the preview shows chips, not {{N}} tokens", async ({
    page,
    context,
    request,
  }) => {
    await loginRealAdmin(context, request);
    const marker = `e2emb${Date.now()}`;
    await openCreate(page, "multi_blank");

    // two default blank rows; the body must carry a matching token per row
    await typeBody(page, `${marker} isi {{1}} dan {{2}}`);

    const answerInputs = page.getByLabel("Jawaban yang diterima");
    await answerInputs.nth(0).fill("satu");
    await answerInputs.nth(1).fill("dua");

    await page.getByLabel("Poin", { exact: true }).nth(0).fill("3");

    await save(page);

    const q = await fetchSavedQuestion(request, marker);
    const blanks = q.blanks as Array<{ index: number; points?: number }>;
    expect(blanks.find((b) => b.index === 1)?.points).toBe(3);
    expect(blanks.find((b) => b.index === 2)?.points).toBeUndefined();

    // preview renders chips in place of tokens — the confusion reported on
    // 2026-07-31 was literal {{1}} text in this exact dialog
    await page.reload();
    await page.getByRole("textbox", { name: "Cari…" }).fill(marker);
    const row = page.getByRole("row", { name: new RegExp(marker) });
    await expect(row).toBeVisible({ timeout: 10000 });
    await row.click();

    const preview = page.locator("[data-rich-content]").first();
    await expect(preview).toBeVisible();
    await expect(preview.locator("[data-blank-chip]")).toHaveCount(2);
    await expect(preview).not.toContainText("{{1}}");
  });
});
