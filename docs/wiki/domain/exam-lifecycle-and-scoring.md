---
title: Exam lifecycle and scoring — question model, session flow, grading authority
updated: 2026-10-02
verified_against: 6ebe3dc6
---

What an exam/package/question actually looks like today, how a student gets access, what happens from session start to result, and where scoring authority sits. See [[system-overview]] for component boundaries, [[exam-capacity]] for the write-path/perf angle on autosave and submit, [[certificate-rendering]] for what happens after a session is graded, and [[rbac-and-accounts]] for who can call the admin endpoints below.

## Content model

- `exam` (one exam) → `exam_test` (join, ordered) → `test` (a section/subtest, e.g. one subject) → `test_question` (join, ordered) → `question` (bank item). Chain: `db/migrations/0014_exam.up.sql`, repointed in `db/migrations/0025_question_bank.up.sql`.
- Questions are bank items, not test-owned: migration 0025 drops `question.test_id`/`sort_order`, adds `test_question(test_id, question_id, sort_order)` and `question.topic_id → exam_topic` (`db/migrations/0025_question_bank.up.sql:12-45`). One question can be reused across multiple tests/exams via `test_question`.
- `exam.mode` discriminates `standard | utbk | ielts` (`db/migrations/0021_exam_modes.up.sql:2-4`, enforced in code via `validModes`, `backend/internal/service/exam.go:37-42`). `utbk`/`ielts` are "sectioned": their attached `test`s each run as a timed section with independent state (`isSectionedMode`, `exam.go:55`).
- `test.section_type` (`listening|reading|writing`, nullable) exists only for the sectioned modes (`0021_exam_modes.up.sql:7-9`, `validSectionTypes`, `exam.go:44-49`).

## Composable exam format (`exam.format_version = 2`)

Migration `db/migrations/0068_exam_format_policy.up.sql` adds the format columns (`1` = legacy mode-driven, `2` = composable). Slice 1 uses `format_version` for publish validation, edit safety and question ordering; session timing and transitions still branch on `exam.mode`. New and existing exams stay `1`; switching new exams to `2` requires PR-2's runtime and admin UI. See [[composable-exam-format]] for why the design doc's policy set was trimmed.

- **Section configuration is per attachment**: `exam_test.question_presentation` (`one_per_page | all_on_page`, CHECK in 0068). The section label is `test.title`; the section duration stays `test.duration_minutes` — there is deliberately no `exam_test.duration_minutes`.
- **Shared vs question material**: `test.shared_material_html` and `test.instructions_html` carry section-wide context; `question.material_html` carries per-question reference content (alongside the pre-existing `question.image_url`/`audio_url`). All three pass the question-body allowlist (`sanitizeQuestionBody`, `backend/internal/service/exam.go`) on every write path. `question.body` is never material.
- **Material contract**: `hasQuestionMaterial` (`backend/internal/service/exam.go`) detects question-owned material, and publish rejects an `all_on_page` section containing any. Rendering question material with shared fallback for `one_per_page`, or shared material once for `all_on_page`, is PR-2 work; slice 1 does not add material to student session payloads.
- **Publish matrix for format 2** (`validateComposablePublishSections`, `backend/internal/service/exam.go`): at least one attached test; positive `exam.duration_minutes` when `timer_mode=overall`; positive section duration and `navigation_policy=sequential` when `timer_mode=per_test`; every section has questions; the `all_on_page` material rule. Timers need no new column because `timer_mode` (`overall | per_test`) plus durations already expresses exam timer, section timer, and no timer. Format 1 keeps the FR-19 mode/`section_type` gate unchanged (`validatePublishSections`).
- **Edit safety**: for format 2, `navigation_policy`, `randomize`, and the attached-test list/`question_presentation` are rejected once the exam has any session (`Repository.ExamHasAnySession`) and audit-logged as `exam.format_policy` while only registrations exist (`checkExamPolicyEditSafety`/`checkExamTestsEditSafety`). Format-1 exams keep the previous latitude. There is no per-attempt policy snapshot table. The guard is a pre-write check: a session starting concurrently with an admin edit can still race it; Test-owned content also remains editable and is read live.
- **Randomization**: for format 2 with `exam.randomize`, each section's question order is a deterministic permutation seeded by `(session_id, test_id)` (`orderQuestionsForAttempt`, `backend/internal/service/exam_session.go`), so refresh/reconnect reuse the same order without storing it. Format 1 keeps `test_question.sort_order` and the toggle stays inert, as before.
- **Session payloads**: `sessionTestsPayload` is the single assembly point for start, reconnect, and section advance, so all three agree on the order. Section timers still read the snapshot `exam_session_section.duration_minutes`, not the live Test.

