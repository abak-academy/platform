# E2 — Exam authoring

| | |
|---|---|
| **Issue** | [#61](https://github.com/abak-academy/platform/issues/61) |
| **Objective** | An admin can write, find, correct and score questions the way the client actually works — including question types and answer shapes the engine cannot express today. |
| **Source IDs** | FB-1, FB-3, FB-4, FB-5, FB-6, FB-7, FB-9, FB-10, FB-10a + **FB-21** (Part C, added 2026-07-30) |
| **Unscheduled, same surface** | FB-21, FB-22, FB-23, **FB-24 (P0)**, FB-25 — [H1](h1-live-bugs-2026-07-30.md). **FB-3 was re-reported on 2026-07-30 as FB-30.** |
| **Client items** | 9 |
| **Depends on** | E3 (results tab, for FB-10a) — hard · E1 (D-1) — **soft**: regrade compiles without the storage seam, you just lose any reason to trust its tests |
| **Blocks** | E7 — **soft**: the bundle serialises questions, so serialising twice is waste, not breakage |
| **Verified against** | `main` @ `211b7b1`, 2026-07-29 |

Two halves that share one migration: **list ergonomics** (find, delete, import, protect) and the
**answer model** (decimal points, multiple accepted answers, true/false). They were separate sessions
until it became clear both add columns to `question` — doing them apart means two migrations over the
same table.

---

## Part A — List ergonomics

### FB-5 + FB-7 — ordering and a readable id *(one change)*

The `question` table has **no `created_at`**
([`0014_exam.up.sql:16-27`](../../backend/db/migrations/0014_exam.up.sql)) and the list orders by
`q.id` ([`exam.go:574`](../../backend/internal/repository/exam.go)) — a UUID, so the order is
effectively arbitrary. The UI shows `question.id.slice(0, 8)`.

The client asked for two things that turn out to be one: newest-first (FB-5) and a short numeric id in
the list (FB-7). A single monotonic `question_number` satisfies both — sort on it descending, display
it. The precedent is already in the tree: `exam_number` and `participant_number`, migrations 0037 and
0039.

### FB-3 — remove a question from the bank list

The backend already supports it — [`exam.go:439-465`](../../backend/internal/repository/exam.go)
deletes options, then blanks, then the question. Only the list UI lacks the action.

Needs a confirm step and a guard: refuse, with a clear reason, when the question belongs to a test
attached to a published exam.

### FB-4 — download a template before bulk upload

The importer exists ([`exam_import.go`](../../backend/internal/service/exam_import.go)) with required
headers `format, body, subject, topic, point_correct, point_wrong` and optional
`difficulty, correct_answer, option_*`. Nothing hands the admin a template.

**Generate it from the same header list the parser uses**, so the two cannot drift. The pattern to
copy is [`BulkImportModal.tsx:26-42`](../../web/components/admin/BulkImportModal.tsx) — header
constant, example row, client-side download, no network call.

### FB-9 — edit a question already used in an exam, but never change its type

Guard `format` changes when the question is attached to a live exam. Everything else stays editable.

---

## Part B — Answer model

### FB-1 — points can be decimal

Today [`0015_exam_scoring.up.sql`](../../backend/db/migrations/0015_exam_scoring.up.sql):

```sql
point_correct INT NOT NULL DEFAULT 1 CHECK (point_correct >= 1)
point_wrong   INT NOT NULL DEFAULT 0 CHECK (point_wrong >= 0)
```

Widening to `NUMERIC` is not enough on its own. **`CHECK (point_correct >= 1)` rejects `0.5`** — the
constraint has to move or decimals below 1 remain impossible, which is most of the point.

**Decided 2026-07-30 — the new bound is `> 0`:**

```sql
CHECK (point_correct > 0)
```

Every question is always worth something. `0.1`, `0.2`, `0.25` are allowed; **zero is not**, which
preserves the intent of the original constraint. Zero-point questions were never asked for and are not
in scope.

**Do not miss the twin guard in the service layer** — `exam.go:275` has `if q.PointCorrect < 1`.
Changing only the migration leaves decimals rejected before they ever reach the database.

`point_wrong` already allows zero (`>= 0`) and keeps that bound.

### FB-10 — more than one correct answer accepted

`question.correct_answer` is a single `TEXT`, and `question_blank` stores one `correct_answer` per
blank ([`0028_multi_blank_question_audio.up.sql`](../../backend/db/migrations/0028_multi_blank_question_audio.up.sql)).
The client's example — `1+1` accepting both `2` and `dua` — cannot be expressed.

Needs an accepted-answers set per question *and* per blank, plus **matching rules the client agrees
to**. Trimming and case are obvious. Accents, punctuation and number-word equivalence are not — decide
explicitly and write the rule into this doc before implementing, because it is the kind of thing that
silently diverges between the grader and the admin's expectation.

Applies to `short`, `fill_blank` and `multi_blank`.

### FB-6 — true/false with several statements per question

A seventh format. Each statement is independently true or false and independently scored, which makes
it structurally closer to `multi_blank` than to `mcq` — a child table of statements, not options.

Current formats: `mcq, multi_answer, short, fill_blank, essay, multi_blank`. Note `multi_blank`
shipped without ever reaching the PRD — see [PRD/TRD drift](prd-trd-drift.md) §2.

### FB-10a — edit the correct answer and recalculate before publishing

A regrade command over an exam's submitted sessions, reachable from the results tab **E3** builds, and
gated so it cannot run after results are released.

**This is the item that most needs E1's storage seam.** It changes scores; its tests must run against
shipping code, not a shim copy.

---

## Part C — The editor itself (FB-21: tables in a question body)

> **Added 2026-07-30.** FB-21 arrived worded *"jika memungkinkan"*, so H1 recorded it as the batch's one
> soft ask. **The client confirmed on 2026-07-30 that it is a real need**, which turns it from a nice-to-have
> into the item that forces a decision Part A and Part B could both avoid.

**The current editor cannot do this, and it is not a matter of effort.**
`web/components/admin/RichTextEditor.tsx` (332 lines) is built on `document.execCommand`, which has **no
table command at all**. The only way to produce a `<table>` is to inject raw HTML — after which nothing
manages it: adding a row, deleting a column or merging cells all become hand-written DOM manipulation.
There is no document model to keep the structure valid.

`execCommand` is also deprecated in every browser, and that is not theoretical here: **four of this
batch's fifteen items came out of it** — FB-22, FB-23, FB-24 and FB-25 — including FB-24, a P0 that
destroyed data on every save.

So Part C is a **replacement**, not an extension.

### Library choice — OPEN

**The user is evaluating options as of 2026-07-30. Do not start implementation until it is chosen.**
Recorded criteria, in the order they matter for this codebase:

| Criterion | Why it matters here |
|---|---|
| **Official table support** | rows, columns, merged cells — the actual requirement |
| **Emits HTML** | `question.body` stores HTML today. An editor with its own JSON state changes the storage contract and forces a data migration on existing questions |
| **Headless / unstyled** | the admin shell has its own design system; an opinionated theme fights it |
| **Schema-validated document** | a real node schema makes invalid markup impossible to author, which is stronger than sanitising after the fact |
| **KaTeX coexistence** | `katex@0.17.0` is already a dependency and math rendering must survive |
| **React 19 / Next 15 support** | the app's current versions |

Candidates surveyed but **not chosen**: TipTap (ProseMirror), Lexical, Slate. TipTap scored best on
*emits HTML* + *headless*; Lexical's table support was judged less mature and its JSON-first state would
change the storage contract. **This is a survey note, not a decision.**

### What changes regardless of which library wins

1. **The server allowlist must be widened in lockstep — this is the FB-24 trap.** `table`, `thead`,
   `tbody`, `tr`, `td`, `th` plus `colspan`/`rowspan` must be added to **both**
   `web/lib/question-html.ts` **and** the bluemonday policy in `backend/internal/service/exam.go`.
   Widening only the frontend means every authored table is **silently stripped on save** — exactly
   FB-24 repeated. The cross-language test added in `f278364` goes red if the two lists diverge, so this
   failure is now caught, but only if nobody deletes that test.
2. **Seven files** consume the editor or its renderer, including the student exam-session page and the
   result page — this is not an admin-only change.
3. **`defaultParagraphSeparator`** (`RichTextEditor.tsx`, added in `f278364`) is half of the FB-24 P0
   fix and exists only because `execCommand` emits `<div>` on Enter. A replacement editor should make it
   unnecessary — verify that before deleting it.
4. **The Playwright suite** (`web/e2e/question-editor.spec.ts`) is selector-coupled to the current
   toolbar and will need rewriting. Its four cases are the regression guard for FB-22/23/24/25; keep
   them green through the swap.
5. **Existing questions are stored as HTML** and must keep rendering. Any candidate that parses HTML on
   load avoids a data migration entirely.

### Done when

- An admin can insert a table, add and remove rows and columns, and save — and it **survives a reload**,
  proving it passed server-side sanitisation rather than being stripped.
- The cross-language allowlist test passes with the table tags present on both sides.
- Math (KaTeX) still renders in the editor, the preview, and the student exam view.
- All four Playwright cases still pass.
- No migration was needed for existing question bodies, or one was written and proven.

---

## Acceptance

- A 2.5-point question scores 2.5 end to end and displays correctly on the result page.
- A short-answer question accepts every configured answer and rejects the rest, with the agreed
  matching rule tested at its boundary.
- A true/false question with 4 statements scores partially and renders as one question to the student.
- The bank lists newest-first with a short numeric id, stable across reloads.
- Deleting an unused question works; deleting one inside a published exam is refused with a reason.
- The downloaded template imports without edits.
- Editing a used question succeeds; changing its type is refused.
- Changing a correct answer and regrading updates score and rank for every affected session, is
  refused once results are released, and writes an audit row.

## Out of scope

> **Changed 2026-07-30:** replacing the rich-text editor used to be implicitly out of scope — H1 and the
> zone spec both told Task 13 *not* to replace `execCommand`, because pulling FB-21 forward was a bigger
> decision than a bugfix batch could authorise. The client has since confirmed tables are a real need, so
> that replacement is now **Part C of this epic**. What remains out of scope is starting it before the
> library is chosen.


- Question randomisation (`Exam.randomize` is stored but shuffling was deferred 2026-07-07).
- Reusing a question across tests — `question.test_id` is `NOT NULL` by design (FR-EXAM-03).

## Resolved

**The lower bound on `point_correct` is `> 0`** *(2026-07-30)*. Decided internally — it is a forced
consequence of FB-1, not a product choice, since `>= 1` would reject every decimal below 1. Zero-point
questions stay impossible; nobody asked for them.

*An earlier draft raised "should zero be allowed?" as a question for the client. It should not have
been — it was an observation made while reading the migration, turned into scope nobody requested.*

## Open questions for the client

1. FB-10 matching: are accents, punctuation and number-word equivalence (`2` / `dua` / `two`) in or
   out? **Proposed answer, needs only a nod:** none of them. The client's own example (`2`, `dua`) is
   a *list of accepted answers*, so if the admin writes both, the grader needs no language knowledge.
   Rule stays minimal and deterministic — trim, case-fold, collapse internal whitespace, then exact
   match against each listed answer. Anything else is another accepted answer, not another rule.
