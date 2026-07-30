# E2 Part C — editor swap: consumer inventory and Playwright rewrite plan

**Library: TipTap.** Decided 2026-07-31 — see [decision-editor-library.md](../../.claude/zone-v2/decision-editor-library.md).
The decisive fact: `question.body` stores HTML, and TipTap emits HTML natively, so no data
migration is required. Lexical was rejected because its own docs call HTML serialization lossy;
Slate was rejected for having no official table support.

Two constraints that choice carries, both load-bearing for this plan:

- **KaTeX needs no editor extension.** `RichContent.tsx` renders math as a post-render DOM pass
  (`renderMathInElement` from `katex/contrib/auto-render`) over plain `\(…\)` text already inside
  the sanitized HTML. Any HTML-emitting editor — TipTap included — passes that text through
  untouched, so the swap does not need `@tiptap/extension-mathematics` to satisfy FR-38/FR-43.
  Live in-editor math rendering (FR-44) is a separate, optional enhancement, not a requirement of
  the swap itself.
- **Merged cells are gated on the Task 16 spike.** `decision-editor-library.md` records a live
  rough edge in TipTap's table merge (ueberdosis/tiptap discussion #7456). No consumer below may be
  touched with a table UI until Task 16 proves `mergeCells`/`splitCell` round-trip through the real
  backend sanitiser (FR-42).

## The two escape hatches — hard requirements on any replacement

`RichTextEditor.tsx` exposes an imperative handle (`RichTextEditorHandle`, defined lines 38–41)
with two methods, both required capabilities on whatever replaces the contentEditable div:

- **`insertTextAtCaret(text: string)`** — inserts text at the current caret position without
  disturbing surrounding content.
- **`setContent(html: string)`** — replaces the editor's full HTML content and re-syncs the
  `value`/`onChange` contract.

Both exist because of one caller: `QuestionEditor.tsx`'s multi-blank token bookkeeping.

- `insertBlankToken(index)` (QuestionEditor.tsx:463–465) calls `insertTextAtCaret({{${index}}})`
  when `BlankEditor.add()` appends a new blank row — this is FB-25's fix: the `{{N}}` label
  BlankEditor shows must actually land in the body, at the caret, not appended blindly to the end.
- `removeBlankToken(removedIndex)` (QuestionEditor.tsx:467–469) calls
  `setContent(renumberTokensInBody(body, removedIndex))` when a blank row is deleted, so every
  token above the removed one shifts down by one and the body stays a contiguous `{{1}}..{{N}}` set
  in lockstep with `BlankEditor.remove()`'s row renumbering.

A TipTap-based `RichTextEditor` must keep exposing `insertTextAtCaret`/`setContent` (or a shim with
identical behavior) via `useImperativeHandle`, because `QuestionEditor.tsx` calls them directly —
changing that contract means also changing `QuestionEditor.tsx`, which is out of scope for the
editor swap itself.

## The HTML-storage constraint

`question.body` and `option.text` are stored and read back as HTML strings, sanitized server-side
by a bluemonday policy (`backend/internal/service/exam.go`) and mirrored client-side by
`QUESTION_BODY_ALLOWED_TAGS` (`web/lib/question-html.ts`). A JSON-state editor (Lexical's native
format, or a hand-rolled Slate document) would force one of: (a) migrating every existing question
body to JSON, changing the sanitiser's input and the renderer, or (b) a lossy HTML round-trip on
every save — which is FB-24 rebuilt on purpose. TipTap sidesteps this because `getHTML()`/`setContent(html)`
operate on HTML directly; no migration is needed.

## Consumer inventory — seven files (FR-39)

