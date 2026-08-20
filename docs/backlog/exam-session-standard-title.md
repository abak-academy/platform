# Standard exam top bar keeps the first test’s title

| | |
|---|---|
| **Status** | Fixed — standard-mode heading follows `currentQ.test_id` |
| **Date** | 2026-08-20 |
| **Surface** | Student exam overlay (`GET /api/v1/exam/sessions/:id`, standard / overall timer) |
| **Not the bug** | `test_question` joining a question onto the wrong test; UTBK/IELTS section heading |

On a **standard** paper with more than one attached test, the top-left heading stayed on **test A’s title** for every question. Question 33 of 50 could be a Matematika item while the bar still showed e.g. `TKA BAHASA INDONESIA SD/MI`.

---

## What the student sees

The heading is the large label in the exam top bar (circled in the live screenshot: package/test name next to answered count, Saved, timer, Submit). It is **not** a per-question field. Questions themselves have `body` only.

---

## Root cause

The reconnect payload is grouped correctly. `ReconnectSession` → `GetSessionWithQuestions` → `ListQuestions` (`question` JOIN `test_question` WHERE `test_id`) → `groupQuestionsByTest` copies `test.title` / `test.subject` onto each `tests[]` object and sets each question’s `test_id` to that test.

JSON shape:

```
tests[0].title = "TKA BAHASA INDONESIA SD/MI"
tests[0].questions[].test_id = tests[0].id
tests[1].title = "Tes Matematika"
tests[1].questions[].test_id = tests[1].id
```

The overlay then:

1. Flattens **all** tests: `session.tests.flatMap((t) => t.questions)`
2. Used **`session.tests[0].title`** as the only heading

That was documented in code as a stand-in because `SessionState` has no `exam_title`. Session check-in already returns `exam_title`; reconnect does not. Using the first test’s name is wrong as soon as a second test is attached.

UTBK/IELTS is fine: mode label + `activeTest.title`, and only the active section’s questions are shown.

---

## What is *not* wrong

- Bank questions leaking across tests in SQL.
- Needing to copy `subject` onto each `SessionQuestion`.
- The number grid mixing IDs; cells are the flattened list in exam-test order.

If a **single** test is titled Bahasa but math items were attached to it, that is authoring data, not this UI bug.

---

## Solution (shipped)

Resolve the heading from the **current question’s test**:

```ts
session.tests.find((t) => t.id === currentQ?.test_id)?.title
  ?? session.tests[0]?.title
  ?? ""
```

Sectioned mode unchanged (UTBK/IELTS i18n label + active section subtitle).

No backend or schema change. Optional later: add `exam_title` on reconnect and show package name + current test as subtitle.

---

## Acceptance

- One-test standard exam: top bar still shows that test’s title.
- Two-test standard exam: question 1 shows test A’s title; jumping to the first test B question shows test B’s title.
- UTBK/IELTS: still mode label + active section title.
