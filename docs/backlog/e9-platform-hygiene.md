# E9 — Platform hygiene

| | |
|---|---|
| **Issue** | [#92](https://github.com/abak-academy/platform/issues/92) |
| **Objective** | Deploys stop being hand-typed, CI stops burning double, and the dead weight the codebase is still carrying comes out. |
| **Source IDs** | F-5, D-5, D-7, D-3 (`invoice_url`) + the infra/CI items and the orphaned `school/classes` stub |
| **Client items** | none |
| **Depends on** | — |
| **Verified against** | `main` @ `d2efa3a`, 2026-07-30 |

> **Not a vertical slice** — no schema, no UI, nothing the client sees. Like the old E1 foundation cleanup,
> it earns its place by what it stops costing elsewhere. Do not try to force a UI deliverable into it.

Items are independent; take them in any order. F-5 is the only one with a compounding cost.

---

## 1. F-5 — Continuous deployment

Still a manual `IMAGE_TAG` edit followed by `pull && up -d` on both VMs. WIF now exists, so the
WIF + IAP path is far cheaper than when this was parked.

**More urgent than its priority suggests.** Production has repeatedly trailed `main` by several
commits, and E5 §4 is the worked example: FB-15 was reported as broken when
the fix was already merged. Until F-5 lands, **every "still broken" report carries that ambiguity**,
and the cost is paid in trust at every demo — first action on any such report has to be "deploy and
re-check", which is a whole round-trip before the work even starts.

Watch the traps already recorded in the runbooks: compose interpolates `${IMAGE_TAG:?}` across the
whole file, and the staging VM has no git clone — its compose file is hand-edited in place.

---

## 2. CI — `pipeline.yml` runs twice per push

A bare `on: push` **and** `on: pull_request`, neither filtered
([`.github/workflows/pipeline.yml:3-5`](../../.github/workflows/pipeline.yml)):

```yaml
on:
  push:
  pull_request:
```

~13 min each. Free while the repo is public; **the single largest minute drain the day it isn't.**

One-commit fix: `push: branches: [main]` plus a `concurrency` group with `cancel-in-progress`.

> Do **not** path-filter the image build. `build-image.sh` publishes api, worker and web under one
> SHA, and the compose files deploy all three from a single `IMAGE_TAG` — skipping one on a
> path filter produces a tag that cannot be deployed.

---

## 3. Infra loose ends

- **The repo is still public** (`abak-academy/platform`). Going private breaks the staging VM's
  `docker login ghcr.io` and starts metering Actions — so item 2 above should land first.
  Runbook: [`../runbooks/repo-migration-to-client-org.md`](../runbooks/repo-migration-to-client-org.md).
- **`deploy/compose/staging.yml` in the repo is deliberately stale** — the VM's copy is hand-edited and never
  pulled, and the repo copy's GHCR paths still point at the old org. Either reconcile it or mark it
  clearly, because it currently reads as configuration and is not.
- **The empty `default` VPC network should be deleted** so nobody launches a VM into it by accident.

---

## 4. Dead weight

| ID | Item | Detail |
|---|---|---|
| **D-7** | Fazpass is unused | OTP goes over SMTP. Still referenced in `cmd/api/main.go`, `cmd/worker/main.go`, `config/config.go` + `config_test.go`, [`internal/adapter/fazpass.go`](../../backend/internal/adapter/fazpass.go), `internal/adapter/notify_test.go`, `internal/service/ports_notify.go`, and **six** config/secrets files across dev, staging, prod and the two examples. Delete the adapter and the config surface together — a half-removal leaves a config key nothing reads. |
| — | Orphaned `school/classes` stub | [`web/app/(admin)/admin/school/classes/page.tsx`](../../web/app/(admin)/admin/school/classes/page.tsx). Created 2026-06-19 in `d371213` as coming-soon scaffolding, orphaned 2026-07-05 when `ee9bc6f` reverted the `feat/school-slice-a` merge (PR #18). A later commit titled *"remove last ComingSoon stub"* missed it — it has no nav entry and is unreachable in the UI. **Not a commitment to the client.** One file plus six dead i18n lines. |
| **D-3** | `orders.invoice_url` | **Keep — do not drop.** Half-wired rather than dead: no producer anywhere, but the column is selected ([`repository/order.go:43`](../../backend/internal/repository/order.go)), modelled ([`model/order.go:28`](../../backend/internal/model/order.go)) and *rendered* — [`orders/[id]/page.tsx:190`](../../web/app/(student)/orders/[id]/page.tsx) pushes an "order_invoice" row whenever it is non-empty. So the display side is already built and waiting for something to write it. |

---

## 5. D-5 — no gofmt gate

**37 files** under `backend/` fail `gofmt -l` as of `d2efa3a`. The count grows with every epic that
touches Go.

Adding the gate means formatting all 37 in **one commit first**, or the gate lands red. Do that commit
on its own, touching nothing else — a formatting sweep mixed into feature work is unreviewable.

---

## Acceptance

- A merge to `main` deploys to staging without anyone editing `IMAGE_TAG` by hand.
- One pipeline run per push to `main`; a second push cancels the first.
- `grep -ri fazpass backend/ deploy/` returns nothing outside coverage artefacts, and the api and
  worker still boot with the config files as shipped.
- `gofmt -l backend/` is empty and CI fails when it isn't.
- `school/classes/page.tsx` and its i18n keys are gone; no other route 404s.

## Out of scope

- Dropping `orders.invoice_url`. Explicitly retained above.
- Any change to what the compose files deploy, beyond automating who edits them.
