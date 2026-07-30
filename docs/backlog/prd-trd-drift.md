# PRD / TRD drift

**Not an epic — a ledger.** Nothing here is scheduled; it records where the specification and the
shipped code disagree, so that neither is trusted blindly and nobody re-specs something that already
works.

Mapped, **not amended** — with one exception: the four §1 scope reversals *were* amended, and PRD
**v1.2 (2026-07-30)** closed the last of them. Everything below §1 is still documentation that fell
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
| 1 | **NF-1** Biteship Level 3 — [E6](e6-shipping-logistics.md) | The PRD listed *"Auto-waybill generation and real-time tracking (logistics Level 2/3)"* under **Explicitly Out of Scope (MVP)**, with only rate calculation (Level 1) in scope. Level 3 reverses that line. **✅ Signed off verbally by the client 2026-07-29. ✅ Written 2026-07-30 (PRD v1.2)** — the out-of-scope bullet is struck through and **FR-STORE-ADM-08a/b/c** and **FR-STORE-05c** added; FR-STORE-ADM-08 and FR-SA-14b amended. |
| 2 | **NF-5** load test at 5000 — [E7](e7-scale-event-readiness.md) | The PRD success metric and Phase 4 both say **10,000 CCU**. Treating 5000-on-current-spec as phase 1 is a phasing decision. **✅ Written 2026-07-30 (PRD v1.2)** — 5,000 on current VM spec is phase 1, the 10,000 target unchanged as phase 2. |
| 10 | **FB-26** multi-attempt exams — [H1 §8](h1-live-bugs-2026-07-30.md#8-fb-26--multi-attempt-exams-approved-2026-07-30) | Reverses **FR-COMP-02 (Must)**, which deferred multi-attempt on 2026-07-07 (*"`max_attempts` is stored but not consulted"*), plus two TRD column comments. **✅ Signed off by the client 2026-07-30. ✅ Written 2026-07-30** — FR-COMP-02, the two TRD comments and the two schema.dbml notes all state the `max_attempts IS NULL or 0 = single-attempt` rule. |
| 11 | **FB-32** audio scope — [H1 §9](h1-live-bugs-2026-07-30.md#9-fb-32--audio-is-per-question-and-per-section-answered-2026-07-30) | The PRD contradicted itself: prose promised audio *"per question"*, **FR-COMP-19a** and **FR-EXAM-05** specified section-level only, **FR-EXAM-01** listed images only. **✅ Resolved by the client 2026-07-30 — both scopes are supported.** FR-EXAM-01 must gain audio; FR-COMP-19a stands. |

### Amendment status — all closed

**Nothing is owed. All four amendments are signed off and written** as of PRD **v1.2 (2026-07-30)**.

1. **NF-1** (Biteship Level 3) — signed off 2026-07-29, **written 2026-07-30**. The out-of-scope bullet
   is struck through and reversed in place; **FR-STORE-ADM-08a** (provider creates the shipment and
   returns the waybill), **08b** (printable label from that waybill) and **08c** (tracking webhooks
   update status) are new, plus FR-STORE-05c. E6 builds against these.
   > ⚠️ **FR-STORE-ADM-08c says *"Webhook signature verified; unsigned or tampered callbacks
   > rejected"* — Biteship publishes no signing scheme**, only a static custom header configured in
   > their dashboard. As written the criterion cannot be met by any implementation. E6 compensates
   > (constant-time compare on the static header, 401 when the secret is unset, then re-fetch
   > `GET /v1/orders/:id` and trust Biteship's own answer over the payload). The wording overstates
   > what the provider supports and should be softened to "callback authenticity checked" on the next
   > PRD pass. **Provider-imposed, not an implementation shortfall.**
2. **NF-5** (load-test phasing) — **written 2026-07-30**. 5,000 concurrent on current VM spec is
   phase 1; the 10,000 CCU target is unchanged as phase 2.
3. **FB-32** (audio at both scopes) — resolved 2026-07-30, **written 2026-07-30**. FR-EXAM-01 now lists
   audio alongside image as a per-question attachment, and records that the
   `section_type = 'listening'` render narrowing was removed at both question and section scope. The
   zone spec's C3/FR44/FR45 were amended in the same pass — they had described the opposite of what
   shipped, which is how this was caught.
4. **FB-26** (multi-attempt exams) — signed off 2026-07-30, **written 2026-07-30**. FR-COMP-02, the
   TRD's comments on `max_attempts` and `attempts_used`, and the matching `schema.dbml` notes now state
   the `max_attempts IS NULL or 0 = single-attempt` rule.

**`requirements/` is outside any git repository** — `git rev-parse --show-toplevel` fails there — so none
of these could ride an implementation PR. This ledger is the repo-side record and is what changes with
the code; the PRD and TRD are edited out-of-band, which is exactly how NF-1's amendment came to be owed
for two days before v1.2 closed it.

None blocks its epic, and **none carries an open action any more.** The only thing left on this page
worth a decision is the FR-STORE-ADM-08c wording flagged above, which is a softening, not a debt.

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
