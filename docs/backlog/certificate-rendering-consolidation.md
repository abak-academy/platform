# Backlog: consolidate certificate rendering into one implementation

**Raised:** 2026-07-26 · **Status:** ▶ **CHOSEN AS NEXT WORK, 2026-07-30** (was: accepted as tech debt,
deliberately not scheduled)

> **Why now, in the user's words:** *"sangat tidak enak melihat kita maintain 2 source html dan di
> generate di backend."* That is this document's premise, and it is the right reason — the duplication is
> a standing correctness risk, not a tidiness complaint.
>
> **D-1's certificate slice is folded in here** — see [Folded-in scope](#folded-in-scope-from-d-1) below.
> The rest of D-1 stays in [E1](e1-foundation-unblock.md).

## The problem

The certificate visual is implemented **twice**:

1. **In the browser** — the admin drag-and-drop design editor renders it live, loading
   `web/public/fonts`. *(Updated 2026-07-29: the studio rebuild in `211b7b1` deleted
   `CertificateFonts.module.css`; the `@font-face` declarations moved into the exam-package layout at
   `web/app/(admin)/admin/exam/packages/[id]/layout.tsx`. The duplication itself is unchanged — the
   font files still exist twice.)*
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

## Folded-in scope from D-1

**Decided 2026-07-30.** D-1 (the storage/repository seam) is not taken as a whole. Its **certificate
slice** moves here, because this work rewrites those exact files anyway and the seam is already the thing
blocking the adapter move.

### Why the overlap is real, not manufactured

The "Related" note below always said the `pdf_generator.go` → `internal/adapter/` move was *"blocked only
by `certificate_integration_test.go` … constructing the concrete generator directly."* **That
understated it.** Verified on `main` 2026-07-30 — six construction sites inside `package service`:

| File | Sites |
|---|---|
| `pdf_generator_test.go` | `:60`, `:107`, `:137`, `:162` |
| `certificate_integration_test.go` | `:84`, `:184` |

Plus production wiring at `service.go:96`. **[PR #63](https://github.com/abak-academy/platform/pull/63)
adds a seventh** (`certificate_render_gotenberg_test.go`), which is an honest cost of that PR: the render
proof was written the same way as everything around it, so it grew the blocker rather than avoiding it.

Every one of those sites exists *because* there is no `pdfGenerator` seam at the constructor. The
interface is already declared in `pdf_generator.go`; nothing injects it.

### In scope here

- Inject `pdfGenerator` at `NewService` instead of constructing it inside. That alone unblocks the
  `internal/adapter/` move and removes the reason those seven sites reach for the concrete type.
- The shims in `certificate_test.go` (3 fake methods) and whatever `exam_result_test.go` needs for the
  certificate path — **only** what this rewrite touches.
- Keep the **`render-gate` CI job** working throughout. It is a real job
  (`.github/workflows/pipeline.yml:19` → `deploy/pipeline/backend-render-gate.sh`) and the `images` job
  `needs:` it, so breaking it blocks every image build.

### Explicitly NOT in scope here

- **The other ~50 shim methods** — `course_test.go`, `store_test.go`, `auth_test.go`,
  `announcement_test.go`, `exam_session_test.go`. Nothing about certificate rendering touches them.
- The `storeRepo *repository.Repository` seam. Bigger than the storage one and unrelated to this path.

> **This respects E1's rule rather than breaking it.** E1 says *"do not graft either onto a feature branch
> — mixing a mechanical refactor with behavioural change makes both unreviewable."* Folding all 62 shims
> in would be exactly that mistake. **Sequence it as two commits, seam first, behaviour second**, so the
> mechanical change is reviewable on its own and the consolidation diff stays about rendering.

### Prerequisite

**[PR #63](https://github.com/abak-academy/platform/pull/63) and
[PR #64](https://github.com/abak-academy/platform/pull/64) must both merge first.** Both are open and both
touch `internal/service`; #63 also changes `certificate.go`, `certificate_layout.go` and
`certificate_test.go` — the files this work rewrites.

## Related

- `docs/runbooks/postgres-16-to-17.md` — unrelated, but the same "verify the layer you did not
  change" lesson applies.
- The Gotenberg client lives at `internal/service/pdf_generator.go`; moving it to `internal/adapter/`
  is now part of the scope above rather than a separate cleanup.
- [`gotenberg-render-proof-local.md` technique](e1-foundation-unblock.md) — the render loop this work
  needs: local Gotenberg on host port 3001, an env-gated in-package test, then **read the PDF back and
  look at it**. Byte assertions cannot catch a layout regression, which is the entire risk here.
