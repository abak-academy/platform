---
title: Infra and Deploy
updated: 2026-09-25
verified_against: e6cf227c
---

# Infra and Deploy

Current topology, build/release flow, CI gates, edge TLS, object storage, environment config, and observability. See [[./system-overview.md]] and [[./exam-capacity.md]] for component/capacity views; operating procedures live in [`../../runbooks/`](../../runbooks/).

## Topology

Single combined VM (`vm-app`) runs everything except Postgres, per `deploy/compose/production.yml`:

| Service | Role | Notes |
|---|---|---|
| `nginx` | only port-publishing service (443) | TLS termination, reverse proxy |
| `web` | Next.js, 3 replicas | `mem_limit: 512m` |
| `api` | Go HTTP API, 2 replicas | `mem_limit: 768m` |
| `worker` | Go background worker | outbox/event processing |
| `redis` | cache/session store | |
| `gotenberg` | headless-Chromium HTML→PDF | certificate/card rendering |
| `node-exporter`, `cadvisor`, `alloy` | observability agents | ship to Grafana Cloud |

Postgres and PgBouncer are **not** containers here — native on `vm-db` (internal network), per the header comment in `deploy/compose/production.yml`. Object storage is GCS. Replica counts live in `deploy.replicas` in the manifest; the file warns against a CLI `--scale` flag, which silently overrides it outside `up -d`.

There is **no `deploy/compose/staging.yml` in this repo** — only `deploy/nginx/staging.conf` remains, which still references `docker compose -f deploy/compose/staging.yml`. `backend/config/env/staging/config.yaml` and the `staging` Artifact Registry config copy in `deploy/images/Dockerfile.api`/`Dockerfile.worker` also still exist. No CI job builds or pushes a staging web image (`.github/workflows/pipeline.yml` only defines `images-backend` and `images-web-prod`). Treat staging as configured-but-not-currently-deployed from this repo's evidence; do not assume it is live.

## Build and release flow

CI is `.github/workflows/pipeline.yml`, a single workflow with 5 jobs:

