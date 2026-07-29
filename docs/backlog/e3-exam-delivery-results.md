# E3 — Exam delivery, results & certificates

| | |
|---|---|
| **Issue** | [#58](https://github.com/abak-academy/platform/issues/58) |
| **Objective** | Results reach exactly the people who should see them, a disconnected student can carry on, and the certificate studio stops being a door that is always open. |
| **Source IDs** | FB-2, FB-8, FB-13, FB-16, FB-17, FB-20 + **GitHub issue #55** · optional: F-1a, F-1b, D-8 |
| **Client items** | 6 |
| **Depends on** | E1 (B-8 — the results export writer) |
| **Verified against** | `main` @ `211b7b1`, 2026-07-29 |

The connecting thread is *what happens around the exam rather than inside it* — who may see a result,
what a student comes back to, and what leaks when a gate is skipped.

---

## 1. Issue #55 — the result gate is skipped when the certificate is cached

**Not from the demo feedback.** [Issue #55](https://github.com/abak-academy/platform/issues/55) was
filed during the PR #53 review and deliberately did not block that merge. It was missed in the first
consolidation pass because that pass swept two sources — the demo notes and the old register — and
never swept open GitHub issues.

`resolveCertificateURL` stopped evaluating the result gate once the PDF is cached, so **a certificate
printing a score can still be downloaded after an admin has hidden the results.** In
`backend/internal/service/certificate.go` the `allowed` computation moved inside the
`if needsRegeneration` branch, to avoid DB work on the cache path:

```go
if needsRegeneration {
    allowed, err := s.addCertificateSessionValues(...)
    if !allowed { return nil, nil }     // gate only checked here
}
signed, err := s.presignReadURL(...)    // cache path: gate skipped
```

It belongs in this epic because it is the same question as FB-16: **who is allowed to see a result.**
Fix it alongside the release semantics rather than as an isolated patch, so both paths end up behind
one gate.

---

## 2. FB-2 — build the results tab

The tab is a stub: [`packages/[id]/page.tsx:605`](../../web/app/(admin)/admin/exam/packages/[id]/page.tsx)
renders `<UnderMaintenance>`. The old register never listed it.

Build it: name, school, score, submitted date, per-topic breakdown. FR-EXAM-17 has always marked this
**Must**. This is also where **E2**'s regrade action (FB-10a) will hang.

**Tests go in at the handler level**, not the service layer — the service tests for this area run on
shim copies until E1's seam lands, and this epic should not wait for it.

---

## 3. FB-16 and FB-17 — copy, not features

Both are already correct in the PRD, the TRD **and** the code. Recorded here so nobody re-specs them.

**FB-16 — "make sure result release is optional."** Already true: `result_config` supports `hidden`
and `result_release_at` is nullable. An exam whose results are never published is a supported
configuration today. Nothing in the UI *says* so.

**FB-17 — "what does draft status mean?"** `Exam.status` is `draft | published`, mutable only through
the publish command and never through `PATCH` — the lifecycle-vs-flag convention the TRD sets out. A
draft exam is invisible to students and freely editable; a published one is neither. The client asked
because the UI never explains it.

The work is affordance and wording. Resist turning either into a feature.

---

## 4. FB-20 — resume at the same question after a disconnect

Answers already survive — they are persisted server-side on every change. **The position does not.**
`currentQIndex` is `useState(0)` in
[`(exam-session)/exam/sessions/[id]/page.tsx:68`](../../web/app/(exam-session)/exam/sessions/[id]/page.tsx),
and there is no `localStorage` or `sessionStorage` anywhere in that route group. A reconnecting student
lands back on question 1 with their answers intact.

**Persist the position server-side, alongside the answers — not in the browser.** The client described
*logging out and logging back in*, possibly on another device; browser storage would not survive
either. FR-COMP-10 already promises resume "from last saved position".

---

## 5. FB-13 — end-of-exam screen can carry an image and a promo

Admin-configurable content shown after submission. Keep it dumb: one image, one promo block, per exam.
No templating engine.

---

## 6. FB-8 — gate the certificate studio

The studio rebuild landed in `211b7b1` (PR #53) — new editor, `certificate-studio.ts`, inspector
panel, five brand fonts. **None of it touched when the studio is reachable:** `CertificateDesignTab`
is still an unconditional tab
([`packages/[id]/page.tsx:593`](../../web/app/(admin)/admin/exam/packages/[id]/page.tsx)).

The client's ask is narrow — the studio should only be reachable when an admin is actually enabling or
creating a certificate for that exam. Because the component was just rebuilt, this is now a small
change against clean code rather than a change on top of work in flight.

**Acceptance for this item specifically:** the tab is absent until certificates are enabled for the
exam; enabling is an explicit action; disabling does not destroy a saved design.

---

## Acceptance

- Results tab lists submitted results for an exam with a real session behind it, checked in a browser.
- A certificate for an exam with `result_config = hidden` cannot be downloaded — **including when the
  PDF is already cached**, with a regression test that fails on the cache path before the fix.
- The result-config control states in words what each option means, and that "not published" is a
  valid end state.
- Opening a draft exam explains what draft means.
- Submitting nothing, closing the tab, logging in on a second device and reopening the session lands
  on the same question with the same answers.
- An exam with a configured end screen shows the image and promo after submit; one without shows the
  plain result.
- The certificate tab is unreachable until certificates are enabled.

## Optional scope — certificates, if there is room

**These three come from the old `register.md`, not from the demo feedback.** The client's ask was
gating only (FB-8); they were folded in here because this epic already owns the certificate surface.
None of them is required for the epic to be done — take them in order of cost, drop them freely.

| ID | Item | Cost |
|---|---|---|
| **F-1a** | Redesign the 3 built-in templates | **Cheap.** Swap 3 PNGs in `internal/service/assets/` and tune `defaultLayout`. No migration. `modern` and `elegant` have dead white space mid-lower. |
| **F-1b** | Admin-definable template *types* | **Medium.** Move built-ins out of hardcoded Go into a `certificate_template` table (`name, background_key, layout JSONB, is_builtin`); the 3 become seed rows. Reuses the DnD editor and Gotenberg renderer unchanged. |
| **D-8** | Consolidate the two renderers | **Expensive — do not start casually.** The certificate is rendered twice, in the browser and in Go, with no test comparing them. The obvious fix needs a signed short-lived URL for Gotenberg's Chromium — a new auth surface, not a refactor. Full reasoning: [`certificate-rendering-consolidation.md`](certificate-rendering-consolidation.md). |

If only one is taken, take F-1a. If D-8 is taken, it should probably be its own branch rather than a
tail on this epic.

## Out of scope

- Essay grading queue — already shipped and working.
- Leaderboard — shipped; `allow_leaderboard` already gates it.
