# PRD / TRD drift

**Not an epic — a ledger.** Nothing here is scheduled; it records where the specification and the
shipped code disagree, so that neither is trusted blindly and nobody re-specs something that already
works.

Mapped, **not amended**. Two entries need a product decision; the rest is documentation that fell
behind the code.

**Verified against** `main` @ `211b7b1`, 2026-07-29.

**Entries 10 and 11 added 2026-07-30** — the two signed-off scope reversals from the
[H1](h1-live-bugs-2026-07-30.md) batch, checked against `d2efa3a`. They are one-liners on purpose:
**H1 owns the engineering detail for that batch, and this ledger only records what a spec-amender needs
to know.** Anything else from H1 deliberately does not appear here.

---

## 1. Scope changes needing sign-off

| # | Item | Conflict |
|---|---|---|
| 1 | **NF-1** Biteship Level 3 — [E6](e6-shipping-logistics.md) | The PRD lists *"Auto-waybill generation and real-time tracking (logistics Level 2/3)"* under **Explicitly Out of Scope (MVP)**, with only rate calculation (Level 1) in scope. Level 3 reverses that line. **Signed off verbally by the client 2026-07-29** — E6 records the sign-off before its first commit. |
| 2 | **NF-5** load test at 5000 — [E7](e7-scale-event-readiness.md) | The PRD success metric and Phase 4 both say **10,000 CCU**. Treating 5000-on-current-spec as phase 1 is a phasing decision recorded nowhere. |
| 10 | **FB-26** multi-attempt exams — [H1 §8](h1-live-bugs-2026-07-30.md#8-fb-26--multi-attempt-exams-approved-2026-07-30) | Reverses **FR-COMP-02 (Must)**, which deferred multi-attempt on 2026-07-07 (*"`max_attempts` is stored but not consulted"*), plus two TRD column comments. **✅ Signed off by the client 2026-07-30. ✅ Written 2026-07-30** — FR-COMP-02, the two TRD comments and the two schema.dbml notes all state the `max_attempts IS NULL or 0 = single-attempt` rule. |
| 11 | **FB-32** audio scope — [H1 §9](h1-live-bugs-2026-07-30.md#9-fb-32--audio-is-per-question-and-per-section-answered-2026-07-30) | The PRD contradicted itself: prose promised audio *"per question"*, **FR-COMP-19a** and **FR-EXAM-05** specified section-level only, **FR-EXAM-01** listed images only. **✅ Resolved by the client 2026-07-30 — both scopes are supported.** FR-EXAM-01 must gain audio; FR-COMP-19a stands. |

### Still owed

**Three PRD amendments, all signed off; one written:**

1. **NF-1** (Biteship Level 3) — signed off 2026-07-29. Still owed.
2. **FB-32** (audio at both scopes) — resolved 2026-07-30. FR-EXAM-01 must gain audio alongside image.
   Still owed.
3. **FB-26** (multi-attempt exams) — signed off 2026-07-30, **written 2026-07-30**. FR-COMP-02, the
   TRD's comments on `max_attempts` and `attempts_used`, and the matching `schema.dbml` notes now state
   the `max_attempts IS NULL or 0 = single-attempt` rule.

**`requirements/` is outside any git repository** — `git rev-parse --show-toplevel` fails there — so none
of these can ride an implementation PR. This ledger is the repo-side record and is what changes with the
code; the PRD and TRD are edited out-of-band, which is exactly how NF-1's amendment came to be owed for a
second day.

None blocks its epic. All three are debts against the spec, and they are the only items on this page with
an action attached.

---

## 2. Code ahead of the docs

| # | Drift |
|---|---|
| 3 | TRD ERD `Question` is missing `point_correct`, `point_wrong` and `audio_url`; the `question_blank` table is absent entirely; the `format` CHECK omits `multi_blank`. |
| 4 | PRD FR-COMP-18 and FR-EXAM-01 list five question formats. `multi_blank` shipped undocumented (migration 0028); FB-6 makes **seven**. |
| 5 | TRD marks `ExamRegistration` **"⏳ PHASE-3 DEFERRED — not used in phases 1–2"**. It shipped in migration 0014; roster, participant numbers and check-in all run on it today. |

---

## 3. Genuinely new — no FR covers it

| # | Item | Epic |
|---|---|---|
| 6 | Proof of payment on manual confirmation | [E5](e5-orders-payments.md) |
| 7 | Listing *active* promos at checkout | [E5](e5-orders-payments.md) |
| 8 | Multiple accepted correct answers | [E2](e2-exam-authoring.md) |
| 9 | Configurable end-of-exam content | [E3](e3-exam-delivery-results.md) |

---

## 4. Not drift — the spec is fine, the UI never explained it

**FB-16** (release is optional) and **FB-17** (what draft means) are already correct in the PRD, the
TRD and the code. They are **copy problems**, and they belong to E3 as such.

Recorded here so nobody re-specs them.