1. **`backend`** — `go build ./...`, `go vet ./...`, `go test -race -shuffle=on -timeout 20m ./...` via `deploy/pipeline/backend.sh`.
2. **`render-gate`** — boots Postgres/MinIO/Gotenberg from `deploy/compose/local.yml` and runs `TestCertificateRender_*`/`TestCardRender_*` against real Gotenberg (`deploy/pipeline/backend-render-gate.sh`).
3. **`frontend`** — `npm ci`, `tsc --noEmit`, `vitest run` (`deploy/pipeline/frontend.sh`); also installs Go 1.26 because Vitest's sanitizer-bridge tests spawn `go test` against `backend/internal/service`.
4. **`images-backend`** — gated on `startsWith(github.ref, 'refs/tags/v')` or `workflow_dispatch`; needs all three jobs above. Compiles static binaries via `deploy/pipeline/build-artifact.sh` (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`), then `deploy/pipeline/build-image.sh` builds+pushes `api`+`worker` images using `deploy/images/Dockerfile.api`/`.worker` (thin: copy prebuilt binary + `config/env/{staging,prod}/config.yaml`, never the gitignored `dev/` dir).
5. **`images-web-prod`** — same gate, builds+pushes `web:prod-<tag>` via `deploy/images/Dockerfile.web`-equivalent path in `build-image.sh`, with `NEXT_PUBLIC_*` build args baked in for prod (`hub.abakacademy.id`).

Release is **tag-based**: pushing a `v*` tag (or manual `workflow_dispatch`) is what builds and pushes images; a plain merge to `main` only runs the verification jobs (`pipeline.yml:9-16` — `push` is scoped to `main`+`v*` tags, `images-*` jobs gate on the tag prefix). Images carry two tags pointing at the same digest: the commit SHA (immutable identity, diffed against `docker ps` on `vm-app`) and, when triggered by a version tag, the human-facing version (`deploy/pipeline/build-image.sh:12-20`).

`api`/`worker` are built once and are byte-identical across environments (no env baked in); `web` is built per target environment because `NEXT_PUBLIC_*` values are inlined into the client bundle at build time (`deploy/pipeline/build-image.sh:32-45`) — this is why there's a separate `deploy` compose Dockerfile (`backend/Dockerfile`, multi-stage, used only by `deploy/compose/local.yml`) versus the CI path (`deploy/images/Dockerfile.api`/`.worker`, prebuilt-binary based).

Concurrency: one workflow run per ref; runs on non-`main` refs cancel superseded runs, `main` runs are never cancelled (`pipeline.yml:18-23`).

Deploying the built images is a separate manual step, not part of CI: `IMAGE_TAG=<version-or-sha> docker compose -f deploy/compose/production.yml up -d` (header comment, `deploy/compose/production.yml:1-30`). A push to `main` does not deploy.

## CI gates in detail

- `deploy/pipeline/backend.sh` — `-timeout 20m` on `go test` because `internal/service` alone exceeds Go's default 10m/package on CI (testcontainers spin up multiple Postgres containers).
- `deploy/pipeline/backend-render-gate.sh` — bootstraps `backend/config/env/dev/{config,secrets}.yaml` from the committed `*.example.yaml` templates (the real `dev/` dir is gitignored and doesn't exist on a fresh CI checkout), then starts `postgres`, `minio`, `createbuckets`, `gotenberg` from `deploy/compose/local.yml` and waits on MinIO/Gotenberg health endpoints before running the gated tests. Kept separate from `backend.sh` so the main suite doesn't pull the Chromium image.
- `deploy/pipeline/frontend.sh` — `tsc --noEmit` then `vitest run`; no `npm run build` or `npm run lint` step in this script.
- `deploy/pipeline/build-artifact.sh` — compiles `bin/api` and `bin/worker` with `-ldflags "-s -w"`, `GOARCH=amd64` (hard requirement: images are built `linux/amd64` regardless of runner arch).

## TLS / edge

**Production** (`deploy/nginx/production.conf`, fronts `hub.abakacademy.id`):
- Cloudflare Full (strict) — nginx presents a **Cloudflare Origin Certificate** (`/etc/nginx/certs/origin.pem`/`origin.key`), not a publicly-trusted cert; no ACME client on this path.
- `resolver 127.0.0.11` + a `set $upstream` variable per location, not `upstream {}` blocks — avoids nginx caching a container IP at config-load time (recreating `api` alone would otherwise 502 every `/api/` route until nginx restarts).
- Cloudflare IP ranges are allow-listed via `set_real_ip_from` and `CF-Connecting-IP` is trusted for real client IP.
- Single entrypoint: `/api/` → `api:8080`, everything else → `web:3000`.
- JSON access log format (`json_access`) feeds Grafana Cloud / Loki via Alloy reading container stdout — no separate log-shipping process.
- Only `nginx` publishes a port (443); `api`/`web`/`redis`/`gotenberg` are internal-network only.

**Staging** (`deploy/nginx/staging.conf`, `stg.abakacademy.id`) — file present but has no corresponding compose manifest in this repo (see Topology). As written it describes a different model than prod: plain `upstream {}` blocks, Let's Encrypt via `certbot` (webroot renewal + ACME HTTP-01 challenge location, no Cloudflare origin cert), :80→:443 redirect. Do not assume this configuration is currently running; verify live state before relying on it.

## CDN / object storage

- Object storage is GCS, accessed directly (`object_storage_endpoint: storage.googleapis.com`, `object_storage_use_ssl: true` — `backend/config/env/prod/config.yaml`, `staging/config.yaml`). No CDN/edge-cache layer sits in front of the buckets in this repo's config or code.
- CORS on the bucket(s) is applied out-of-band via `gcloud storage buckets update --cors-file=deploy/storage-cors.json`, allowing `GET/HEAD/PUT` from `hub.abakacademy.id`, `stg.abakacademy.id`, and `localhost:3000` — needed because presigned-URL downloads/uploads (exam-card PDFs, certificate assets) go browser→GCS directly, not through the API (`deploy/compose/local.yml` comment; `deploy/storage-cors.json`).
- An `Exam.CDNBundle` boolean field exists end-to-end (`backend/internal/repository/exam.go:1197` etc., `backend/internal/handler/exam_package.go:203`) alongside `BundleURL`/`BundleGeneratedAt` columns, but no service code currently generates, uploads, pre-warms, or purges a bundle — it's a stored flag with no processing pipeline behind it. Any pull-through-cache / pre-warm design for exam bundles is **not implemented**; treat it as a future design, not current behavior.
- Local dev uses MinIO (`deploy/compose/local.yml`) with `MINIO_API_CORS_ALLOW_ORIGIN: "*"` standing in for the GCS CORS config above.

## Environment configuration

Config is loaded per environment directory under `backend/config/env/<env>/` (`backend/config/config.go:115-118`, `Load(env, configDir)` reads `<configDir>/<env>/config.yaml` + `secrets.yaml`). Environments present in the repo:

| Env dir | Committed? | Used by |
|---|---|---|
| `config.example.yaml` / `secrets.example.yaml` | yes | template for `dev/` and CI's render-gate bootstrap |
| `dev/` | no (gitignored, `.gitignore:20`) | local `go run` / compose |
| `staging/config.yaml` | yes (no secrets file committed) | baked into `deploy/images/Dockerfile.api`/`.worker`; no compose manifest currently deploys it |
| `prod/config.yaml` | yes (no secrets file committed) | baked into the same images; secrets mounted at runtime from `deploy/secrets/prod-secrets.yaml` (gitignored, hand-placed on the box) |

Selected differences (`prod/config.yaml` vs `staging/config.yaml`): `midtrans_env` (`production` vs `sandbox`), `cors_origins` (`hub.abakacademy.id` vs `stg.abakacademy.id`), separate Google OAuth client IDs, separate SMTP relays, separate GCS bucket names (`abak-academy-media` vs `abak_std`). Both set `db_max_conns: 10` (comment: 10 api + 10 worker = 20 clients against PgBouncer's `default_pool_size=20` on `vm-db`) and `enforce_school_npsn_registration: false`.

`api`/`worker` containers mount their secrets file read-only (`deploy/compose/production.yml`); `APP_ENV` selects the env directory at runtime (`environment: APP_ENV: prod`).

## Observability

Stack: `node-exporter` + `cadvisor` + `grafana/alloy`, shipping to Grafana Cloud (remote-write Prometheus + Loki push), provisioned by `deploy/alloy/config.alloy` — **the same file** is mounted by both `deploy/compose/local.yml` (profile `observability`, opt-in) and `deploy/compose/production.yml`; only environment variables differ (`DEPLOY_ENV`, `PROMETHEUS_URL`, `LOKI_URL`, credentials, `PGBOUNCER_EXPORTER_ADDRESS`).

Alloy pipelines (`deploy/alloy/config.alloy`):
- `prometheus.scrape "apps"` — scrapes `api:9102` and `worker:9102` (internal `/metrics`, exposed by `backend/cmd/api/main.go:60-67` and mirrored in `cmd/worker`; nginx never proxies this port) at 15s interval.
- `prometheus.scrape "infra"` — `node-exporter:9100`, `cadvisor:8080`, `docker_only` + a `--disable_metrics` list trimmed to what `deploy/grafana/dashboards/exam-day.json` reads, to stay under the Grafana Cloud free-tier series budget.
- `prometheus.scrape "pgbouncer"` — targets `PGBOUNCER_EXPORTER_ADDRESS` (default in the alloy config); down by design on local (no PgBouncer there).
- Log pipeline: `discovery.docker` → relabel container name/compose-service to `container`/`service` labels → `loki.process` promotes a JSON `level` field to an index label → `loki.write` push, giving `{service="api", level="error"}`-style queries that work identically against local Loki and Grafana Cloud.

A single dashboard is checked in: `deploy/grafana/dashboards/exam-day.json`, provisioned automatically into the local Grafana container (`deploy/compose/local.yml`, profile `observability`) and imported manually into Grafana Cloud for prod (per pipeline comments; no automated provisioning to Cloud in this repo).

`mem_limit` per service in `deploy/compose/production.yml` reflects load-tested numbers noted inline (e.g. `api` peak 53 MiB, `web` peak 242 MiB, `nginx` hit 98 MiB under IKLC event load) — these are comments in the manifest, not something this page re-derives.

## Unverified (not in repo)

- Live state of `vm-app`/`vm-db` (running containers, actual image tags deployed, whether staging is currently reachable) — not observable from repo files; `docker ps` on the box is the source of truth per `deploy/pipeline/build-image.sh` comments.
- Whether the GCS CORS policy in `deploy/storage-cors.json` has actually been applied to the live buckets (`gcloud storage buckets update` is run out-of-band, not in CI).
- Whether `deploy/compose/staging.yml` (referenced by `deploy/nginx/staging.conf`) exists elsewhere and was only removed from this checkout, versus staging being fully decommissioned.
