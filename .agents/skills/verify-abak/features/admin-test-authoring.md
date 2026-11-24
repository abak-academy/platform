# Admin test authoring

Exam admins create and edit tests at `/admin/exam/tests` (page title `Penyusun Tes` / `Test authoring`), open one at `/admin/exam/tests/[id]` to add questions, and bundle tests into packages at `/admin/exam/packages`.

## Sub-features

- `tests-create` creates a test from the `Tes baru` / `New test` modal and opens its detail page.
- `tests-search` filters the list through the toolbar search.
- `tests-delete` deletes a test after confirmation.
- `questions-edit` authors questions in the question editor (`/admin/exam/questions`, test detail).

## How to get to it (user POV)

- Sign in at `/admin/login` (super_admin or admin_exam lands in `/admin/exam`), then open the test-authoring nav item.
- Or open `/admin/exam/tests` directly.

## Driving it with abak-verify

Preconditions:

- `abak-verify doctor` prints `OK`, signed in as `verify-admin@example.test`.

- **Create.** Press `Tes baru` / `New test`, fill the modal, and save. Expect the toast `Tes dibuat` / `Test created` and a URL of `/admin/exam/tests/<id>`. Second view: the list at `/admin/exam/tests` shows the new name.
- **Search.** Type into `data-testid="tests-search-input"` (placeholder `Cari tes…`). Only matching rows remain.
- **Delete.** Delete the row and confirm. Expect a success toast and the row gone after reload.

## Gotchas

- The toolbar is `data-testid="exam-tests-toolbar"`. Existing `web/e2e/exam-tests-toolbar.spec.ts` and `test-authoring-flow.spec.ts` show working selectors, so read them before writing a journey.
- The question editor relies on real `document.execCommand` and selection. Only a real browser proves it (see `web/e2e/question-editor.spec.ts`).
- `getByRole("button", { expanded: false })` also matches the Next.js Dev Tools button. Name the button.
