# Web image is built twice — move `NEXT_PUBLIC_*` to runtime

| | |
|---|---|
| **Issue** | tracked as §6 of [#92](https://github.com/abak-academy/platform/issues/92) — was #93 (2026-08-13 → 2026-08-14, folded back, not dropped) |
| **Status** | ⏸️ **Premise suspended 2026-08-17** — see below. The design is not wrong; the problem it solves is currently absent |
| **Objective** | One `web` image, promotable staging→prod, instead of one build per environment. |
| **Depends on** | — |
| **Verified against** | `chore/ci-speedup-bcrypt-and-workflow` @ `541584b`, 2026-08-02 |
| **Supersedes** | the "revisit later" left open by the 2026-07-10 decision |

> ## ⏸️ Suspended 2026-08-17 — read this before implementing any of it
>
> Staging was deleted (VM, disk, 15 snapshots, bucket, IP) and the decision is **prod-only,
> permanent**. **There is nothing to promote between.** `images-web` now builds an image pinned to
> `stg.abakacademy.id`, a host with no backend behind it — so the immediate fix is to *delete that
> job*, not to make its output promotable. That is what #92 §6 asks for.
>
> Everything below stays accurate and comes back the day a non-production target exists again —
> which [#95](https://github.com/abak-academy/platform/issues/95) may well stand up for a load-test
> run. Keep this doc; do not implement it in the meantime.

## What happens today

`images-web` and `images-web-prod` run the same `web/Dockerfile` twice. They differ in
six values, all of them `NEXT_PUBLIC_*`:

| | staging | prod |
|---|---|---|
| `NEXT_PUBLIC_API_BASE_URL` | `https://stg.abakacademy.id/api/v1` | `https://hub.abakacademy.id/api/v1` |
| `NEXT_PUBLIC_MIDTRANS_SNAP_URL` | sandbox (script default) | `https://app.midtrans.com/snap/snap.js` |
| `NEXT_PUBLIC_MIDTRANS_CLIENT_KEY` | `secrets.MIDTRANS_CLIENT_KEY` | `secrets.MIDTRANS_CLIENT_KEY_PROD` |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID` | hardcoded in the workflow | `vars.PROD_GOOGLE_CLIENT_ID` |
| registry | `ghcr.io` | `asia-southeast2-docker.pkg.dev` |
| `TARGET_ENV` | `staging` | `prod` |

Next inlines `NEXT_PUBLIC_*` into the **client bundle at build time**. The staging image
therefore points at `stg.abakacademy.id` permanently and cannot be promoted.

**Confirmed, not assumed** (2026-08-02): built the staging image locally and grepped the
bundle — `stg.abakacademy.id/api/v1` is present in `.next/static/chunks/*.js`.

Contrast `api` and `worker`: `Dockerfile.api` copies **both**
`config/env/staging/config.yaml` and `config/env/prod/config.yaml` into the image and
selects at runtime. One image serves both environments — which is why
`chore/ci-speedup-bcrypt-and-workflow` builds that pair once and pushes it to both
registries, while `web` still needs two jobs.

## This is not a Next.js limitation

It is a consequence of choosing `NEXT_PUBLIC_*`. Amartha's own Next app,
`bitbucket.org/Amartha/ng-mis`, promotes a single image dev→uat→prod with
`docker pull` → `docker tag` → `docker push`, no rebuild, because `next.config.js` sets
`publicRuntimeConfig` — read at server start, never inlined into the client bundle.

**Blocker to copying it verbatim:** `publicRuntimeConfig` is Pages Router only.
`ng-mis` uses `src/pages`; this app uses `web/app` (App Router).

## What changed since the 2026-07-10 decision

The decision then was "accept build args, revisit later". Two facts are now different:

- **There is a server-side surface.** `web/app/api/admin/certificate-template/route.tsx`
  is a route handler. The old note that `web/` has *zero* route handlers is stale.
- **One of the four values is already fetched at runtime.** Per
  [`repo-migration-to-client-org.md`](../runbooks/repo-migration-to-client-org.md), the
  storefront reads the Midtrans client key from the API and only falls back to the build
  arg — which is why `secrets.MIDTRANS_CLIENT_KEY` has never been set and staging still
  works.

Still true: the print route (`app/(print)/exam/[id]/card/page.tsx`) is `"use client"`, so
there is no server-rendered page to hang request-time config on today.

## Sketch

`NEXT_PUBLIC_API_BASE_URL` is the hard one — it is the address used to fetch everything
else, so it cannot itself come from the API. Options, cheapest first:

1. **Same-origin API — verified viable.** Both environments already serve the app and
   the API from one host, so a relative `/api/v1` needs no build arg at all:
   - `deploy/nginx/staging.conf:36-50` — `server_name stg.abakacademy.id`,
     `location /api/` → `api`, `/` → `web`
   - `deploy/nginx/production.conf:45-63` — `server_name hub.abakacademy.id`, same split

   This is the whole fix for `NEXT_PUBLIC_API_BASE_URL`, and after E3 there is no
   non-browser consumer left to worry about: the render flow now **posts** HTML to
   Gotenberg rather than having Gotenberg fetch a Next route
   (`backend/internal/service/card_generate.go:75-79`), and no `web_internal_url` /
   `INTERNAL_API_BASE_URL` setting exists in `backend/config/` any more — both were
   removed with the Go renderer. Checked 2026-08-02; the older three-hop description of
   this flow is stale.
2. **Container-start injection.** An entrypoint writes `/public/env.js` with
   `window.__ENV__ = {...}` from container env; the client reads that instead of
   `process.env.NEXT_PUBLIC_*`. Works with a fully client-side app.
3. **Server component + `headers()`** for a per-request value. Needs the consuming pages
   to stop being `"use client"` — the largest change.

Option 1 removes the problem rather than solving it, and should be ruled in or out first.

## Payoff

Small in CI time — the two web builds run in parallel, so the saving is one job's
runner-minutes, not wall-clock. The real gain is that **the artifact tested on staging
is the artifact that ships to prod**, which is the property the current setup gives up.
