---
title: Certificate Rendering
updated: 2026-09-25
verified_against: e6cf227c
---

Related: [[system-overview]] · [[exam-lifecycle-and-scoring]]

## Trigger — when a certificate is issued

Async, event-driven — never inline in a request handler.

- `SubmitSession`, `ForceSubmitSession`, and `GradeEssayAnswer` each insert a `CertificateNeeded`
  outbox event in the same transaction as the state change
  (`backend/internal/service/certificate.go:455-464`).
- The worker's `handleCertificateNeeded` (`backend/internal/worker/certificate.go:32-59`) calls
  `Service.GenerateCertificatePDF` and marks the outbox event processed only after a successful
  (possibly no-op) call — the render/upload/persist is **not** inside the same DB transaction as
  `MarkOutboxProcessed` (a Gotenberg round trip shouldn't hold a Postgres tx open).
- `GenerateCertificatePDF` (`certificate.go:480-577`) is documented as **the only place** a
  certificate is ever generated, and is idempotent: it re-derives readiness from current DB state
  every call and no-ops when the session isn't submitted, certificates are disabled, no template is
  saved, a score-bearing layout is still awaiting grading, or the cached PDF is already current
  (`certificateNeedsRegeneration`, line 449). Regeneration triggers: no cached key yet, a re-grade
  landed after the cached PDF's timestamp, or an admin saved a design change after it
  (`exam.certificate_design_updated_at`).

## Certificate numbering

- Format: `ABK/<year>/<exam_number:%04d>/<participant_number:%06d>`, composed in Go
  (`backend/internal/repository/exam.go:2895`, `AllocateCertificateNumber`). Year comes from
  `exam.scheduled_at` (Asia/Jakarta) when set, else `time.Now()`.
- **One number per registration, not per session/attempt** (fixed 2026-08-28, see
  `docs/backlog/certificate-number-collision-23505.md`). Resolution order: (1) session already has a
  number → return it; (2) a sibling session of the same `registration_id` already has one → reuse it,
  no compose; (3) compose + guarded `UPDATE ... WHERE certificate_number IS NULL`; (4) a `23505`
  unique-violation with no sibling number falls through and is surfaced as an error (only reachable
  via legacy pre-migration-0041 rows).
- Uniqueness is a **partial unique index**, not a sequence: `idx_exam_session_certificate_number ON
  exam_session(certificate_number) WHERE certificate_number IS NOT NULL`
  (`backend/db/migrations/0035_certificate_design.up.sql`). The paired `certificate_number_seq`
  sequence was dropped in `0041_drop_certificate_number_seq.up.sql` — "numbers are now composed in
  Go; no global sequence is consumed."
- Consequence: the number is *persisted* on only the one session row that composed it first; sibling
  (retake) sessions keep `certificate_number = NULL` and read the winner's number via reuse, but every
  sibling's own rendered PDF still shows the shared number. A per-row number would need the unique
  index re-scoped (e.g. `(exam_id, certificate_number)`); not done.

## Template / design model

- `exam.certificate_design` — a single JSONB blob (migration `0042_certificate_design.up.sql`
  consolidated the earlier separate `certificate_template` / `certificate_background_key` /
  `certificate_layout` columns into it). Decoded shape: `Template`, `Background`, `Fields`
  (`backend/internal/service/certificate_layout.go`).
- `exam.certificate_template_html` (migration `0056_certificate_template_html.up.sql`) — **FE-authored,
  self-contained HTML** with `{{token}}` placeholders, the closed set in `validCertificateTokens`
  (`certificate_layout.go:60-64`: `student_name`, `exam_title`, `completion_date`,
  `certificate_number`, `score`, `max_score`, `score_percent`, `rank`, `percentile`, `duration`,
  `total_questions`). The admin design editor (`web/components/admin/CertificateDesignTab.tsx`,
  `web/components/certificate/CertificateDocument*.tsx`) serializes the studio's layout to this markup
  once per design save (fonts/assets embedded), and `AdminUpdateExamCertificateDesign`
  (`backend/internal/handler/exam.go:1054`) persists it via `ValidateCertificateTemplateHTML`
  (sanitized with `bluemonday`) alongside the design JSON. **No Go-side HTML builder exists for the
  certificate body** — the worker only substitutes verified values into the saved template.
- Layout coordinate contract: millimetres, origin top-left, Y increasing downward, on an A4-landscape
  page (297×210mm) — "no flip anywhere" (`certificate_layout.go:1-8`). This exists because of a prior
  incident: a certificate rendered fully upside-down while byte-level tests stayed green.
- Card model is separate and simpler: the exam card has exactly **one static template**
  (`backend/internal/service/assets/exam_card_template.html`, embedded), generated build-time from
  `web/components/exam/ExamCardPrintable.tsx` via `npm run build:card-template`
  (`backend/internal/service/card_generate.go:1-24`) — no per-exam admin design for cards.

## PDF render flow

**One hop, not two.** Both certificate and card render by substituting values into a self-contained
HTML string in Go, then POSTing that HTML directly to Gotenberg — Gotenberg never fetches a URL.

1. `GenerateCertificatePDF` builds the token map (`buildCertificateValues`), resolves the background
   as a presigned-GET URL (custom upload) or an inlined `data:` URI (built-in), presigns any uploaded
   image-field assets, then calls `substituteTemplateTokens(*exam.CertificateTemplateHTML, values)`
   (`certificate.go:563`). Any `{{token}}` not in the trusted value map is blanked, not left literal —
   "the FE-authored template is untrusted input from the worker's point of view" (`certificate.go:426-430`).
2. `s.renderer.RenderHTML(ctx, []byte(html))` → `gotenbergPDFGenerator.RenderHTML`
   (`pdf_generator.go:65-88`) builds a `multipart/form-data` body (`files=index.html`) and posts to
   Gotenberg's `/forms/chromium/convert/html` via `net/http` + `mime/multipart` directly (no client SDK).
3. Shared page options across every Chromium route: `printBackground=true`, `preferCssPageSize=true`,
   all four margins `0` (`pageOptionFields`, `pdf_generator.go:56-63`) — **the config that must
   agree**: certificate and card both depend on `preferCssPageSize` so the page's own `@page` CSS
   size wins.
4. A 30s timeout (`defaultGotenbergTimeout`) is applied as a context deadline around the whole
   request+read, since `http.DefaultClient` (the production injection) has none of its own.

The card path (`generateExamCardPDF`, `card_generate.go:118-125`) is structurally identical.

**History note:** an earlier design (in older wiki sources) planned to have Gotenberg fetch a Next.js
print route behind a short-lived token instead of receiving posted HTML. The shipped code took the
opposite path and says so explicitly — `backend-render-gate.sh:6-11`: *"Nothing here asks Gotenberg
to fetch a URL... no `api`, `web`, or Redis container [needed]."* `web/app/(print)/exam/[id]/card/page.tsx`
is a **browser-facing** `window.print()` view for the student's own on-screen download, unrelated to
the server-side Gotenberg render — never fetched by Gotenberg.

## Preview path (admin studio)

`GetCertificatePreviewPDF` (`certificate.go:588-599`) is a thin pass-through: the FE has already
serialized the in-progress (possibly unsaved) design to self-contained HTML with placeholder preview
values baked in (`web/app/api/admin/certificate-template/route.tsx`, `PREVIEW_VALUES`) — the handler
only 404s an unknown exam id, then calls `RenderHTML` directly. No DB write, no object-store write, no
token substitution — a different code path from `GenerateCertificatePDF`, so a preview and a real
render can diverge if the two builders drift.

## Storage of rendered PDFs

- Object key: `certificates/<sessionID>.pdf` in the private bucket `s.cfg.ObjectStorageBucketName`
  (`uploadCertificatePDF`, `certificate.go:104-121`). One object per **session** (not per
  registration) — every retake gets its own rendered PDF even though siblings share one certificate
  *number*.
- `exam_session.certificate_key` (renamed from `certificate_url` in migration
  `0040_rename_certificate_key.up.sql` — "stores a private-bucket object KEY, not a URL") plus
  `certificate_generated_at` cache the object key and the render timestamp
  (`UpdateSessionCertificate`, called at `certificate.go:573`).
- The bucket is private; every read is a presigned GET, TTL 15 minutes
  (`presignedDocumentURLTTL`, `backend/internal/service/exam.go:1653`).

## Access / "verification" — there is no public lookup

There is no public certificate-verification or lookup-by-number endpoint. The only way to obtain the
PDF is the presigned URL surfaced on the exam session result payload:

- `resolveCertificateURL` (`certificate.go:379-416`) is **read-only** — never renders, uploads, or
  persists. Returns `(nil, nil)`, not an error, whenever the session isn't submitted, certificates
  aren't enabled, or nothing has been generated yet; callers cannot distinguish "not generated" from
  "not visible," by design.
- The authorization gate (`certificateLayoutAllowed` → `resultGate`) is re-evaluated on **every**
  access, never cached alongside the artifact — the fix for the historical "#55" bug where a cached
  PDF outlived the decision that authorized it. `certificateIsSensitive` (`certificate_layout.go:625-642`)
  checks both the layout's declared fields *and* the persisted template HTML's tokens, so a template
  can't smuggle a `{{score}}` token past a layout that never declared a score field.
- Surfaced field: `model.SessionResult.CertificateURL *string`, intentionally **without**
  `omitempty` (`backend/internal/model/exam.go:451-455`) so the key is present (possibly `null`) on
  every result response regardless of gate state.
- Admin-only endpoints for design/enablement (`backend/internal/server/routes.go:293-298`):
  `POST /:id/certificate-preview`, `GET`/`PUT /:id/certificate-design`,
  `PATCH /:id/certificate-enabled`, `POST /:id/certificate-assets/presign`.

## Known limitations

- No per-row certificate number: retake siblings share one number but each keeps its own rendered
  PDF and `certificate_key`/`certificate_generated_at` — re-scoping the unique index would be needed
  to change this (see Numbering above).
- Preview (`GetCertificatePreviewPDF`) and real generation (`GenerateCertificatePDF`) are independent
  code paths that both end at `RenderHTML`; nothing tests that a saved preview and the eventual real
  render match byte-for-byte, only that both consume the same page options.
- `certificate_enabled` defaults `false` per exam (`0051_exam_certificate_enabled.up.sql`) —
  fail-closed; a new or migrated exam issues no certificates until an admin explicitly enables it.
- Storage-pressure failures surface as an ordinary `upload certificate pdf: ...` error and leave the
  `CertificateNeeded` event unprocessed to retry — forever, with no backoff/attempt cap, since the
  outbox failure path isn't implemented (`outbox.attempts`/`last_error` columns exist, nothing writes
  them; open issue B in `docs/backlog/certificate-number-collision-23505.md`).
- No automated byte/visual regression test compares the admin studio's live preview against the
  actual Gotenberg render pixel-for-pixel. `Unverified`: whether any automated visual-diff step runs
  in CI today, versus a manual render-and-look loop.

## How to verify a render change

`deploy/pipeline/backend-render-gate.sh` is a real CI gate (the `images` job depends on it):

```bash
docker compose -f deploy/compose/local.yml up -d postgres minio createbuckets gotenberg
cd backend
go test -tags gotenberg_integration \
  -run 'TestCertificateRender_|TestCardRender_' -count=1 -v ./internal/service/
```

Runs against a **real** Gotenberg sidecar (not the fake renderer the unit suite uses); needs only
Postgres, MinIO, and Gotenberg (no `api`/`web`/Redis) since nothing is fetched by URL. Current gate
tests (`backend/internal/service/certificate_render_gate_test.go`):
`TestCertificateRender_ThroughWorker`, `TestCertificateRender_ThroughWorker_NoOpWhenDisabled`,
`TestCertificateRender_DesignChangeRegeneratesThroughWorker`, `TestCardRender_ThroughWorker`. The
gate matches by prefix, not exact name, so a new gate test is picked up automatically.
