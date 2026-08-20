# `max_attempts` NULL means unlimited retake

| | |
|---|---|
| **Status** | In progress |
| **Date** | 2026-08-20 |
| **Surface** | `POST /api/v1/exam/sessions`, student `/exam` card, admin exam package create/edit |
| **Not this change** | Continue button on the student card; paid vs free commercial policy; backfilling live rows to `1` |

Admin overview already labelled a blank **Maks. Percobaan** as unlimited. The engine (FR18) treated `NULL` and `0` as **one sitting**. This change makes the label true: **`NULL` is unlimited retakes**. There is **no data backfill** — every existing exam with `max_attempts` NULL becomes unlimited when this ships.

---

## What the student and admin see

- **Admin create/edit:** leave Max Attempts empty → `null` → unlimited. `0` or `1` → cannot retake. `2+` → that many sittings.
- **Admin overview:** `null` → Unlimited / Tidak terbatas. `0` and `1` → cannot retake (1).
- **Student `/exam` after submit:** Retake stays when `max_attempts` is `null`. Hidden when `0` or `1` and `attempts_used >= 1`.

---

## Registration model

Access is **one `exam_registration` per student per exam** (checkout, free, or admin grant). Retakes do not buy or grant again. Each start after a **submitted** sitting inserts another `exam_session` on that row and increments `attempts_used`.

Schedule `scheduled_at` / `scheduled_end_at` still gates Start when a window is set, including unlimited exams.

---

## Old FR18 vs now

| `max_attempts` | Before (FR18) | Now |
|---|---|---|
| `NULL` | One sitting | Unlimited new sittings after submit |
| `0` | One sitting | One sitting (cannot retake) |
| `1` | One sitting | One sitting (cannot retake) |
| `>= 2` | That ceiling | Unchanged |

`CreateExamSessionTx` used `attempts_used < COALESCE(NULLIF($2, 0), 1)`, so NULL collapsed to 1. The predicate now skips the ceiling when `$2` is NULL, and still uses ceiling 1 for `0`. The one-live-session lock is `SELECT … FOR UPDATE` on the registration row, then a count of `in_progress` sessions, then the ceiling UPDATE.

---

## Resume if in progress

Start must **not** mint a second live session. Closing the tab and clicking Start again returns the existing `in_progress` session (same `session_id`, remaining time from `started_at`). A new row is created only when there is no live session **and** the ceiling (if any) still has room.

Without that, unlimited NULL would allow unbounded parallel `in_progress` sessions on one registration.

---

## Open: which submitted attempt is canonical

The student list join is `ORDER BY attempt_number DESC` (latest). Result, leaderboard, and certificate have not been given a “best score vs latest” rule; they keep current “latest / whatever the query already returns” behaviour until that is specified.

---

## Acceptance

- Exam with `max_attempts` NULL: submit, Start/Retake again → new session, `attempts_used` increments.
- Exam with `max_attempts` `0` or `1`: second Start after submit → 409 `already_attempted`.
- Exam with `max_attempts` 2: two submitted sittings, third Start → 409.
- Start twice with no submit → same `session_id`; only one `in_progress` row.
- Admin empty Max Attempts field still saves `null`; overview shows unlimited.
- Admin `0` overview is not a bare “0”; it reads as cannot retake.
- Existing NULL packages are **not** migrated to `1`.