| # | File | Role | Imports (editor-relevant) | What a library swap demands |
|---|---|---|---|---|
| 1 | `web/components/admin/RichTextEditor.tsx` | The editor itself — contentEditable div, toolbar (Bold/Italic/Underline/lists/superscript/subscript/formula/image), paste sanitisation, imperative handle. | `DOMPurify`, `QUESTION_BODY_ALLOWED_TAGS` from `@/lib/question-html`, `usePresignAdminImageUpload` | Full rewrite. Must keep the same prop contract (`value`, `onChange`, `disabled`, `aria-label`, `id`, `compact`), the same imperative handle (`insertTextAtCaret`, `setContent`), the same toolbar actions with the same `aria-label`s consumed by tests, and paste-sanitisation against the same allowlist. `document.execCommand("defaultParagraphSeparator", ...)` (line 75) stays until the replacement is proven not to emit `<div>` on Enter — FR-40. |
| 2 | `web/components/admin/QuestionEditor.tsx` | Question authoring dialog. Owns the `body` state, the ref to `RichTextEditorHandle`, and the multi-blank token bookkeeping (`insertBlankToken`/`removeBlankToken`) that depends on the two escape hatches. | `RichTextEditor`, `RichTextEditorHandle` from `@/components/admin/RichTextEditor` | No functional change if `RichTextEditor`'s prop/handle contract is preserved — this file calls `bodyEditorRef.current?.insertTextAtCaret(...)` and `.setContent(...)` directly (lines 464, 468) and would break the moment those methods disappear or change signature. |
| 3 | `web/components/admin/RichContent.tsx` | Read-only renderer for stored question/option HTML. Sanitises with the same allowlist, then runs KaTeX's `renderMathInElement` as a post-render pass. | `DOMPurify`, `renderMathInElement` from `katex/contrib/auto-render`, `QUESTION_BODY_ALLOWED_TAGS` | No change required by the editor swap — it renders HTML regardless of which editor produced it. Table markup (`<table>`/`<thead>`/`<tbody>`/`<tr>`/`<td>`/`<th>`, `colspan`/`rowspan`) must pass through its `ALLOWED_TAGS`/`ALLOWED_ATTR` once FR-36 widens the allowlist (Task 16+), independent of TipTap. |
| 4 | `web/components/admin/QuestionPreview.tsx` | Read-only preview dialog for a bank question, before edit. Renders question body and each option's text via `RichContent`. | `RichContent` | No change required — consumes `RichContent`, not the editor. |
| 5 | `web/app/(exam-session)/exam/sessions/[id]/page.tsx` | Student-facing exam-taking page. Renders question body and option text via `RichContent`, with its own local `sanitizeForRichContent` helper (lines 735–741) duplicating `RichContent`'s allowlist for the multi-blank inline-input path. | `RichContent`, `QUESTION_BODY_ALLOWED_TAGS`, `DOMPurify` | No editor-library change required — read-only consumer. This is the bundle-size-sensitive path flagged in `decision-editor-library.md` (student session bundle should not carry the editor's ProseMirror weight); confirm the editor chunk is not imported here, only `RichContent`. |
| 6 | `web/app/(student)/exam/sessions/[id]/result/page.tsx` | Student result/pembahasan page. Renders each reviewed question's body via `RichContent`. | `RichContent` | No change required — read-only consumer. |
| 7 | `web/app/(admin)/admin/school/reports/page.tsx` | Admin school-reports page. Renders a question body (`p.body`) via `RichContent` in some report-detail view. | `RichContent` | No change required — read-only consumer. |

Five of the seven consume only the renderer (`RichContent`), which is library-agnostic and needs
no changes for the swap itself (table-allowlist widening is a separate, parallel FR-36 concern).
Only two consume the editor directly: `RichTextEditor.tsx` (the rewrite target) and
`QuestionEditor.tsx` (the one caller of its imperative handle).

## Playwright rewrite plan — `web/e2e/question-editor.spec.ts`

All four current cases scope the toolbar via `bodyEditorScope()` (line 113–115):
`page.locator("#question-body").locator("xpath=ancestor::div[2]")` — this walks up from the
contentEditable div two DOM levels to its toolbar-sharing wrapper, and every toolbar button is
found via `getByRole("button", { name: <aria-label> })` using the current `aria-label`s ("Bold",
"Insert image", etc). Both are coupled to `RichTextEditor.tsx`'s current DOM shape: the toolbar and
the contenteditable as adjacent siblings, `#question-body` as the contenteditable's own id.