## Question formats — current state

Current DB CHECK constraint (`question_format_check`, last widened in `db/migrations/0049_question_authoring.up.sql:37-38`, chain: 0014 → 0028 adds `multi_blank` → 0049 adds `true_false`):

```
mcq | multi_answer | short | fill_blank | essay | multi_blank | true_false
```

Mirrored in Go as `validQuestionFormats` (`backend/internal/service/exam.go:27-35`) — both lists agree; no format exists in one but not the other.

| Format | Validated (`validateQuestion`, `exam.go:238-`) | Graded (`gradeObjective`/`gradeAnswer`, `exam_grading.go`) | Notes |
|---|---|---|---|
| `mcq` | ≥2 options, exactly 1 `is_correct` | `gradeMCQ` — exact key match (`exam_grading.go:37-49`) | |
| `multi_answer` | ≥2 options, ≥1 correct | Partial credit per selected option since 2026-07-31 (`gradeMultiAnswerPartial`, `exam_grading.go:60-96`); binary variant `gradeMultiAnswer` (`:98-122`) is now dead code for objective grading but still called from `gradeAnswer`'s default path — not reached because `gradeObjective` special-cases `multi_answer` before falling through (`exam_grading.go:315-321`) |
| `short` | no options, needs accepted-answer set | Auto-graded, same rule as `fill_blank` — `matchesAnyAccepted` (`gradeAnswer`, `exam_grading.go:30-31`) | Fully active in grading/DB/CSV-parsing — but **not creatable as a new question**: cut from `CREATABLE_FORMATS` in the admin editor ("user decision, 2026-07-31", `web/components/admin/QuestionEditor.tsx:482-487`) because `multi_blank` with one blank covers the same inline-answer case; existing `short` questions stay editable via the fallback that prepends the current format when it's not in `CREATABLE_FORMATS` (`QuestionEditor.tsx:756-760`) |
| `fill_blank` | no options, accepted-answer set | Same as `short` | Same authoring cut as `short` — also dropped from the bank picker's format filter (`ALL_FORMATS`, `web/components/admin/QuestionPickerModal.tsx:36-44`) and never emitted by the CSV import template generator (guarded by `TestBuildQuestionImportTemplate_omitsFormatsTheCSVCannotExpress`, `backend/internal/service/exam_import_test.go:274-289`) — a hand-written import CSV can still carry `format=short`/`fill_blank` and will import fine (`importRowToQuestion`, `backend/internal/service/exam_import.go:266-290`, has no format rejection) |
| `multi_blank` | stem needs `{{N}}` tokens; blanks count must match token count | `gradeMultiBlank` — per-blank partial credit, JSON array answer (`exam_grading.go:124-168`) | |
| `true_false` | needs `question_statement` rows | `gradeTrueFalse` — per-statement partial credit (`exam_grading.go:170-224`) | Answer key (`is_true`) never sent to the client (`SessionStatement`, `exam_session.go:76-79`) |
| `essay` | body only | **Not auto-graded** — `IsCorrect/Score/GradedAt` left nil at submit (`gradeObjective`, `exam_grading.go:277-283`); scored later via `GradeEssayAnswer` (admin) | |

