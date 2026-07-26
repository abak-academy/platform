# Backlog: consolidate certificate rendering into one implementation

**Raised:** 2026-07-26 · **Status:** accepted as tech debt, deliberately not scheduled

## The problem

The certificate visual is implemented **twice**:

1. **In the browser** — the admin drag-and-drop design editor renders it live, loading
   `web/public/fonts` through `web/components/admin/CertificateFonts.module.css`.
2. **In Go** — `internal/service/certificate_html.go` builds self-contained HTML, embedding the
   same font files (`//go:embed fonts`, base64 into `@font-face`) for Gotenberg to turn into PDF.

Two renderers that must agree pixel-for-pixel, with no test comparing one against the other. The
duplicated font directories are the visible symptom; the real exposure is that the editor can preview
a layout the PDF does not reproduce, and nothing fails.

This project has already been burned by the same class of bug — see the note on a certificate that
rendered fully upside-down while byte-level assertions stayed green.

## Why it was not fixed now

The obvious fix is to let the frontend own the markup and have Gotenberg fetch a print route. That is
a legitimate architecture, and it would collapse both renderers into one and delete the font
duplication outright. It was rejected **for timing, not for merit**:

- The certificate page carries student data, so Gotenberg's Chromium would need a signed, short-lived
  URL or a token passed into the browser. That is a new auth surface to design, not a refactor.
- Certificates are generated synchronously inside an API request (`exam_result.go` →
  `resolveCertificateURL`). Routing through the web app makes that path fail whenever the web
  container is down.
- The Gotenberg work had just merged, had never been deployed to staging, and had never been visually
  verified. Rewriting it in the same window as a repository migration would stack three unverified
  changes.

## Mitigation in place

`internal/service/fonts_parity_test.go` fails if the embedded font set and the browser-served set
diverge by name or by content. That closes the silent-drift half of the risk. It does **not** close
the layout half: the two renderers can still disagree about positioning.

## What a real fix needs

- A single source of truth for layout → CSS. Today `certificate_layout.go` holds the model
  (`Layout`, `LayoutField`, page/background config, persisted as JSONB) and both renderers interpret
  it independently.
- Either the FE-owned print route above, or a shared template compiled into both — the second is hard
  across Go and TypeScript.
- A gate that renders the same layout through both paths and compares. Without it, any consolidation
  is unverifiable.

## Related

- `docs/runbooks/postgres-16-to-17.md` — unrelated, but the same "verify the layer you did not
  change" lesson applies.
- The Gotenberg client now lives at `internal/service/pdf_generator.go`. Moving it to
  `internal/adapter/` alongside the other external-service clients is a separate, smaller cleanup —
  blocked only by `certificate_integration_test.go` living in `package service` and constructing the
  concrete generator directly, which the render-gate CI job depends on.
