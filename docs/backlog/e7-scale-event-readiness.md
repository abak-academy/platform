# E7 — Scale & event readiness

| | |
|---|---|
| **Issue** | [#62](https://github.com/abak-academy/platform/issues/62) |
| **Objective** | A 5000-participant exam event runs on the current VM spec without the API serving every question payload itself — and we know where it breaks before the day. |
| **Source IDs** | NF-4, NF-5 + D-3 (bundle columns) |
| **Client items** | 2 |
| **Depends on** | E1 (F-6 — 5000 registrants means 5000 emails), E2 (the bundle serialises questions) |
| **Verified against** | `main` @ `211b7b1`, 2026-07-29 |

---

## 1. NF-4 — question bundle on GCS + Cloudflare

**This is not new design work.** The TRD already carries the full design of record, and the columns
already exist — D-3 keeps `exam.bundle_url`, `bundle_generated_at` and `cdn_bundle` alive on purpose,
and the handler **explicitly rejects** the fields today
([`exam_package.go:101`](../../backend/internal/handler/exam_package.go)).

This is the implementation of **FR-EXAM-09c**, deferred on 2026-07-07 with the note *"revisit before
the first 10K-CCU competition."* That competition is now scheduled.

The design as specified:

| Element | Spec |
|---|---|
| Key | `/exam-bundles/{exam_id}/{epoch}/bundle.json` — versioned, so a regenerated bundle lives at a new URL and old copies age out by TTL rather than needing a purge |
| Cache header | `Cache-Control: public, max-age=7200` |
| Generation | on `draft → published`; serialise attached Tests + Questions + QuestionOptions |
| Pre-warm | `GET` the URL after upload; warning (non-blocking) if `scheduled_at < now() + 1h` |
| Staleness | editing or attaching/detaching any Test or Question sets `bundle_generated_at = NULL` |
| Fallback | stale or missing → inline payload; correct content always wins over CDN offload |
| Alert | when the fallback fires **inside an active exam window**, raise `notif:admin_exam` + ops log, throttled once per exam per window |

**The alert is the part that matters most here.** A silent fallback at 5000 concurrent defeats the
entire CDN offload and hammers the API — which is exactly the failure a load test should surface,
rather than discovering it live.

**Why it depends on E2.** The bundle serialises questions. E2 changes the question shape — decimal
points, accepted-answer sets, true/false statements. Serialising twice, before and after, is wasted
work and leaves a bundle format that has to be versioned for no reason.

---

## 2. NF-5 — load test to 5000 participants

**The PRD says 10,000.** Both the success metric (*"Exam engine concurrent users (peak load) ≥
10,000"*) and Phase 4 target it. The client's ask is **5000 on current VM spec as phase 1** — not a
contradiction, but a phasing decision that exists nowhere in writing. Record it as part of this epic.

**Needs a target that is not production.** Current infra is 2 VMs in `asia-southeast2-b` with native
PostgreSQL 17.10 behind PgBouncer.

**Blocked on E1 for a reason that is easy to underestimate:** 5000 participants means 5000
registration emails, and the current mailbox caps at 100/day with OTP riding the same channel. Without
SES the event fails at registration, long before any of this matters.

**What the run must produce**, not just "it held":

- p95 recorded against the NFR-PERF-01 target of **300ms**.
- The bottleneck **named** — DB connections, PgBouncer pool, API CPU, bundle origin fetches.
- Confirmation that the bundle path, not the inline path, served the questions.
- The stale-bundle alert firing when deliberately provoked.

---

## Acceptance

- A published exam with `cdn_bundle = true` serves `bundle_url` on session start.
- Editing a question nulls `bundle_generated_at`.
- A stale bundle falls back inline **and** raises the throttled alert.
- A documented load-test run at 5000 concurrent, with p95 against the 300ms target and the bottleneck
  named.
- The 5000-as-phase-1 decision is written down against the PRD's 10,000 metric.

## Out of scope

- Reaching 10,000 CCU. Explicitly phase 2 of this work.
- Hardware changes. The client asked for current VM spec.
- Bundle retention/cleanup job — the TRD calls it optional and the cost is negligible.
