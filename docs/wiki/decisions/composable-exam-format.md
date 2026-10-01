---
title: Composable exam format — explicit section configuration instead of named modes
updated: 2026-10-01
verified_against: 57b2fd87
---

Why the exam runtime stops branching on `exam.mode` and what was deliberately left out of `docs/backlog/exam-format-system-design.md`. Behaviour lives in [[exam-lifecycle-and-scoring]]; the implementation contract for the first slice is `docs/specs/composable-exam-format.md`.

## Context

`standard | utbk | ielts` acted as an implicit configuration: the name decided flat vs sectioned runtime, timer scope, and (implicitly) presentation. Two client needs could not be expressed at all — showing a whole section on one page, and section-wide context material for the questions underneath it. Every new format would have added another branch.

## Decisions

1. **Runtime path is an explicit `exam.format_version` (1 legacy, 2 composable), not an adapter over every exam.** An adapter would move all legacy exams onto one runtime and make rollback a data write; a version flag keeps legacy behaviour byte-identical and makes rollback a value flip. Cost: two paths until the legacy path is retired.
2. **No per-attempt policy snapshot; edit safety instead.** Once a format-2 exam has any session, `navigation_policy`, `randomize`, and the attached-test list are rejected, so a running attempt cannot be re-configured; while only registrations exist the change is allowed and audit-logged. A snapshot table would have been state that duplicates what the edit block already guarantees.
3. **Section presentation lives on `exam_test`; duration stays on `test`.** Presentation is per attachment (the same Test can be shown differently in two exams), while duration is a property of the Test itself — the product decision is that attaching a Test adopts its duration, so no per-attachment duration column exists.
4. **No `timer_scope` column.** `exam.timer_mode` (`overall | per_test`) plus `duration_minutes` already expresses exam timer, section timer, and no timer.
5. **Randomization is derived, not stored.** A deterministic permutation seeded by `(session_id, test_id)` gives per-attempt stability without a stored order; dropping the seed is only safe because the seed is a pure function of ids that never change mid-attempt.
6. **Audio deferred.** Section audio fields, `audio_policy`, and play-limit enforcement are out of the first slice: enforcement semantics across refresh and device changes are unresolved, and a cosmetic limit would be worse than none.
7. **`section_transition_policy` and `result_grouping` deferred** until a real requirement (a second transition mode, or grouped results/export) needs them.

## Consequences

- Publish for format 2 runs a format matrix (`validateComposablePublishSections`) and never consults `mode` or `test.section_type`; format 1 keeps the FR-19 gate.
- Question material is machine-detectable (`material_html` / `image_url` / `audio_url`), which is what lets `all_on_page` refuse to silently drop authored content.
- Test-owned content (title, question list, shared material) is still read live, so editing a Test affects in-flight attempts that use it. Closing that needs Test revisions — deferred, and the main residual risk.
- `exam.mode` keeps being written and read for format-1 exams until the conversion of existing exams happens.

## Migration and rollback

Migration `0068_exam_format_policy.up.sql` is additive with no backfill; existing rows stay `format_version = 1`. New exams also stay `1` until the admin section-configuration UI ships, at which point the create path switches to `2`. Rollback is leaving the columns in place and unused: format-1 behaviour never changed.
