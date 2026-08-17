# E9 — Platform hygiene

| | |
|---|---|
| **Issue** | [#92](https://github.com/abak-academy/platform/issues/92) — filed 2026-08-13 |
| **Objective** | Deploys stop being hand-typed, CI stops building an image nobody can use, and the dead weight the codebase is still carrying comes out. |
| **Source IDs** | F-5, D-5, D-3 (`invoice_url`) + the infra/CI items and the orphaned `school/classes` stub |
| **Client items** | none |
| **Depends on** | — |
| **Verified against** | `main` @ `3d0d8a1`, 2026-08-17 |

> ## Changed 2026-08-17
>
> - **§2 (CI runs twice) is DONE** on `main` — see below. This doc claimed it was open for two weeks after it was fixed.
> - **D-7 (Fazpass) moved to [#103](https://github.com/abak-academy/platform/issues/103)**, not dropped: its file list is the same one the SES cutover and the reset-password work touch.
> - **Staging was deleted** (VM, disk, 15 snapshots, bucket, IP) and the decision is **prod-only, permanent**. That rewrites §1, §3 and §6 and unblocks the repo-private move.
> - **The `web` double build shrank from a refactor to a deletion** — with one environment left, `images-web` publishes an image pointing at a dead host. See §6, new.

> **Not a vertical slice** — no schema, no UI, nothing the client sees. Like [E1](e1-foundation-unblock.md)
> it earns its place by what it stops costing elsewhere. Do not try to force a UI deliverable into it.

Items are independent; take them in any order. F-5 is the only one with a compounding cost.

---

## 1. F-5 — Continuous deployment

Still a manual `IMAGE_TAG` edit followed by `pull && up -d` on both VMs. WIF now exists, so the
WIF + IAP path is far cheaper than when this was parked.

**More urgent than its priority suggests.** Production has repeatedly trailed `main` by several
commits, and [E5](e5-orders-payments.md) §4 is the worked example: FB-15 was reported as broken when
the fix was already merged. Until F-5 lands, **every "still broken" report carries that ambiguity**,
and the cost is paid in trust at every demo — first action on any such report has to be "deploy and
re-check", which is a whole round-trip before the work even starts.

**Prod-only since 2026-08-17.** The old acceptance ("a merge deploys to staging") has no target left;
the destination is production, which makes the manual-gate question real — deploying to prod on every
merge is a decision, not a default. A one-click approved deploy still removes the SSH-and-edit step.

Watch the traps already recorded in the runbooks: compose interpolates `${IMAGE_TAG:?}` across the
whole file, and reaching the VM needs `gcloud compute ssh --tunnel-through-iap` — direct port 22 is
blocked on Saiful's network.

---

## 2. ~~CI — `pipeline.yml` runs twice per push~~ — DONE

Fixed on `main`: `push: branches: [main]` plus a `concurrency` group. `cancel-in-progress` is
deliberately **off for `main`** — runs there publish images to two registries from three jobs, and a
cancelled one leaves a partial tag set.

> Do **not** path-filter the image build. `build-image.sh` publishes api, worker and web under one
> SHA, and the compose files deploy all three from a single `IMAGE_TAG` — skipping one on a
> path filter produces a tag that cannot be deployed.

---

## 3. Infra loose ends

- **The repo is still public** (`abak-academy/platform`) — but **nothing blocks the move any more.**
  What blocked it was the staging VM's `docker login ghcr.io` breaking; that VM is gone. Only Actions
  metering is left, and item 2 already halved that.
  Runbook: [`../runbooks/repo-migration-to-client-org.md`](../runbooks/repo-migration-to-client-org.md).
- **Delete the staging leftovers.** `deploy/compose/staging.yml` and `deploy/nginx/staging.conf` read
  as configuration and are not — there is no VM behind either. Same for
  `backend/config/env/staging/config.yaml` and `deploy/secrets/staging-secrets.example.yaml`, which
  overlap D-7's file list in [#103](https://github.com/abak-academy/platform/issues/103); whichever
  lands first shrinks the other.
- **The dangling Cloudflare A record for `stg.abakacademy.id`** still points at a released IP. Left as-is
  during teardown on purpose; remove it here.
- **The empty `default` VPC network should be deleted** so nobody launches a VM into it by accident.

---

## 4. Dead weight

| ID | Item | Detail |
|---|---|---|
| **D-7** | ~~Fazpass is unused~~ | **Moved to [#103](https://github.com/abak-academy/platform/issues/103) on 2026-08-17.** Not done — re-homed. It touches `newNotifyProviders`, `adapter/smtp.go` and the same six config/secrets files as the SES cutover and the reset-password work, so splitting it across two epics guaranteed a collision. |
| — | Orphaned `school/classes` stub | [`web/app/(admin)/admin/school/classes/page.tsx`](../../web/app/(admin)/admin/school/classes/page.tsx). Created 2026-06-19 in `d371213` as coming-soon scaffolding, orphaned 2026-07-05 when `ee9bc6f` reverted the `feat/school-slice-a` merge (PR #18). A later commit titled *"remove last ComingSoon stub"* missed it — it has no nav entry and is unreachable in the UI. **Not a commitment to the client.** One file plus six dead i18n lines. |
| **D-3** | `orders.invoice_url` | **Keep — do not drop.** Half-wired rather than dead: no producer anywhere, but the column is selected ([`repository/order.go:43`](../../backend/internal/repository/order.go)), modelled ([`model/order.go:28`](../../backend/internal/model/order.go)) and *rendered* — [`orders/[id]/page.tsx:190`](../../web/app/(student)/orders/[id]/page.tsx) pushes an "order_invoice" row whenever it is non-empty. So the display side is already built and waiting for something to write it. |

---

## 5. D-5 — no gofmt gate

**34 files** under `backend/` fail `gofmt -l` as of `3d0d8a1` (37 at `d2efa3a` — the drop is incidental
fixes, not a sweep). The count grows with every epic that touches Go.

Adding the gate means formatting all 37 in **one commit first**, or the gate lands red. Do that commit
on its own, touching nothing else — a formatting sweep mixed into feature work is unreviewable.

---

---

## 6. The `web` image is built twice — now a deletion, not a refactor

`images-web` ([`pipeline.yml:113`](../../.github/workflows/pipeline.yml)) and `images-web-prod` (`:140`)
run the same `web/Dockerfile` and differ only in six `NEXT_PUBLIC_*` values. Next inlines those into
the **client bundle at build time**, which is why the staging image was permanently pinned to
`stg.abakacademy.id` and could never be promoted. Confirmed by building it and grepping
`.next/static/chunks/*.js` (2026-08-02), not assumed.

**With staging gone that image points at a host with no backend.** So the fix is no longer runtime
config — it is deleting `images-web` and its six build args, leaving `images-web-prod` as the only
web build. Roughly half the web build time per commit, one commit.

[`web-build-once-runtime-config.md`](web-build-once-runtime-config.md) stays on disk as the design for
the day a non-production target exists again. Its premise is suspended, not wrong. The cheapest first
step recorded there still holds: app and API are served from one host, so `NEXT_PUBLIC_API_BASE_URL`
can be a relative `/api/v1` and needs no build arg at all.

## Acceptance

- A merge to `main` deploys to **prod** without anyone editing `IMAGE_TAG` by hand.
- ~~One pipeline run per push to `main`~~ — done.
- One `web` image built per commit; no job publishes an image pointing at a dead host.
- `git grep -i staging deploy/` finds nothing that reads like a live environment.
- `gofmt -l backend/` is empty and CI fails when it isn't.
- `school/classes/page.tsx` and its i18n keys are gone; no other route 404s.
- CI wall-clock down by the figure PR #74 measured.

## Out of scope

- Dropping `orders.invoice_url`. Explicitly retained above.
- Any change to what the compose files deploy, beyond automating who edits them.
- Rebuilding a non-production environment. If that changes,
  [`web-build-once-runtime-config.md`](web-build-once-runtime-config.md) comes back to life and §1
  turns back into a refactor instead of a deletion.