Answer matching rule (FB-10, `backend/internal/service/answer_match.go`): trim → Unicode-lowercase → collapse internal whitespace → exact string equality. No accent folding, no punctuation stripping, no number-word equivalence (pinned by `TestMatchesAnyAccepted_pinsTheConfirmedRule`).

Scoring inputs: `point_correct` (NUMERIC, `> 0`) and `point_wrong` (NUMERIC, `>= 0`) live per-question, with an optional per-option/per-blank/per-statement `points` override (`itemPoints`, `exam_grading.go:53-58`, added by `db/migrations/0050_per_item_points.up.sql`). A session/topic score is floored at 0 (`clampScore`, `exam_grading.go:364-366`) — wrong answers can never push a question or session below zero, but an individual multi-part question's own sub-score is *not* independently clamped before summing.

## Cloze syntax (`multi_blank`)

- Token pattern: `{{N}}` where `N` is a positive integer, parsed by `extractTokensFromStem` (`backend/internal/service/exam.go:151-171`, regex `\{\{(\d+)\}\}`).
- `validateTokenSequence` (`exam.go:173-198`) requires an exact contiguous `1..N` set — no gaps, no duplicates, no starting at 0. Malformed token numbers reject with `ErrValidation`; there is no fallback rendering for a malformed cloze — authoring is rejected outright.
- `question_blank`/blank-count must equal token count (`validateQuestion`, `exam.go:300-302`). No shared-passage/shared-stem primitive exists — each question stands alone; the only reuse mechanism is bank-level (`test_question`), not sub-question composition.
- On the student payload, blanks are exposed as an ordered index list only (`SessionQuestion.Blanks`, `exam_session.go:61,164-169`) — never the accepted answers.

## Rich-text sanitisation

Single tag allowlist, defined once and shared:

- Source of truth: `questionBodyAllowedTags` (`backend/internal/service/exam.go:66`) = `b, i, u, ul, ol, li, sup, sub, img, br, p, table, thead, tbody, tr, td, th`. Applied via `bluemonday` policy `questionBodyPolicy` (`exam.go:68-79`) inside `sanitizeQuestionBody` (`exam.go:87-105`), called at every question/option/statement write path (`CreateBankQuestion`, `SaveQuestion`, `CreateQuestionForTest`, `ProcessQuestionImportRows`, plus `sanitizeQuestionOptions`/`sanitizeQuestionStatements`, `exam.go:130-149`).
- Same list is mirrored in TypeScript as `QUESTION_BODY_ALLOWED_TAGS` (`web/lib/question-html.ts:8`), consumed at 3 frontend sites: `RichTextEditor.tsx` (paste sanitizer), `RichContent.tsx` (admin render), and the exam session page (`app/(exam-session)/exam/sessions/[id]/page.tsx`) — all three call `DOMPurify.sanitize(html, { ALLOWED_TAGS: QUESTION_BODY_ALLOWED_TAGS, ... })`. This is **one shared constant referenced from 4 call sites (1 Go + 3 TS)**, not 4 independent copies — cross-language equality is pinned by a Go↔TS bridge test (`sanitize_bridge_test.go`, invoked from the vitest suite via `SANITIZE_BRIDGE_IN/OUT` env vars) rather than 4 hand-maintained lists.
- Attribute allowlists deliberately differ per site (documented at `exam.go:60-63`): Go allows `src,alt` on `img` and `colspan,rowspan` on `td/th`, plus a restricted `style` subset on `img` (`color, background-color, text-align, font-weight, font-style, text-decoration`) with `url(...)` and any `position:` declaration stripped regardless of value/casing (`exam.go:80-83, 92-98`). `RichContent.tsx`/exam-session page additionally allow `style` in `ALLOWED_ATTR`; `RichTextEditor.tsx`'s paste sanitizer does not (`web/components/admin/RichTextEditor.tsx:196-197`).
- Empty-body check (`isQuestionBodyEmpty`, `exam.go:112-119`) strips tags before judging emptiness so an all-`<br>`/`<p>` body isn't accepted as content, but an `<img>`-only body is.

