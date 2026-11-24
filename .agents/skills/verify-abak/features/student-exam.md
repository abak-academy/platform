# Student competitions and exams

Students see competitions and tryouts at `/exam` (`Kompetisi & Tryout` / `Competition & Tryout`), with `Paket gratis` / `Free packages` and `Paket saya` / `My packages`. They start a session at `/exam/sessions/[id]`, download an exam card at `/exam/[id]/card`, and see the result and leaderboard after submitting.

## Sub-features

- `exam-list` lists free and owned packages.
- `exam-start` starts a session from a package and lands in `/exam/sessions/<session_id>`.
- `exam-answer-submit` answers questions, then submits on the last question.
- `exam-result` shows the score at `/exam/sessions/<id>/result` and the leaderboard.
- `exam-card` produces the exam-card PDF through Gotenberg (`:53001`) and a presigned MinIO URL (`:59000`).

## How to get to it (user POV)

- The student nav item for competitions, or open `/exam` directly.
- From a package, start, answer, and submit.
- From a registration, open its exam card.

## Driving it with abak-verify

Preconditions:

- `abak-verify doctor` prints `OK`, signed in as `verify-student@example.test`.
- A published package with a test and questions, free or owned by the student. The fresh DB has none, so build it as the admin first (see [admin-test-authoring](./admin-test-authoring.md)).

- **List.** Open `/exam` and expect the `Kompetisi & Tryout` heading and the package under `Paket gratis`.
- **Start.** Start the package. Expect the URL `/exam/sessions/<id>` and the first question.
- **Submit.** Answer every question and press submit on the last one. Expect the result page with a score. Second view: the admin results workspace for that exam lists the attempt.
- **Card.** Open the exam card. A PDF downloads from a `localhost:59000` presigned URL. Save it in the evidence dir and look at it, because layout needs visual proof.

## Gotchas

- Autosave retries a permanent 409 forever and drops answers silently. Assert the result score, not only that submit succeeded.
- Google Translate in the browser crashed the session page (last-question Next→Submit swap). Never run journeys with page translation on.
- `web/e2e/exam-student-flow.spec.ts` and `composable-exam-session.spec.ts` show working selectors for this flow.
