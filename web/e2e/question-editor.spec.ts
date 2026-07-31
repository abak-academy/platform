import { test, expect, type BrowserContext, type Page } from "@playwright/test";

/**
 * Executable repro for FB-22, FB-23, FB-24 and FB-25 (spec.md "the editor
 * toolbar loses the caret" + the P0 line-break data-loss bug). These live in
 * jsdom's blind spot: vitest cannot drive document.execCommand or real
 * Selection/Range, so the only prior evidence was reading the code.
 *
 * This spec is written to describe CORRECT behaviour and is expected to FAIL
 * until tasks 2, 13 and 14 land the fixes. A failure here is the repro; a
 * "cannot find the editor" style error is a broken selector, not a repro —
 * see the run notes recorded in task_17_result.json for which is which.
 *
 * FB-22, FB-23, FB-25 seed a synthetic admin session and mock every backend
 * call the admin shell makes (topics, question list, image presign) — those
 * three bugs are pure browser/DOM defects in RichTextEditor/QuestionEditor
 * and do not need a real save. Mocking the backend here is mocking a system
 * boundary, not the thing under test.
 *
 * FB-24 is different: the P0 claim is that the REAL backend's
 * sanitizeQuestionBody (backend/internal/service/exam.go:83) strips <br>/<p>
 * on save. Faking that round trip would prove nothing, so FB-24 requires a
 * real admin session and hits the live API on :8080. Set
 * E2E_ADMIN_IDENTIFIER / E2E_ADMIN_PASSWORD to a local admin_exam or
 * super_admin account before running it — there is no way to mint one from
 * inside this spec (no seed script, no bootstrap endpoint), so this is the
 * one manual setup step left to whoever runs the suite locally.
 *
 * The table and math round-trip cases (Task 20) follow FB-24's real-backend
 * pattern for the same reason: a table's merged colspan and a live math
 * node's "\(latex\)" storage text are both save+reload claims, not DOM-only
 * ones.
 */

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";

const FAKE_ADMIN_USER = {
  id: "e2e-fake-admin-id",
  email: "e2e-fake-admin@example.test",
  name: "E2E Fake Admin",
  role: "admin_exam",
};

// 1x1 transparent PNG.
const TINY_PNG_BASE64 =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=";

interface SeededSession {
  token: string;
  refreshToken: string;
  user: Record<string, unknown>;
}

// Mirrors the zustand persist shape stores/auth.ts writes under the
// "abak-auth" localStorage key.
async function seedSession(context: BrowserContext, session: SeededSession) {
  await context.addInitScript((s) => {
    window.localStorage.setItem(
      "abak-auth",
      JSON.stringify({ state: { token: s.token, refreshToken: s.refreshToken, user: s.user }, version: 0 })
    );
  }, session);
}

// AdminLayout only trusts the persisted user.role (no /me call once a role is
// present), but authFetch hard-redirects to /login on any 401 — and AppShell
// fires unrelated queries (cart, topics, question list) on every admin page.
// A garbage token would 401 those and bounce us out before the dialog even
// opens, so everything under /api/v1 gets a safe default, then the two
// endpoints the editor dialog actually reads get real-shaped fixtures.
async function mockBackendForFakeSession(page: Page) {
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "{}" })
  );
  await page.route("**/api/v1/admin/topics*", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: [{ id: "e2e-topic-1", name: "E2E Topic", subject: "general" }] }),
    })
  );
  await page.route("**/api/v1/admin/questions*", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: [], next_cursor: null }),
    });
  });
  await page.route("**/api/v1/admin/uploads/image*", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ url: "http://localhost:8080/e2e-fake-upload", method: "PUT", key: "question/e2e-fake.png" }),
    })
  );
  await page.route("**/e2e-fake-upload", (route) => route.fulfill({ status: 200, body: "" }));
  // The inserted <img> reads its src back through the read-proxy — without a
  // real image body here the browser fails the load and the element renders
  // 0x0 (Playwright reports it "hidden"), which would be a harness artifact,
  // not evidence about the caret bug.
  await page.route("**/api/v1/files/question/e2e-fake.png", (route) =>
    route.fulfill({ status: 200, contentType: "image/png", body: Buffer.from(TINY_PNG_BASE64, "base64") })
  );
}

async function openCreateDialog(page: Page) {
  await page.goto("/admin/exam/questions");
  await page.getByRole("button", { name: "Buat", exact: true }).click();
  await expect(page.locator("#question-body")).toBeVisible();
}