## Exam access

Three ways a student gets an `exam_registration` row (`exam_registration.status`: `registered | revoked`, plus `checked_in_at`):

1. **Purchase-linked**: order fulfillment inserts via `CreateExamRegistration` from the outbox worker (`backend/internal/worker/outbox.go:235`), not the service layer directly.
2. **Admin grant** (bypasses orders entirely): `Service.GrantExamAccess` (`backend/internal/service/exam_grant.go:33-130`) inserts per-student rows in one tx, `ON CONFLICT DO NOTHING` for already-registered students (silent skip), audit-logged (`exam_grant.create`). A previously-revoked row is revived (status/token/checked_in_at/revoked_at reset) rather than duplicated (`insertExamRegistrationRow`, `exam_grant.go:157-173`).
3. **Revoke** (soft, mirrors `course_session`): `Service.RevokeExamAccess` (`backend/internal/service/exam_revoke.go:53-`) sets `revoked_at`/`revoked_by` (`db/migrations/0064_exam_registration_revoked.up.sql`) — rows are never deleted (certificate/participant-number history must survive). Per-row outcome: no registration → `failed`; already revoked → `skipped`; **session `in_progress` → `failed`, admin must force-submit first**; otherwise → `revoked`.

Server-side enforcement, not just UI-hidden:
- `CheckIn` and `StartSession` both reject `status == "revoked"` explicitly (`ErrRegistrationRevoked`, `exam_session.go:266-268, 353-355`).
- Every session read/write is scoped by student ID **in the SQL itself**: `GetExamSessionForStudent(sessionID, studentID)` is `WHERE id = $1 AND student_id = $2` (`backend/internal/repository/exam.go:2314-2331`) — a session ID alone (e.g. guessed/leaked) cannot be used to read or mutate another student's session; it 404s via `ErrNotFound` → `ErrSessionNotFound`.
- Check-in device lock: a Redis key `exam:device:<registration_id>` binds the first fingerprint (SHA-256 of IP+UA, `fingerprint`, `exam_session.go:119-122`) seen at check-in; `StartSession` rejects a mismatched fingerprint with `ErrDeviceMismatch` (`exam_session.go:384-394`).
- `max_attempts`: `nil` = unlimited, `<= 0` or `1` = single sitting, `>= 2` = that ceiling (`StartSession`, `exam_session.go:405-415`, mirrored in `CreateExamSessionTx`).

## Session lifecycle

State machine on `exam_session.status`: `in_progress → submitted` (terminal; no "expired" status — expiry finalizes into `submitted` the same way a normal submit does, see below).

- **Start** — `StartSession` (`exam_session.go:335-477`). If an `in_progress` session already exists for the registration, it's returned as-is (idempotent resume, not a new attempt) via `GetInProgressSessionForRegistration` (`exam_session.go:397-400`). For `requires_checkin` exams: scheduled-window check, check-in-status check, device-fingerprint check, in that order, before any row is created. For sectioned modes (`utbk`/`ielts`), `CreateExamSessionTx` additionally seeds one `exam_session_section` row per attached test in the same tx — first is `active` with `started_at=now()`, rest `pending` (`exam_session.go:441-470`; `db/migrations/0021_exam_modes.up.sql:11-21`).
- **Position tracking** — `exam_session.current_position` (`db/migrations/0052_exam_session_position.up.sql`) is an optional server-persisted "last question index", validated against `[0, total_questions)` before write (`SaveAnswers`, `exam_session.go:666-674`); returned on reconnect as `CurrentPosition`.
- **Autosave** — `PATCH` → `SaveAnswers` (`exam_session.go:613-687`). Rejects with `ErrAlreadySubmitted` if `status != "in_progress"`. Validates every `question_id` belongs to the exam (`GetQuestionTestMap`) before writing anything. For sectioned exams, an answer targeting a question in a non-`active` section is rejected wholesale with `ErrSectionLocked` (`exam_session.go:649-663`) — standard mode skips this guard entirely. See [[exam-capacity]] for the write path's single round-trip upsert.
  - **Frontend retry has no error-type awareness**: the autosave retry loop in `app/(exam-session)/exam/sessions/[id]/page.tsx:317-330` catches *any* mutation failure and reschedules with capped exponential backoff (`backoffDelayMs`, `web/lib/exam-session-queue.ts:146-148`, capped at `RETRY_MAX_MS` but with **no attempt limit** and no inspection of the error/status code). A permanent rejection — e.g. `ErrAlreadySubmitted` (409, session already finalized elsewhere: admin force-submit, expiry sweep, or the student submitting from a second tab) or `ErrSectionLocked` — is retried forever in the same way as a transient network blip, since nothing here distinguishes them. **Verified against current code, still present at HEAD** — this is not a stale note.
