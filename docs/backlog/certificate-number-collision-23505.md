# Certificate number 23505 collision — worker error storm (2026-08-28)

| | |
|---|---|
| **Source** | Grafana Explore (local env), error-level logs across `api|worker|web|nginx` — [short link](http://localhost:3002/goto/ssq8cg) (`{env=~"local", service=~"api|worker|web|nginx"} | json | level=~"(?i)error"`) |
| **Scope of this doc** | **Two issues.** A — the collision itself, **fixed 2026-08-28** (this doc records the RCA + decision); B — the outbox retry storm it exposed, **not yet fixed**, needs its own PR |
| **Verified against** | `main` @ `8686846`; fix written on top (backend/internal/repository/exam.go, exam_certificate_design_test.go) |
| **Status** | A: **fixed and verified end-to-end** on the local stack 2026-08-28 (0 pending outbox events, 0 worker errors). B: designed below, unscheduled |

## What the logs showed

Every error in the window was the same one, all from the **worker** service, repeating every ~5s per
`(event_id, session_id)` pair:

```
generate certificate pdf: allocate certificate number: ERROR: duplicate key value violates
unique constraint "idx_exam_session_certificate_number" (SQLSTATE 23505)
```

Six `session_id`s were looping: `93400aa7…`, `9b131ad1…`, `9ae6b406…`, `47b2ee17…`, `e9ae8428…`,
`be444162…` (event_ids 32–42).

---

## Issue A — one number per registration, allocated per session (fixed)

### Root cause

`AllocateCertificateNumber` composed the certificate number
`ABK/<year>/<exam_number(pad4)>/<participant_number(pad6)>` from the session's **registration**
(`participant_number`, unique per exam) and **exam** (`exam_number`, globally unique). The value is
therefore a per-registration identity — but it was allocated per-**session**:

- `exam_session.certificate_number` carries a global unique index
  `idx_exam_session_certificate_number` (migration 0035).
- The only guard on the composing UPDATE was `WHERE certificate_number IS NULL` — the session's own
  row. Nothing checks whether a **sibling session of the same registration** already took the value.

The live DB proves the shape: registration `ffffffff-ffff-4fff-afff-fffffffffff7` (exam #7,
participant #1) has **13 sessions** — every retake creates a new `exam_session` row on the same
registration. Session `fe40aee7…` legitimately took `ABK/2026/0007/000001`. Every later session
composed the *identical* string, passed its own `IS NULL` guard, and deterministically violated the
unique index → 23505, forever. Retrying cannot help: the collision is a function of the data, not of
timing. (Contributing quirk: `exam.scheduled_at` is NULL for that exam, so the year segment falls
back to `time.Now()` — harmless here, noted because it makes the composed number depend on the clock
when the exam has no schedule.)

### The decision: one number per registration

A certificate number identifies the **participant in that exam**, not an attempt — consistent with
`participant_number`'s per-exam uniqueness (migration 0037). Platform rules already say *"latest
attempt is authoritative, everywhere"* (FB-26/FR22, `dedupedSubmittedSessions`,
backend/internal/repository/exam.go:2354), which answers the natural follow-up: **the certificate a
student sees shows their latest attempt's score/rank**; each session's PDF still renders from its own
session data; older attempts' renders keep their own score under the same number. Rejected
alternative: an attempt-suffixed number segment (new format, backfill decision, changes what the
number means for every consumer) — unnecessary since no code parses the number's shape.

### The fix

`AllocateCertificateNumber` (backend/internal/repository/exam.go) now resolves in order:

1. Session already has a number → return it (unchanged).
2. **New — sibling reuse:** a sibling session of the same `registration_id` already has a number →
   reuse it. Retakes never compose.
3. Compose + guarded UPDATE (unchanged). A concurrent sibling that wins the race (23505) falls back
   to (2) and picks up the winner's committed number.
4. A 23505 **with no sibling number** is surfaced as an error, not masked — only possible via legacy
   3-segment `ABK/YYYY/NNNNNN` rows (migration 0041), i.e. a data problem that must not be hidden.

Tests: `TestAllocateCertificateNumber_RetakeSharesRegistrationNumber` (deterministic reuse) and
`TestAllocateCertificateNumber_ConcurrentRetakesConverge` (two siblings racing → one shared number),
plus the pre-existing `TestAllocateCertificateNumber` suite unchanged.

**Consequence worth knowing:** the global unique index means the number can be *persisted* on only
one row per registration — whichever session composed it first (in the live data that was
`fe40aee7…`, attempt #2, not attempt #1). Sibling sessions keep `certificate_number = NULL` and
reuse is read-only; the number still appears in every sibling's rendered PDF. If anyone ever needs
the column to carry the number on every row, the index must first move to
`(exam_id, certificate_number)`-style scoping or become a non-unique index.

### Remediation

**None required — verified.** After rebuilding and restarting api+worker on the local stack, the 12
pending `CertificateNeeded` outbox events self-healed on the worker poll: allocation succeeded
(sibling reuse), all 13 sessions of the registration rendered and persisted their PDFs, and every
event is now `processed_at` set (61/61 `CertificateNeeded` done, 0 pending, 0 worker errors).

Verification note: during the first poll wave the worker logged a *different* transient error —
`upload certificate pdf: Storage backend has reached its minimum free drive threshold` (MinIO on a
nearly-full Docker VM disk). It cleared within ~20s and all uploads succeeded; if it recurs, reclaim
host disk first (`docker system df` showed ~21GB reclaimable build cache → `docker builder prune`).
That storage-pressure error looping forever would have been issue B's showcase — one more reason to
build the outbox failure path.

---

## Issue B — the outbox has no failure path (open)

The storm's *volume* is issue B. The `outbox` table already carries `attempts` and `last_error`
columns — `ClaimOutboxEvents` even selects them (backend/internal/repository/outbox.go:27) — but
**no code ever writes them**:

- Every handler failure path is log-and-return (e.g. backend/internal/worker/certificate.go:39-42).
- `MarkOutboxProcessed` only sets `processed_at = now()` (backend/internal/repository/outbox.go:51).
- `ClaimOutboxEvents` re-claims every `processed_at IS NULL` row each poll (~5s), `attempts` never
  increments, so a poisoned event is retried forever — the 6 events above looped for days.

This is generic: an `OrderPaid` handler that fails permanently would loop the same way, and the
`attempts`/`last_error` schema shows the failure path was anticipated but never implemented.

**Proposed design (for its own PR):**

1. On handler error, record it: `UPDATE outbox SET attempts = attempts + 1, last_error = $2
   WHERE id = $1`.
2. Cap claims: `WHERE processed_at IS NULL AND attempts < $cap` (cap ≈ 10) — a simple, schema-less
   dead-letter.
3. Optional backoff so transient-but-slow failures don't burn the cap in a minute: a
   `next_retry_at`-style predicate (`COALESCE(next_retry_at, created_at) <= now()`), set to
   `now() + jitter*attempts` on failure.
4. Observability: a Grafana alert on `SELECT count(*) FROM outbox WHERE processed_at IS NULL AND
   attempts >= $cap` — that query is the dead-letter queue.
5. Open question: whether past-cap events get an admin nudge endpoint (re-arm attempts = 0 after a
   fix ships) or are handled by re-inserting a fresh event.

**Why not bundled with the fix:** the collision fix is one pure function; the outbox guard touches
claim semantics for *every* event type and deserves its own review + alert wiring.