// The default "mcq" format also renders a RichTextEditor per option, each
// with its own Bold/Italic/Insert-image toolbar — an unscoped getByRole
// match on those labels hits 3 elements. Scope to the question-body field's
// own toolbar (its great-grandparent per RichTextEditor's JSX: toolbar and
// the contenteditable are siblings two levels up from #question-body).
function bodyEditorScope(page: Page) {
  return page.locator("#question-body").locator("xpath=ancestor::div[2]");
}

// A real mouse-drag selection over a word — dispatches genuine
// mousedown/mousemove/mouseup, unlike a scripted Range/Selection, which is
// exactly what jsdom cannot produce. Coordinates come from the live
// Range.getBoundingClientRect() of the word's text node, not a guessed pixel
// offset, so it's robust to font metrics.
//
// TipTap wraps typed text in a <p>, so the contenteditable's firstChild is
// that <p>, not a Text node — findTextNodeContaining walks down to it.
// Duplicated inside clickBeforeWord's own evaluate() below because
// Playwright serializes each callback standalone; it can't close over a
// helper defined out here.
async function selectWordByMouseDrag(page: Page, fieldSelector: string, word: string) {
  const rect = await page.locator(fieldSelector).evaluate((el, w) => {
    function findTextNodeContaining(node: Node, target: string): Text {
      const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
      let n: Node | null;
      while ((n = walker.nextNode())) {
        if (n.textContent?.includes(target)) return n as Text;
      }
      throw new Error(`no text node containing "${target}" found`);
    }
    const node = findTextNodeContaining(el, w);
    const idx = node.textContent!.indexOf(w);
    const range = document.createRange();
    range.setStart(node, idx);
    range.setEnd(node, idx + w.length);
    const r = range.getBoundingClientRect();
    return { left: r.left, right: r.right, top: r.top, bottom: r.bottom };
  }, word);
  const y = (rect.top + rect.bottom) / 2;
  await page.mouse.move(rect.left + 1, y);
  await page.mouse.down();
  await page.mouse.move(rect.right - 1, y, { steps: 5 });
  await page.mouse.up();
}

// Real mouse click placing a collapsed caret immediately before `word`,
// again via the word's own Range.getBoundingClientRect() rather than a
// guessed offset.
async function clickBeforeWord(page: Page, fieldSelector: string, word: string) {
  const point = await page.locator(fieldSelector).evaluate((el, w) => {
    function findTextNodeContaining(node: Node, target: string): Text {
      const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
      let n: Node | null;
      while ((n = walker.nextNode())) {
        if (n.textContent?.includes(target)) return n as Text;
      }
      throw new Error(`no text node containing "${target}" found`);
    }
    const node = findTextNodeContaining(el, w);
    const idx = node.textContent!.indexOf(w);
    const range = document.createRange();
    range.setStart(node, idx);
    range.setEnd(node, idx);
    const r = range.getBoundingClientRect();
    return { x: r.left, y: (r.top + r.bottom) / 2 };
  }, word);
  await page.mouse.click(point.x, point.y);
}

// Shared by every case below that must hit the real backend (FB-24, and the
// table/math round trips) rather than the mocked-fetch session the pure-DOM
// cases (FB-22/23/25) use.
async function loginRealAdmin(context: BrowserContext, request: import("@playwright/test").APIRequestContext) {
  const identifier = process.env.E2E_ADMIN_IDENTIFIER;
  const password = process.env.E2E_ADMIN_PASSWORD;
  if (!identifier || !password) {
    throw new Error(
      "this case exercises the real backend save/reload round trip and needs a real session: " +
        "set E2E_ADMIN_IDENTIFIER and E2E_ADMIN_PASSWORD to a local admin_exam or super_admin " +
        "account before running this spec. This is an environment/credentials gap, not a repro."
    );
  }
  const loginRes = await request.post(`${API_BASE}/auth/login`, { data: { identifier, password } });
  if (!loginRes.ok()) {
    throw new Error(
      `real-backend setup: login failed (${loginRes.status()}) — check E2E_ADMIN_IDENTIFIER/E2E_ADMIN_PASSWORD. ` +
        `Body: ${await loginRes.text()}`
    );
  }
  const session = (await loginRes.json()) as {
    access_token: string;
    refresh_token?: string;
    user: Record<string, unknown>;
  };
  await seedSession(context, {
    token: session.access_token,
    refreshToken: session.refresh_token ?? "",
    user: session.user,
  });
}

// Opens the create-question dialog with a real session, sets format and the
// first real topic — the shape FB-24, the table round trip and the math
// round trip all need before typing into the body.
async function openRealCreateDialog(page: Page, format: string) {
  await page.goto("/admin/exam/questions");
  await page.getByRole("button", { name: "Buat", exact: true }).click();
  await page.locator("#question-format").selectOption(format);
  const topicOptionCount = await page.locator("#question-topic option").count();
  expect(
    topicOptionCount,
    "no exam topics exist in the local database — seed at least one to run this case"
  ).toBeGreaterThan(1);
  await page.locator("#question-topic").selectOption({ index: 1 });
}