- **Time limit** — server-side authoritative. Standard mode: `computeRemainingSeconds` (`exam_session.go:216-225`) from `started_at + duration_minutes`, extendable by `extended_until`. Sectioned mode: same shape per-section, `computeSectionRemaining` (`exam_session.go:233-242`) — deliberately a separate function from the flat one; a past regression (PR#25, per comment at `exam_session.go:230-232`) came from routing sectioned remaining-time through the exam-level function and got an instant-0 auto-submit. The client only *displays* a countdown from the server-issued `remaining_seconds`; nothing server-side trusts a client-reported elapsed time.
- **Auto-submit on expiry** — background sweep, not client-triggered: `Service.ExpireExamSessions` (`backend/internal/service/exam_expiry.go:18-47`), paged via `ListDueExamExpiryCandidatesPage`. Standard mode: `standardSessionDue` → `finalizeLockedSessionTx` directly submits/grades. Sectioned mode: `sectionDue` on the *active* section → `AdvanceSessionSectionTx`; only finalizes the whole session once there's no next section (`exam_expiry.go:95-136`). Grace window (`exam.grace_window_minutes`) extends the deadline before a session is considered due (`expiryDeadline`, `exam_expiry.go:153-162`). Confirmed exercised by `backend/integration/sweeper_test.go`.
- **Submit** — student path `SubmitSession` (`exam_session.go:790-817`) re-locks the session row `FOR UPDATE` inside a fresh tx (`finalizeSession`/`finalizeLockedSessionTx`, `exam_session.go:819-886`) and re-checks `status == "in_progress"` under the lock — a second concurrent submit/expiry/force-submit gets `ErrAlreadySubmitted`, not a double grade. Grading reads answers **from the DB inside the same tx** (`GetSessionAnswersTx`), never from the request body — the submit request carries no answer or score payload at all (`Handler.StudentSubmitSession`, `backend/internal/handler/exam.go:739-750`, takes only the session ID from the URL). Certificate generation is enqueued via outbox in the same tx (`exam_session.go:878`).
- **Admin overrides**: `ReopenSession` (extends `extended_until` or the active section's, `exam_session.go:929-970`) and `ForceSubmitSession` (`exam_session.go:975-998`, same finalize path, `adminSubmitted=true`) — both require an `in_progress` session.
- **Resume**: any `in_progress` session is resumable via `ReconnectSession` (`exam_session.go:527-601`), returning full state (tests, saved answers, current position, remaining time). Once `status == "submitted"` there is no resume path — `SaveAnswers`/`AdvanceSection`/`LogViolation` all reject a non-`in_progress` session with `ErrAlreadySubmitted`.

## Scoring authority

- **Server-only, always**. No handler or service function for save/submit/reconnect accepts a client-supplied score, correctness flag, or points value — `AnswerInput` (`exam_session.go:112-116`) carries only `question_id`, `answer`, `flagged_for_review`. Grading (`gradeObjective`, `exam_grading.go:269-360`) runs entirely from DB-loaded question definitions and DB-loaded submitted answers, both read inside the finalize transaction.
- The one place a numeric score *is* client-influenced is admin essay grading — `GradeEssayAnswer` (`exam_result.go:232-286`) takes an admin-supplied `score`, but only for `essay` questions, only from an admin actor, and range-checked against the question's `point_correct` (`validateGrade`, `exam_result.go:222-227`) before being folded into `computeSessionTotal` and re-clamped at 0.

## Results visibility

Three-gate precedence in `resultGate` (`exam_result.go:111-126`), evaluated in `GetSessionResult` (`exam_result.go:23-109`), ownership enforced the same way as session access (`GetExamSessionForStudent`):

1. `exam.result_config == "hidden"` → state `hidden`, nothing else computed.
2. Not yet submitted, or submitted but not fully graded (any `essay` answer missing `graded_at`, `isFullyGraded`, `exam_result.go:129-140`) → state `grading`.
3. `exam.result_release_at` set and still in the future → state `locked` (carries `result_release_at` so the client can show a countdown).
4. Otherwise → state `result`: score, correct/wrong/empty counts (`objectiveCounts`, `exam_grading.go:421-443`, essays excluded from these counts), rank (`computeRank` — 1-based, ties share a rank, `exam_grading.go:381-385`). `result_config == "score_pembahasan"` additionally attaches per-topic breakdown (`topicBreakdown`) and a full answer review (`buildPembahasan`) — plain `"score"` config gets score/rank only, no per-question review.
- Certificate URL resolution happens before the gate check so it's present even in `hidden`/`grading`/`locked` states once a session is submitted — see [[certificate-rendering]].

## Proctoring / device integrity — logging, not enforcement

- **Fullscreen is a client-side soft gate, not server-enforced.** The exam session page blocks rendering behind a "grant fullscreen" screen (`fullscreenGranted` state, `app/(exam-session)/exam/sessions/[id]/page.tsx:145,991-998`) and attempts silent re-entry into fullscreen on exit via a keyboard/gesture-triggered retry (`requestKeyboardFullscreenRecovery`, lines ~749-787). Nothing server-side checks whether the client was ever in fullscreen.
- **Violations are recorded, not acted on.** `validViolationTypes = {fullscreen_exit, tab_switch, copy_attempt}` (`exam_session.go:125-129`). `LogViolation` (`exam_session.go:891-923`) only inserts a `session_violation_log` row after confirming the session is still `in_progress` — it has no side effect on the session, score, or status. There is no auto-submit-on-violation, no violation-count threshold, no webcam/device-integrity check anywhere in this codebase. Frontend triggers: `fullscreenchange` and `visibilitychange` listeners call `logViolation.mutate(...)` (page.tsx:770-817); copy events likewise (`onCopy`, page.tsx:817).
- Admin visibility only: `GetSessionMonitor` (`exam_session.go:1068-1124`) surfaces recent violations per exam (`GetRecentViolations`, capped at 20) and derives an `overdue` status from the deadline for the proctor UI; `GetSessionViolations` (`exam_session.go:1129-1135`) lists a single session's full log. Nothing here blocks or auto-submits — a human has to act on it (e.g. via `ForceSubmitSession`).
- `docs/backlog/exam-format-system-design.md` proposes richer proctoring (unverified against current code) — treat as a design proposal only, not implemented behavior.

## Unverified / not checked in this pass

- Actual PgBouncer/Postgres tuning values referenced by the write path (see [[exam-capacity]] for that angle) were not re-verified here.
- Resolved, not unverified: a repo-wide grep (`grep -rn "gradeMultiAnswer\|gradeAnswer(" --include="*.go" .`, non-test files) confirms `gradeAnswer`/`gradeMultiAnswer` have no caller outside `exam_grading.go`. `gradeObjective` always intercepts `format == "multi_answer"` at `exam_grading.go:315` and `continue`s before reaching the `gradeAnswer` fallthrough at line 334, so `gradeMultiAnswer`'s binary all-or-nothing rule is dead code in every runtime path, not just the one checked before.
