# PRD / TRD drift

**Not an epic — a ledger.** Nothing here is scheduled; it records where the specification and the
shipped code disagree, so that neither is trusted blindly and nobody re-specs something that already
works.

Mapped, **not amended**. Two entries need a product decision; the rest is documentation that fell
behind the code.

**Verified against** `main` @ `211b7b1`, 2026-07-29. **Entries 10–15 added 2026-07-30** from the
[H1](h1-live-bugs-2026-07-30.md) batch, checked against `d2efa3a` — code unchanged between the two
(docs-only commits).

---

## 1. Scope changes needing sign-off

| # | Item | Conflict |
|---|---|---|
| 1 | **NF-1** Biteship Level 3 — [E6](e6-shipping-logistics.md) | The PRD lists *"Auto-waybill generation and real-time tracking (logistics Level 2/3)"* under **Explicitly Out of Scope (MVP)**, with only rate calculation (Level 1) in scope. Level 3 reverses that line. **Signed off verbally by the client 2026-07-29** — E6 records the sign-off before its first commit. |
| 2 | **NF-5** load test at 5000 — [E7](e7-scale-event-readiness.md) | The PRD success metric and Phase 4 both say **10,000 CCU**. Treating 5000-on-current-spec as phase 1 is a phasing decision recorded nowhere. |
| 10 | **FB-26** multi-attempt exams — [H1](h1-live-bugs-2026-07-30.md) §4 | **FR-COMP-02 (Must) deliberately deferred it on 2026-07-07** — *"`Exam.max_attempts` is stored but not consulted… remedial = a separate exam"* — and the TRD repeats it as a comment on both `max_attempts` and `attempts_used`. Honouring `max_attempts` **reverses that deferral**, same shape as NF-1. The client asked for it on 2026-07-30 having been shown an editable field (FR-EXAM-09). **Not signed off.** |
| 11 | **FB-32** per-question audio — [H1](h1-live-bugs-2026-07-30.md) §6 | **The PRD contradicts itself.** Prose: *"Image and audio attachments **per question**"*. FR-COMP-19a: **section-level** only. FR-EXAM-05 puts `audio_url`/`audio_play_limit` on **Test**. FR-EXAM-01 lists question attachments as **image only**. `question.audio_url` shipped in migration 0028 and reached neither doc. The renderer follows the FR table, the authoring form follows the prose. **Needs a decision, then one of the two ends changes.** |

### Still owed

The **PRD amendment for NF-1**. The client has signed off; the document has not caught up. This is
not a question and does not block E6 — it is a debt against the spec, and the only item on this page
with an action attached.

---

## 2. Code ahead of the docs

| # | Drift |
|---|---|
| 3 | TRD ERD `Question` is missing `point_correct`, `point_wrong` and `audio_url`; the `question_blank` table is absent entirely; the `format` CHECK omits `multi_blank`. |
| 4 | PRD FR-COMP-18 and FR-EXAM-01 list five question formats. `multi_blank` shipped undocumented (migration 0028); FB-6 makes **seven**. |
| 5 | TRD marks `ExamRegistration` **"⏳ PHASE-3 DEFERRED — not used in phases 1–2"**. It shipped in migration 0014; roster, participant numbers and check-in all run on it today. |
| 12 | **The whole rich-text editor is unspecified.** The PRD covers KaTeX math and image attachments and says *nothing* about text formatting, bullets, line breaks or tables — yet `RichTextEditor`, a four-copy HTML allowlist and a paste sanitiser all shipped. FB-21…FB-25 are therefore defects against reasonable expectation, not against a spec. **FB-24 (line breaks stripped at save) is still a P0 data-loss bug** on its own merit. |
| 13 | **Region reference data is unspecified.** Neither document mentions province / city / district. Migration 0029 seeded 34 provinces, 514 cities and 7215 districts from a Permendagri 72/2019-vintage dataset, and `orders` now carries FKs into all three. FB-34 (four post-2022 Papua provinces missing) is a data-quality bug in a surface no FR describes. |
| 14 | **A student-facing result history has no FR.** FR-EXAM-16/17/18 are all *admin* result-tab requirements. FR-COMP-23/25/26/27 describe the result *page* but presume it is reachable — FR-COMP-27 says the certificate is downloaded *"from the result page"*. **FB-27 breaks that premise**: `exam_registration.status` never advances to `submitted` and no list-sessions endpoint exists, so reachability is in scope while the history list itself is new. |
| 15 | **FR-EXAM-15 — *"Registrations tab: manual participant registration"* — is marked `Won't \| growth`**, yet the tab and `ParticipantPicker` shipped and the client now depends on them (FB-12, FB-18, and FB-35 as the second report). [E4](e4-participants-schools.md) is repairing functionality the PRD says was never in MVP scope. Unlike NF-1 that reversal was never recorded. **Record it or re-scope it.** |

---

## 3. Genuinely new — no FR covers it

| # | Item | Epic |
|---|---|---|
| 6 | Proof of payment on manual confirmation | [E5](e5-orders-payments.md) |
| 7 | Listing *active* promos at checkout | [E5](e5-orders-payments.md) |
| 8 | Multiple accepted correct answers | [E2](e2-exam-authoring.md) |
| 9 | Configurable end-of-exam content | [E3](e3-exam-delivery-results.md) |
| 16 | Table in a question body — FB-21 | unscheduled — [H1](h1-live-bugs-2026-07-30.md) |
| 17 | Student result history list — FB-27 | unscheduled — [H1](h1-live-bugs-2026-07-30.md) |
| 18 | A consequence for proctoring violations — FB-31 | unscheduled. **FR-COMP-13 (detect + warn + log) is fully satisfied**, so this is new behaviour, not a repair — [H1](h1-live-bugs-2026-07-30.md) §8 |
| 19 | Payment-instructions page — FB-29 | unscheduled. Deleting the dead button needs no FR; building the page does — [H1](h1-live-bugs-2026-07-30.md) |

---

## 4. Not drift — the spec is fine, the UI never explained it

**FB-16** (release is optional) and **FB-17** (what draft means) are already correct in the PRD, the
TRD and the code. They are **copy problems**, and they belong to E3 as such.

Recorded here so nobody re-specs them.
