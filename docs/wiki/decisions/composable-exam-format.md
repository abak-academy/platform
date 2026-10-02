---
title: Composable exam format — explicit section configuration instead of named modes
updated: 2026-10-03
verified_against: b1bdfeaf
---

Why the exam runtime stops branching on `exam.mode` and what was deliberately left out of `docs/backlog/exam-format-system-design.md`. Behaviour lives in [[exam-lifecycle-and-scoring]]; the implementation contract for the first slice is `docs/specs/composable-exam-format.md`.

## Context

`standard | utbk | ielts` acted as an implicit configuration: the name decided flat vs sectioned runtime, timer scope, and (implicitly) presentation. Two client needs could not be expressed at all — showing a whole section on one page, and section-wide context material for the questions underneath it. Every new format would have added another branch.

## Decisions

1. **Runtime path is an explicit `exam.format_version` (1 legacy, 2 composable), not an adapter over every exam.** An adapter would move all legacy exams onto one runtime and make rollback a data write; a version flag isolates format-2 behaviour and retains the legacy runtime. Cost: two paths until the legacy path is retired.
2. **No per-attempt policy snapshot; edit safety instead.** Once a format-2 exam has any session, changes to `navigation_policy`, `randomize`, and the attached-test list are rejected; while only registrations exist the change is allowed and audit-logged. This avoids a snapshot table but accepts a race between the pre-write check and a concurrent session start, as recorded in the spec.
3. **Section presentation lives on `exam_test`; duration stays on `test`.** Presentation is per attachment (the same Test can be shown differently in two exams), while duration is a property of the Test itself — the product decision is that attaching a Test adopts its duration, so no per-attachment duration column exists.
4. **No `timer_scope` column.** `exam.timer_mode` (`overall | per_test`) plus `duration_minutes` provides whole-exam and per-section clocks; current format-2 publishing requires a positive duration.
5. **Randomization is derived, not stored.** A deterministic permutation seeded by `(session_id, test_id)` gives per-attempt stability without a stored order; dropping the seed is only safe because the seed is a pure function of ids that never change mid-attempt.
6. **Audio deferred.** Section audio fields, `audio_policy`, and play-limit enforcement are out of the first slice: enforcement semantics across refresh and device changes are unresolved, and a cosmetic limit would be worse than none.
7. **`section_transition_policy` and `result_grouping` deferred** until a real requirement (a second transition mode, or grouped results/export) needs them.

## Consequences

- Slice 1 implements persistence, publish validation, edit safety and deterministic ordering. PR-2 adds mode-independent navigation and clocks, material authoring and rendering, and switches new API-created exams to format 2. Existing exams keep their legacy runtime.

- Publish for format 2 runs a format matrix (`validateComposablePublishSections`) and never consults `mode` or `test.section_type`; format 1 keeps the FR-19 gate.
- Question material is machine-detectable (`material_html` / `image_url` / `audio_url`), which is what lets `all_on_page` refuse to silently drop authored content.
- Test-owned content (title, question list, shared material) is still read live, so editing a Test affects in-flight attempts that use it. Closing that needs Test revisions — deferred, and the main residual risk.
- `exam.mode` keeps being written and read for format-1 exams until the conversion of existing exams happens.

## Migration and rollback

Migration `0068_exam_format_policy.up.sql` is additive with no backfill; existing rows stay `format_version = 1`. PR-2 creates new exams as `2` alongside the admin section-configuration UI. Deploy PR-1 before PR-2, and coordinate backend/expiry-worker rollout before authoring format-2 exams: older workers cannot interpret their navigation and clocks. Reload existing admin clients after deployment. Keep additive columns during rollback. Once format-2 attempts exist, retain compatible runtime code or drain those attempts before rolling back; flipping active exams to format 1 changes their semantics and is not a safe rollback.
