# E7 — Scale & event readiness

| | |
|---|---|
| **Issue** | [#62](https://github.com/abak-academy/platform/issues/62) |
| **Objective** | A 5000-participant exam event runs on the current VM spec without the API serving every question payload itself — and we know where it breaks before the day. |
| **Source IDs** | NF-4, NF-5, **F-6** + D-3 (bundle columns) |
| **Client items** | 2 |
| **Depends on** | E2 — **soft**: the bundle serialises questions, so doing this first means serialising twice. Not a compile block; E7 can start today |
| **Spec conflict** | NF-5 contradicts the PRD's 10,000 CCU metric; PRD v1.2 records the 5000-CCU phase-1 decision |
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

### Bcrypt verify cost — measured (login is a candidate bottleneck)

Command:

```
cd backend && go test -run '^$' -bench 'BenchmarkPasswordVerify_ProductionCost' -benchtime 20x ./internal/service/
```

Verbatim output, **cost 10 — the shipped value** (2026-08-14):

```
goos: darwin
goarch: arm64
pkg: akademi-bimbel/internal/service
cpu: Apple M3 Pro
BenchmarkPasswordVerify_ProductionCost-11    	      20	  52659723 ns/op
PASS
ok  	akademi-bimbel/internal/service	2.060s
```

Verbatim output, **cost 12 — historical, superseded by #99** (2026-08-13). Kept because every
capacity number written before 2026-08-14 derives from it:

```
goos: darwin
goarch: arm64
pkg: akademi-bimbel/internal/service
cpu: Apple M3 Pro
BenchmarkPasswordVerify_ProductionCost-11    	      20	 234357979 ns/op
PASS
ok  	akademi-bimbel/internal/service	6.180s
```

The benchmark reads `productionBcryptCost` rather than a literal, so it always measures whatever is
shipped — the two runs above differ only by the constant. Measured ratio 4.45x, against the 4x that
two cost steps predict.

**This was measured on a developer Mac (Apple M3 Pro), not on `e2-standard-2`.** The number above is
not attributable to production hardware and must not be used for capacity planning as-is. Re-run the
identical command on `e2-standard-2` before relying on it:

```
cd backend && go test -run '^$' -bench 'BenchmarkPasswordVerify_ProductionCost' -benchtime 20x ./internal/service/
```

**Derived arithmetic (from the Mac numbers above, for scale only).** At the shipped cost 10,
52,659,723 ns/op ≈ 0.053s/op → ~19 verifies/sec on one M3 Pro core. Assuming `e2-standard-2` is
roughly 2x slower — **an assumption, not a measurement** — that is ~105ms/op, ~9.5/sec per core,
~19/sec across 2 vCPU fully dedicated, and ~12/sec once the five other containers get their share.

| Students | cost 12 (before #99) | cost 10 (shipped) |
|---|---|---|
| 1,000 | ~6 min | ~1.5 min |
| 3,000 | ~18 min | ~4 min |
| 5,000 | ~30 min | ~7 min |

Because exam registration is manual via `super_admin`, there is no natural spread: every student logs
in inside the same minute. Cost 10 shrinks that window, it does not remove it — staged login remains
worth doing, but it is now a convenience rather than a requirement.

**Cost 12 was not retained — it was lowered to 10 in #99 (2026-08-14).** The original text here said
"Cost 12 is retained", which was true when #96 wrote it and false one day later.

The decision, in short: 12 was never chosen. It was hard-coded in the first auth commit (`09e5793`,
2026-06-12) with no rationale recorded anywhere, and every later change to it moved it for CI reasons.
Every password this code hashes is machine-generated — `genTempPassword` draws 10 runes from a 62-rune
alphabet via `crypto/rand`, ~59 bits — and no work factor meaningfully protects or fails to protect
that. Existing hashes keep their own cost, since `CompareHashAndPassword` reads it from the hash;
there is deliberately **no** rehash-on-login, which here would silently downgrade admin hashes.
`TestProductionBcryptCostIsNotWeakened` still guards the floor, now at 10, and carries the condition
that would justify raising it again: students setting their own passwords at scale, which retires the
entropy argument.

---

## 3. F-6 — Amazon SES *(moved here from E1 on 2026-07-30)*

The platform stays on the existing Hostinger SMTP for now. The migration is **deferred, not
cancelled**, and it moved into this epic because this is the epic that forces it.

**The constraint did not move with it.** The mailbox caps at **100 emails/day** and OTP rides the same
SMTP channel, so registration dies at email 101 — no OTP, no account. That ceiling binds long before
5,000: any intake past ~100 sign-ups in a day hits it, and a single broadcast through
`sendAnnouncement` (one email per recipient, no batching, no rate limit) hits it on its own.

**It cannot be done in the last week before an event.** The AWS sandbox review is the one step whose
timing is out of our hands — a new SES account may only send to verified addresses at ~200/day until
approved. Everything else is quick: verification and DKIM are three CNAMEs, and the cutover is a
config swap, since `adapter/smtp.go` is plain SMTP and SES speaks SMTP.

Step-by-step runbook, gotchas and rollback: [`ses-email-migration.md`](ses-email-migration.md).

---

## Acceptance

- A registration OTP arrives at an external mailbox with **DKIM and DMARC passing**, and the daily cap
  demonstrably exceeds the event headcount.
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