// Saves, reloads, finds the row by marker text and opens its read-only
// preview — the same real-database durability check FB-24 relies on.
async function saveReloadAndOpenPreview(page: Page, marker: string) {
  await page.getByRole("button", { name: "Simpan soal" }).click();
  await expect(page.getByRole("dialog")).toBeHidden({ timeout: 10000 });

  await page.reload();
  // Two "Cari…" inputs exist: the admin shell's global search is type="search"
  // (role searchbox); the question-list filter is a plain textbox.
  await page.getByRole("textbox", { name: "Cari…" }).fill(marker);
  const row = page.getByRole("row", { name: new RegExp(marker) });
  await expect(row, "saved question should be findable after reload").toBeVisible({ timeout: 10000 });
  await row.click();

  const preview = page.locator("[data-rich-content]").first();
  await expect(preview).toBeVisible();
  return preview;
}

test.describe("question editor toolbar — caret/selection defects", () => {
  test("FB-22 — Bold applies to the selection, not the whole field", async ({ page, context }) => {
    await mockBackendForFakeSession(page);
    await seedSession(context, { token: "e2e-fake-token", refreshToken: "e2e-fake-refresh", user: FAKE_ADMIN_USER });
    await openCreateDialog(page);

    await page.locator("#question-body").click();
    await page.keyboard.type("hello world");

    await selectWordByMouseDrag(page, "#question-body", "world");

    await bodyEditorScope(page).getByRole("button", { name: "Bold" }).click();

    const html = await page.locator("#question-body").innerHTML();
    const boldText = (await page.locator("#question-body b, #question-body strong").allTextContents()).join("");
    expect(boldText, "only \"world\" should be bold").toBe("world");
    expect(html, "the whole field must not be bolded").not.toMatch(/<(b|strong)>\s*hello world\s*<\/(b|strong)>/i);
  });

  test("FB-23 — an inserted image lands at the caret, not the end", async ({ page, context }) => {
    await mockBackendForFakeSession(page);
    await seedSession(context, { token: "e2e-fake-token", refreshToken: "e2e-fake-refresh", user: FAKE_ADMIN_USER });
    await openCreateDialog(page);

    await page.locator("#question-body").click();
    await page.keyboard.type("before after");

    // Real mouse click placing the caret between "before " and "after".
    await clickBeforeWord(page, "#question-body", "after");

    const fileChooserPromise = page.waitForEvent("filechooser");
    // The real click — this is the mousedown that steals focus from the
    // contenteditable before the (async) upload ever resolves.
    await bodyEditorScope(page).getByRole("button", { name: "Insert image" }).click();
    const fileChooser = await fileChooserPromise;
    await fileChooser.setFiles({
      name: "e2e-fixture.png",
      mimeType: "image/png",
      buffer: Buffer.from(TINY_PNG_BASE64, "base64"),
    });

    await expect(page.locator("#question-body img")).toBeVisible({ timeout: 5000 });
    // Chromium sometimes renders the space adjacent to an inserted inline
    // element back as a literal "&nbsp;" entity in innerHTML, not a plain
    // space — tolerate either so the assertion is about position, not markup.
    const html = (await page.locator("#question-body").innerHTML()).replace(/\s+/g, " ");
    expect(html, "the <img> should sit between \"before\" and \"after\"").toMatch(
      /before(?:\s|&nbsp;)*<img[^>]*>(?:\s|&nbsp;)*after/i
    );
  });

  test("FB-25 — adding a blank row writes the matching {{N}} token into the body", async ({ page, context }) => {
    await mockBackendForFakeSession(page);
    await seedSession(context, { token: "e2e-fake-token", refreshToken: "e2e-fake-refresh", user: FAKE_ADMIN_USER });
    await openCreateDialog(page);

    await page.locator("#question-format").selectOption("multi_blank");
    await page.locator("#question-body").click();
    await page.keyboard.type("Fill in the blanks: ");

    // Default multi_blank starts with 2 rows ({{1}}, {{2}}); this adds the 3rd.
    await page.getByRole("button", { name: "Tambah opsi" }).click();

    const bodyText = await page.locator("#question-body").innerText();
    expect(bodyText, "BlankEditor's {{3}} label is a lie unless the body gets the token too").toContain("{{3}}");
  });

  test("FB-24 — line breaks and paragraphs survive a save (P0, data loss)", async ({ page, context, request }) => {
    await loginRealAdmin(context, request);
    await openRealCreateDialog(page, "essay");

    const marker = `e2efb24${Date.now()}`;
    await page.locator("#question-body").click();
    await page.keyboard.type(`${marker} line one`);
    await page.keyboard.press("Enter");
    await page.keyboard.type(`${marker} line two`);

    const preview = await saveReloadAndOpenPreview(page, marker);
    const previewText = (await preview.innerText()).trim();
    const lines = previewText
      .split(/\n+/)
      .map((l) => l.trim())
      .filter(Boolean);
    expect(lines, "the two typed lines must stay distinct through save + reload").toEqual([
      `${marker} line one`,
      `${marker} line two`,
    ]);
  });

  test("table round trip — insert table, merge cells, save, reload, still a table", async ({ page, context, request }) => {
    await loginRealAdmin(context, request);
    await openRealCreateDialog(page, "essay");

    const marker = `e2etable${Date.now()}`;
    await page.locator("#question-body").click();
    await page.keyboard.type(marker);
    await page.keyboard.press("Enter");

    await page.getByRole("button", { name: "Insert table" }).click();
    // TipTap's table has no <thead> — the header row's <th> cells sit in the
    // same <tbody> as the first row, so row 0 is the header and row 1 is the
    // first <td> row.
    const bodyRow = page.locator("#question-body table tbody tr").nth(1);
    await expect(bodyRow.locator("td")).toHaveCount(3);

    // Real mouse drag from the first data cell into its neighbour — the same
    // native mousedown/mousemove/mouseup mechanism prosemirror-tables needs
    // to build a CellSelection, exactly like selectWordByMouseDrag above.
    const firstCell = bodyRow.locator("td").nth(0);
    const secondCell = bodyRow.locator("td").nth(1);
    const firstBox = await firstCell.boundingBox();
    const secondBox = await secondCell.boundingBox();
    if (!firstBox || !secondBox) throw new Error("table cells not visible");
    await page.mouse.move(firstBox.x + firstBox.width / 2, firstBox.y + firstBox.height / 2);
    await page.mouse.down();
    await page.mouse.move(secondBox.x + secondBox.width / 2, secondBox.y + secondBox.height / 2, { steps: 5 });
    await page.mouse.up();

    await page.getByRole("button", { name: "Merge cells" }).click();
    await expect(bodyRow.locator("td")).toHaveCount(2);
    await expect(bodyRow.locator('td[colspan="2"]')).toHaveCount(1);

    const preview = await saveReloadAndOpenPreview(page, marker);
    const savedRow = preview.locator("table tbody tr").nth(1);
    await expect(
      preview.locator("table"),
      "the table must survive the real backend sanitiser, not just the DOM before save"
    ).toBeVisible();
    await expect(savedRow.locator("td")).toHaveCount(2);
    await expect(
      savedRow.locator('td[colspan="2"]'),
      "the merged cell's colspan must round-trip through save + reload"
    ).toHaveCount(1);
  });

  test("math round trip — an equation survives edit, save and reload with its \\(...\\) delimiters intact", async ({ page, context, request }) => {
    await loginRealAdmin(context, request);
    await openRealCreateDialog(page, "essay");

    const marker = `e2emath${Date.now()}`;
    await page.locator("#question-body").click();
    await page.keyboard.type(`${marker} `);
    // Typing the app's own "\(latex\)" delimiter syntax triggers Task 19's
    // input rule and converts it to a live math node — proves the editing
    // surface itself (Task 19's target) renders KaTeX, not just storage text.
    await page.keyboard.type("\\(x^2\\)");
    await expect(
      page.locator("#question-body .katex"),
      "typing \\(x^2\\) must render live KaTeX in the editing surface (FR-44)"
    ).toBeVisible();

    const preview = await saveReloadAndOpenPreview(page, marker);
    await expect(
      preview.locator(".katex"),
      "the reloaded admin preview must render the saved equation as KaTeX, not literal \\(x^2\\) text"
    ).toBeVisible();
    expect(
      (await preview.innerText()),
      "literal delimiters must not leak into the rendered text once KaTeX has parsed them"
    ).not.toContain("\\(x^2\\)");

    // Re-open the editor on the saved question — this is the actual "survives
    // edit -> save -> reload" proof: the persisted "\(x^2\)" text (getStorageHtml's
    // span-unwrap) must migrate back into a live math node on load (FR-46),
    // not sit there as inert literal text.
    await page.getByRole("button", { name: /action_edit|Edit/i }).click();
    await expect(
      page.locator("#question-body .katex"),
      "reloading into the editor must re-render the persisted equation as live math, proving the \\(...\\) delimiters survived the save intact"
    ).toBeVisible();
  });
});