| Case | What it actually proves | Selector coupling to current toolbar | Replacement strategy under TipTap | What counts as losing coverage |
|---|---|---|---|---|
| **FB-22** — Bold applies to the selection, not the whole field | `document.execCommand("bold")` on a range selection made via `restoreSelection()` bolds only the selected text, not the whole contentEditable (the caret/selection-loss bug class). | `bodyEditorScope(page).getByRole("button", { name: "Bold" })`; assumes `<b>`/`<strong>` markup. | Keep the same real-mouse-drag selection (`selectWordByMouseDrag`, still valid against any contentEditable-backed ProseMirror view) and click TipTap's own Bold button (update `aria-label`/toolbar locator to whatever the TipTap toolbar implementation uses — same button-by-accessible-name approach, new selector string). Assert only the selected text carries TipTap's bold mark in `getHTML()`, not the whole body. | Losing coverage means: bolding a selection under the new editor mutates the *entire* field's content instead of just the selection, and no test catches it — i.e. the assertion stops checking that the surrounding text (`hello`) is excluded from the bold markup. |
| **FB-23** — an inserted image lands at the caret, not the end | A file-upload-triggered `insertHTML` places the `<img>` between the caret position ("before"/"after") rather than appending it after focus is stolen by the native file-chooser dialog. | `bodyEditorScope(page).getByRole("button", { name: "Insert image" })`; asserts `<img>` position via `innerHTML` regex. | Same real-click + `filechooser` event flow (TipTap's own image insert command, whatever UI it's bound to) with the caret placed via `clickBeforeWord`. Assert image position via `editor.getHTML()` (or TipTap's exposed content string) matching `before...<img...>...after`, same as today. | Losing coverage means: the test no longer proves position relative to caret — e.g. it degrades to "an image appears somewhere in the body," which would pass even if TipTap always appended to the end. |
| **FB-25** — adding a blank row writes the matching `{{N}}` token into the body | `BlankEditor.add()` calling `insertBlankToken` actually calls the editor's `insertTextAtCaret`, so the `{{3}}` label BlankEditor shows is backed by a real token in the body — not just UI state drift. | Not toolbar-coupled — locates `#question-body`'s `innerText` and a bank-question-editor "Tambah opsi" button by name. Low selector risk from the library swap itself, but depends on `insertTextAtCaret` continuing to exist on the ref (see escape-hatches section above). | No selector rewrite needed if `RichTextEditorHandle.insertTextAtCaret` is preserved (it must be, per FR-39's naming above). Assert `{{3}}` appears in the TipTap document's plain-text/HTML output. | Losing coverage means: `QuestionEditor` stops calling `insertTextAtCaret` (e.g. a bespoke TipTap-specific insertion path is added instead) and this test is only updated to look for `{{3}}` without proving it lands *in the body*, not just in `BlankEditor`'s own row label. |
| **FB-24** — line breaks and paragraphs survive a save (P0, data loss) | The real backend sanitiser (`sanitizeQuestionBody`, `backend/internal/service/exam.go`) does not strip `<p>`/`<br>` on save+reload — the actual data-loss bug, provable only through a genuine save/reload/reparse round trip against the live API. | Not toolbar-selector-coupled at all — its risk is behavioral: it depends on the editor emitting `<p>` (not `<div>`) on Enter, which is FR-40's guard (`defaultParagraphSeparator`). | Must **keep hitting the real backend** exactly as today — real login, real save via `useSaveQuestion`, real page reload, real re-fetch of the saved question, assert on the re-rendered `[data-rich-content]` text. The only thing that may change is confirming TipTap's Enter-key behavior emits `<p>` elements (ProseMirror's default paragraph node), so the existing keystrokes (`Enter` between two typed lines) keep exercising the same code path. | Losing coverage means: faking the save (mocking the API response instead of a real POST+reload) or asserting only on the DOM immediately after typing, without a save+reload — exactly the trap `decision-editor-library.md` and FR-42 call out ("an assertion against the DOM immediately after insert does not satisfy this; that is how FB-24 shipped green"). FB-24 must stay a real-backend test or it stops proving anything. |

### Net rewrite scope

- Toolbar-button locators in all three UI-driven cases (FB-22/23/25) need updating to whatever
  accessible names TipTap's toolbar implementation uses — this is expected churn, not a coverage
  risk, as long as the *assertions* (selection-scoped bolding, caret-relative image position,
  token-in-body) are preserved verbatim.
- `bodyEditorScope`'s `xpath=ancestor::div[2]` DOM-depth assumption will very likely need
  re-deriving once the TipTap wrapper's DOM shape is known (Task 16 spike) — flagged here as an
  unknown, not solved by this document.
- FB-24 is the one case with a hard, non-negotiable constraint: it must keep exercising the real
  Go sanitiser via a real save + reload. No mocking of `/api/v1` calls for that test, ever.
