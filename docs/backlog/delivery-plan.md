# Delivery plan — index

Single source of open work. Replaces the old `register.md`, folding it together with the demo feedback
of **2026-07-29** and the open GitHub issues.

**Every claim was verified against `main` at `211b7b1` on 2026-07-29.** The old register was last
verified at `03d7bf2`; `main` has moved twice since, so nothing was copied forward on trust. Where a
line number differs from the old register, the new one is the true one.

Detail lives in seven epic documents. This file is the map, the coverage guarantee, and the things
that belong to no single epic.

Production is live at `hub.abakacademy.id` running `4d69591` — **6 commits behind `main`**. That gap
is itself a finding; see [F-5](#f-5--continuous-deployment).

---

## 1. The epics

| Epic | Issue | Objective | Client items |
|---|---|---|---|
| **[E1 — Foundation & unblock](e1-foundation-unblock.md)** | [#56](https://github.com/abak-academy/platform/issues/56) | Remove the three things blocking other epics | — |
| **[E2 — Exam authoring](e2-exam-authoring.md)** | [#61](https://github.com/abak-academy/platform/issues/61) | Write, find, correct and score questions the way the client works | 9 |
| **[E3 — Exam delivery, results & certificates](e3-exam-delivery-results.md)** | [#58](https://github.com/abak-academy/platform/issues/58) | Results reach the right people; a disconnected student carries on | 6 |
| **[E4 — Participants & schools](e4-participants-schools.md)** | [#59](https://github.com/abak-academy/platform/issues/59) | A student with no registered school is a first-class participant | 4 |
| **[E5 — Orders & payments](e5-orders-payments.md)** | [#57](https://github.com/abak-academy/platform/issues/57) | Know who bought what and how it was paid; charge what was shown | 6 |
| **[E6 — Shipping & logistics](e6-shipping-logistics.md)** | [#60](https://github.com/abak-academy/platform/issues/60) | Real rates, real waybill, visible tracking | 2 |
| **[E7 — Scale & event readiness](e7-scale-event-readiness.md)** | [#62](https://github.com/abak-academy/platform/issues/62) | 5000 participants on current spec, with the bottleneck named | 2 |

**9 + 6 + 4 + 6 + 2 + 2 = 29** — every client item is in exactly one epic.

---

## 2. Dependencies

They are **not** independent. Hard blocks are solid; the two dashed ones are judgement, not
compilation.

```
                ┌─→ E3   (B-8: the results export writer)
   E1 ──────────┼─→ E4   (B-8: the importer copies that writer)
                └─→ E6   (Gotenberg proven before the label)

   E1 ╌╌╌╌╌╌╌╌╌╌╌→ E2    (D-1: quality gate on regrade tests, not a compile block)
   E3 ──→ E2             (regrade hangs off the results tab)
   E2 ╌╌╌→ E7            (bundle serialises questions — serialising twice is waste)

   free to start now:  E5, E7

   F-6 (SES) left E1 on 2026-07-30 — staying on Hostinger SMTP. It is now scope
   inside E7, which is where the event that forces it lives. The 100/day cap is
   unchanged and binds at ~100 sign-ups a day, not at 5,000.
```

**Two corrections to how this was first written.** The `E1 → E2` edge is a *quality* gate: regrade can
physically be built without the storage seam; what you lose is any reason to trust its tests. And an
earlier draft had a separate session build an interim shipping label from a manually typed waybill —
that was **duplicated work**, since E6 replaces the number entirely. E6 builds the label once.

### Not every epic is a vertical slice

An earlier version of this plan claimed each unit was a full DB → API → UI → browser-verified slice.
That was asserted, not checked, and it is false for two of them:

| Epic | DB | API | UI | Vertical? |
|---|---|---|---|---|
| E1 | — | ✓ | — | **no** — enabling work |
| E7 | — | ✓ | — | **no** — infra + measurement |
| E2, E3, E4, E5, E6 | ✓ | ✓ | ✓ | yes |

E1 and E7 are honest about this in their own headers. Do not try to force a UI deliverable into them.

---

## 3. Coverage

Nothing is dropped, and nothing is in two places. Three sources were swept — the demo notes, the old
register, and open GitHub issues.

> The third source is why this table exists. The first consolidation pass swept only two and **missed
> [issue #55](https://github.com/abak-academy/platform/issues/55)**, a live result-gate bypass. Any
> future pass must sweep all three.

### Client items

| Item | Epic | Item | Epic |
|---|---|---|---|
| FB-1 points decimal | E2 | FB-14 profile / school | E4 |
| FB-2 results tab | E3 | FB-15 checkout overlap | E5 |
| FB-3 delete question | E2 | FB-16 release optional | E3 |
| FB-4 import template | E2 | FB-17 draft meaning | E3 |
| FB-5 bank ordering | E2 | FB-18 participant picker | E4 |
| FB-6 true/false | E2 | FB-19 buyer name | E5 |
| FB-7 numeric id | E2 | FB-19a proof upload | E5 |
| FB-8 studio gating | E3 | FB-19b proof visible | E5 |
| FB-9 edit-not-retype | E2 | FB-19c audit log | E5 |
| FB-10 multi answers | E2 | FB-20 session resume | E3 |
| FB-10a regrade | E2 | NF-1 Biteship L3 | E6 |
| FB-11 cetak resi | E6 | NF-2 promo | E5 |
| FB-12 unlisted school | E4 | NF-3 bulk school | E4 |
| FB-13 end screen | E3 | NF-4 CDN bundle | E7 |
| | | NF-5 load test | E7 |

### GitHub issues

| Issue | Epic |
|---|---|
| [#55 — result gate skipped on cached certificate](https://github.com/abak-academy/platform/issues/55) | E3 |

### Register items

| ID | Epic | ID | Epic |
|---|---|---|---|
| B-1 promo not sent | E5 | D-1 storage seam | E1 |
| B-2 product list cap | E5 | D-2 `is_estimate` | E6 |
| B-3 digital image | E5 | D-3 `payment_method` | E5 |
| B-4 empty reconcile ref | E5 | D-3 bundle columns | E7 |
| B-5 commit before gateway | E5 | D-4 digital-vs-physical | E6 |
| B-6 cursor 500 | E5 | D-6 webhook sig test | E6 |
| B-7 legacy qty > 1 | E5 | F-6 SES | **E7** *(was E1)* |
| B-8 CSV injection | E1 | F-1a certificate art | E3 *(optional)* |
| | | F-1b template types | E3 *(optional)* |
| | | D-8 renderer consolidation | E3 *(optional)* |

*E3's three optional items came from `register.md`, not from the client. They are folded in because E3
already owns the certificate surface, and are explicitly droppable — see that epic's Optional scope
table.*

**Deliberately unscheduled:** D-3 (`orders.invoice_url`), D-5, D-7, F-2a, F-2b, F-2c, F-3, F-4, F-5
— see §4.

---

## 4. Open work not scheduled

Carried from the old register. No client item touches these, so none is in an epic — but none is
closed either.

### F-5 — Continuous deployment

Still a manual `IMAGE_TAG` edit followed by `pull && up -d` on both VMs. WIF now exists, so the
WIF + IAP path is far cheaper than when this was parked.

**More urgent than its priority suggests.** FB-15 is a bug the client saw as broken purely because
production lags `main` by 6 commits. Until F-5 lands, every "still broken" report carries the same
ambiguity, and the cost is paid in trust at every demo.

### Admin accounts — F-2a, F-2b, F-2c

**F-2a** admins cannot edit their own profile; students can. **F-2b** RBAC pass — review which roles
reach which modules; **scope before changing** (E4 makes one role decision and documents it, but does
not do this review). **F-2c** split `/admin/login` from `/login` — **decided:** frontend-only,
path-based, not a subdomain; the driver is `GoogleLogin` hard-coding `RoleStudent`, so an admin
clicking it gets a student account.

### Storefront — F-3, F-4

**F-3** multi-address book (one address per student today). **F-4** catalog facets and real
pagination; facets need spec-value normalisation and the 10-page cursor loop is a stopgap.

### D-5 — no gofmt gate

**37 files** under `backend/` fail `gofmt -l`. Adding the gate means formatting them all in one commit
first, or it lands red. The count grows with every epic that touches Go.

### D-7 — Fazpass is unused and should be deleted

Still referenced in `cmd/api/main.go`, `cmd/worker/main.go`, `config/config.go` and its tests, and six
config/secrets files across dev, staging and prod. OTP goes over SMTP.

### D-3 — `orders.invoice_url`

Dead column, no producer, nothing scheduled. Keep — do not drop.

---

## 5. PRD / TRD drift

Mapped, **not amended**. Two entries need a product decision; the rest is documentation that fell
behind shipped code.

### Scope changes needing sign-off

| # | Item | Conflict |
|---|---|---|
| 1 | **NF-1** Biteship Level 3 (E6) | PRD lists *"Auto-waybill generation and real-time tracking (logistics Level 2/3)"* under **Explicitly Out of Scope (MVP)**. Level 3 reverses it. Verbally approved 2026-07-29; record it. |
| 2 | **NF-5** load test at 5000 (E7) | PRD success metric and Phase 4 both say **10,000 CCU**. 5000-on-current-spec as phase 1 is a phasing decision recorded nowhere. |

### Code ahead of the docs

| # | Drift |
|---|---|
| 3 | TRD ERD `Question` is missing `point_correct`, `point_wrong` and `audio_url`; the `question_blank` table is absent entirely; the `format` CHECK omits `multi_blank`. |
| 4 | PRD FR-COMP-18 and FR-EXAM-01 list five question formats. `multi_blank` shipped undocumented (migration 0028); FB-6 makes **seven**. |
| 5 | TRD marks `ExamRegistration` **"⏳ PHASE-3 DEFERRED — not used in phases 1–2"**. It shipped in migration 0014; roster, participant numbers and check-in all run on it today. |

### Genuinely new — no FR covers it

| # | Item | Epic |
|---|---|---|
| 6 | Proof of payment on manual confirmation | E5 |
| 7 | Listing *active* promos at checkout | E5 |
| 8 | Multiple accepted correct answers | E2 |
| 9 | Configurable end-of-exam content | E3 |

### Not drift — spec is fine, the UI never explained it

**FB-16** (release is optional) and **FB-17** (what draft means) are already correct in the PRD, the
TRD and the code. They are copy problems. Recorded so nobody re-specs them.

---

## 6. Verification debt

Shipped without ever being looked at in a running browser.

| Surface | What to confirm | Owner |
|---|---|---|
| Certificate / exam card | A real end-to-end Gotenberg render | **E1** |
| `/cart` physical | Saved address renders with **Ubah** + **Cek Ongkir**; per-order overrides survive edit and reopen; estimate badge on the flat-rate quote | **E5** |
| `/catalog` | Sticky category rail holds while the grid scrolls; Merchandise and Medali tabs list products | unscheduled |
| `/catalog/[id]` | "Spesifikasi Produk" table renders; blank-value rows absent | unscheduled |
| `/cart` digital | No qty stepper; "Produk digital dibeli 1× per akun." shown | unscheduled |
| `/orders/[id]` physical | "Pengiriman" block shows address, courier, service, ongkir, resi | unscheduled |

---

## 7. Infra / CI

- **`pipeline.yml` runs twice per push** — a bare `on: push` *and* `on: pull_request`, neither
  filtered, ~13 min each. Free while the repo is public; the single largest minute drain the day it
  isn't. One-commit fix: `push: branches: [main]` plus a `concurrency` group with
  `cancel-in-progress`. Do **not** path-filter the image build — `build-image.sh` publishes api,
  worker and web under one SHA and the compose files deploy them via a single `IMAGE_TAG`.
- **The repo is still public** (`abak-academy/platform`). Going private breaks the staging VM's
  `docker login ghcr.io` and starts metering Actions.
  Runbook: [`../runbooks/repo-migration-to-client-org.md`](../runbooks/repo-migration-to-client-org.md).
- **`app-staging.yaml` in the repo is deliberately stale** — the VM's copy is hand-edited, never
  pulled. Its GHCR paths still point at the old org.
- The empty `default` VPC network should be deleted so nobody launches a VM into it by accident.

---

## 8. Questions for the client

The list started at six. **All six are now closed** — five answered, one withdrawn as scope nobody
asked for. Nothing here blocks any epic.

One thing is still *owed*, though it is not a question: the **PRD amendment** for the NF-1 scope
reversal. The client has signed off; the document has not caught up.

### Closed

| # | Question | Outcome |
|---|---|---|
| 1 | **Active promo codes at checkout** — list every active code? | **Opt-in, agreed 2026-07-30.** `promo_code.is_public BOOLEAN NOT NULL DEFAULT false`; only published codes are listed. Default-closed means nothing leaks when the endpoint ships, including existing codes, and the client opens them one at a time. See [E5](e5-orders-payments.md). |
| 2 | **NF-1 scope change** vs the PRD's out-of-scope list | **Signed off by the client 2026-07-29.** E6 records it before the first commit; the PRD amendment is still owed. |
| 3 | **Email volume / SES** | Not a cost decision — SES is ~$0.10 per 1,000 emails, so a 5,000-person event is about **$0.50**. **Deferred 2026-07-30:** staying on Hostinger SMTP for now; F-6 moved from E1 into E7. The cap is unchanged and binds at ~100 sign-ups a day, and the AWS sandbox review is the step whose timing is out of our hands — so the trigger is *an event date being discussed*, not the event being close. Runbook: [`ses-email-migration.md`](ses-email-migration.md). |
| 4 | **`/admin/school/classes` stub** | **Not a commitment.** Created 2026-06-19 in `d371213` *"Coming-soon scaffolding for not-ready admin modules"*, orphaned on 2026-07-05 when `ee9bc6f` reverted the `feat/school-slice-a` merge (PR #18). A later commit titled *"remove last ComingSoon stub"* missed it — most likely because it has no nav entry and is unreachable in the UI. Removing it is one file plus six dead i18n lines. |
| 5 | **FB-1 zero-point question** | **Withdrawn — this was never a client question.** It came from reading the migration, not from the feedback. The real, forced decision is the lower bound, since `>= 1` rejects `0.5`: settled at **`> 0`** (0.1, 0.2 allowed; zero stays impossible). See [E2](e2-exam-authoring.md). |
| 6 | **FB-10 matching rules** | Answerable without the client — the example (`2`, `dua`) is a *list of accepted answers*, not a request for automatic equivalence. Rule stays minimal: trim, case-fold, collapse internal whitespace, exact match per listed answer. Recorded in [E2](e2-exam-authoring.md) pending a nod. |
