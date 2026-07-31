import { test, expect, type Page } from "@playwright/test";
import { loginRealAdmin, fetchSavedQuestion } from "./helpers/session";

/**
 * Full-flow coverage for the bank-soal walkthrough batch (PR #68): numbered
 * pagination, the short/fill_blank format merge, min-4/max-8 options, the
 * Benar/Salah segmented toggle, and per-item points (migration 0050) surviving
 * the REAL save → database → reload round trip.
 *
 * Real-backend pattern — see helpers/session.ts for the credentials contract.
 */

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
    const token = await loginRealAdmin(context, request);
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

    const q = await fetchSavedQuestion(request, token, marker);
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
    const token = await loginRealAdmin(context, request);
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

    const q = await fetchSavedQuestion(request, token, marker);
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
    const token = await loginRealAdmin(context, request);
    const marker = `e2emb${Date.now()}`;
    await openCreate(page, "multi_blank");

    // two default blank rows; the body must carry a matching token per row
    await typeBody(page, `${marker} isi {{1}} dan {{2}}`);

    const answerInputs = page.getByLabel("Jawaban yang diterima");
    await answerInputs.nth(0).fill("satu");
    await answerInputs.nth(1).fill("dua");

    await page.getByLabel("Poin", { exact: true }).nth(0).fill("3");

    await save(page);

    const q = await fetchSavedQuestion(request, token, marker);
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
