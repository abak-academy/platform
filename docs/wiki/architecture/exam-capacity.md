---
title: Exam capacity — connection pooling, hot paths, and scaling knobs
updated: 2026-09-25
verified_against: e6cf227c
---

What determines how many concurrent exam-takers this platform can serve, and what to turn if it isn't enough. Covers DB pooling, the autosave/submit write path, bcrypt cost, rate limiting, nginx, and web scaling. See [[system-overview]] for component boundaries and [[infra-and-deploy]] for deploy mechanics; domain rules for the exam lifecycle live in [[exam-lifecycle-and-scoring]].

## Database connection pooling

- Postgres and PgBouncer run natively on `vm-db`, not as containers — no `postgres` service in `deploy/compose/production.yml`.
- `db_max_conns: 10` in `backend/config/env/prod/config.yaml:23` (same in `staging/config.yaml:23`, `config.example.yaml:31`). Each of `api` and `worker` opens its own pool at this ceiling → 20 client connections toward PgBouncer.
- Applied in `backend/internal/infra/postgres.go` (`poolConfig`, `NewPoolWithMaxConns`): `maxConns >= 1` always wins over the DSN; `defaultMaxConns = 10` is the fallback if the config value were ever 0. The comment at `postgres.go:10-14` sizes this against "2 vCPU, two Go processes" — that sizing rationale is stale after the vm-app resize below; the value itself (10) has not changed since.
- `cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe` is hardcoded in code (`postgres.go:32`), overriding anything in the DSN. Required because PgBouncer transaction pooling doesn't let named prepared statements survive across transactions — `cache_describe` is pooler-safe. Covered by `backend/internal/infra/postgres_test.go`.
- `deploy/alloy/config.alloy` scrapes `PGBOUNCER_EXPORTER_ADDRESS` (`production.yml:219`) — this is the only pooler-side visibility; nothing in-repo currently graphs `cl_waiting`.
- `Unverified`: current PgBouncer `default_pool_size`/`max_client_conn` and Postgres `max_connections` live on `vm-db`, outside this repo. The prod config comment (`config.yaml:22`) records `default_pool_size=20, max_client_conn=500, max_connections=100` as measured 2026-08-13 — treat as a measurement, not a guarantee it's unchanged.

## bcrypt cost

- `backend/internal/service/password.go:10`: `const productionBcryptCost = 10`. Comment: "10 is bcrypt's own default and the deployed floor for generated and operator-set passwords."
- `hashPassword` (`password.go:13-19`) drops to `bcrypt.MinCost` under `testing.Testing()`, so test suites never pay production cost.
- `CompareHashAndPassword` (`backend/internal/service/auth.go:27`) reads cost from the stored hash — no rehash-on-login, so any hash created before the cost-12→10 change (or any admin hash created at a higher cost) keeps its original cost. This is deliberate: an upgrade-on-login pattern would silently downgrade those instead.
- `backend/internal/metrics/metrics.go:41-47` exposes `login_bcrypt_seconds`, labelled by operation — the only per-request cost visibility; no dashboard confirmed reading it in this repo.
- Guard: `TestProductionBcryptCostIsNotWeakened` (referenced in `docs/backlog/e7-scale-event-readiness.md:136`) asserts a floor, not a ceiling — raising the cost again requires a conscious change to both the constant and the guard.

## Autosave and submit write path

- Route: `PATCH /exam/sessions/:id/answers` → `Handler.StudentSaveAnswers` (`backend/internal/handler/exam.go:718`) → `Service.SaveAnswers` (`backend/internal/service/exam_session.go:613`).
- Per autosave with `len(inputs) > 0`, the service issues, in order: `GetExamSessionForStudent`, `GetExamForSession`, `GetQuestionTestMap`, then (only for `utbk`/`ielts` exams) `GetSessionSections` for the active-section guard, before the write. Standard-mode exams (the only mode observed running in production per historical measurement) skip the section guard entirely (`exam_session.go:648-660` conditions on `exam.Mode == "utbk" || "ielts"`).
- The write itself is one statement: `SaveAnswersTx` (`backend/internal/repository/exam.go:2543`) does `WITH guard AS MATERIALIZED (SELECT id FROM exam_session WHERE id = $1 AND status = 'in_progress' FOR UPDATE) ... FROM guard g, unnest(...)`. The row lock and the batched upsert (via `unnest` over 8 parallel arrays) are inside one round trip regardless of how many answers are in the payload.
- Submit: `POST /exam/sessions/:id/submit` → `Handler.StudentSubmitSession` (`exam.go:739`). Admin override: `POST /admin/.../force-submit` → `AdminForceSubmitSession` (`exam.go:802`).
- `exam_registration` row locking (`repository/exam.go:2229,2341,2381`) uses the same `FOR UPDATE` pattern for registration/live-session-count checks — same anti-clobber approach as the answers guard.
- No per-endpoint rate limit on answers or submit — `backend/internal/server/routes.go:151-152` register them with no rate-limiting middleware; only `/login` carries one (below).

## Rate limiting

- `LoginRateLimiter()` (`backend/internal/handler/middleware.go:71-97`) is the only rate limiter in the codebase. In-memory store, `Rate: 10`, `Burst: 10`, `ExpiresIn: 1m`, keyed by `c.RealIP()`.
- Wired to exactly one route: `auth.POST("/login", h.Login, handler.LoginRateLimiter())` (`routes.go:16`). Nothing else — not autosave, not submit, not registration — is rate-limited.
- Because the store is in-process memory, it is per-container: with `api` at `replicas: 2` (below), the effective ceiling for one client IP is up to 2x the configured 10/s if requests land on different replicas behind nginx's per-request DNS resolution.
- `c.RealIP()` depends on `real_ip_header CF-Connecting-IP` and the `set_real_ip_from` Cloudflare ranges in nginx (below) being correct — if Cloudflare's IP list changes and nginx isn't updated, this degrades to rate-limiting by a shared proxy IP instead of per-client.

## nginx: gzip, TLS, upstream resolution

- `deploy/nginx/production.conf` and `staging.conf` carry identical gzip blocks: `gzip on; gzip_proxied any; gzip_vary on; gzip_comp_level 5; gzip_min_length 1024;` for `application/json application/javascript text/css text/plain application/xml`.
- No `keepalive` directive to either upstream in either nginx conf, and deliberately no `upstream {}` blocks at all — `production.conf:10-14` explains why: an `upstream` block resolves once at config load and caches the IP, so recreating just the `api` container 502s every `/api/` route until nginx restarts. Instead `resolver 127.0.0.11 valid=10s` plus `set $api_upstream`/`set $web_upstream` re-resolves Docker DNS every 10s. Consequence for capacity: nginx does not keep persistent connections to `api`/`web`, so each proxied request pays a fresh upstream connection — same trade Cloudflare-facing TLS termination pays for browser connections; the load-test tooling below still uses connection reuse when talking to nginx from the outside, which is a divergence from real browser behavior worth remembering when reading any load-test result.
- TLS is Cloudflare Full (strict) with a Cloudflare Origin Certificate — no ACME client on the prod path; firewall admits only 443.
- `real_ip_header CF-Connecting-IP` plus the full Cloudflare IP range list makes `X-Real-IP`/`c.RealIP()` reflect the actual client, not Cloudflare's edge IP — this is what the login rate limiter above keys on.
- Access logs are structured JSON (`log_format json_access`) shipped by Alloy from container stdout; `deploy/grafana/dashboards/exam-day.json` reads `container_cpu_*`/`container_memory_*` families but its panel description at line 165 ("vm-app divides 2 vCPU across six containers") reflects the pre-resize VM shape — `Unverified` whether this description or any other in-repo doc has been updated for the current vm-app vCPU count; that count itself lives on GCP, not in this repo.

## Deploy topology: replicas and process scaling

- `deploy/compose/production.yml`: `api` at `replicas: 2` (line 88), `web` at `replicas: 3` (line 122), exactly one `nginx` (only service publishing ports). Replica counts are set in `deploy.replicas` in the manifest itself, not passed via CLI `--scale` — the manifest's own comment (`production.yml:26-29`) warns that a `--scale` flag on `docker compose up` silently overrides the manifest count, and that this exact mistake once left the box on one `web` replica under real load.
- `web`'s comment (`production.yml:117-119`): "Peak 242 MiB measured under IKLC event load with a single replica. next-server is one single-threaded Node process per container, so extra replicas buy resilience against a blocked event loop, not extra cores." This documents the Node-vs-Go asymmetry directly in the manifest: the Go `api`/`worker` binaries see all cores per process (no `GOMAXPROCS` override in this repo), while each `web` container is capped near one core regardless of host size — replicas are the only way to use more cores for SSR.
- `api`'s comment (`production.yml:85`): "Peak 53 MiB measured under IKLC event load; Go memory is predictable" — mem_limit 768m for both `api` and `worker`, 512m for `web`, 256m for `nginx` (raised from 128m after hitting 77% during the same event), 384m for `alloy`.
- Rollback and deploy both go through the same `IMAGE_TAG=<tag-or-sha> docker compose -f deploy/compose/production.yml up -d` command (`production.yml:18-31`); images are pulled from Artifact Registry by tag/SHA digest, never built on the box.

## Load-test tooling

- Lives at repo root `loadtest/`: `exam-lifecycle.js` (the k6 scenario), `exam-lifecycle.test.mjs` (offline harness tests), `run.sh` (container invocation — passes env to the k6 container via an explicit `-e` list, so a new env var must be added there or it's silently `undefined` inside the container), `seed.sql` (RUN_ID-scoped, idempotent — safe to rerun with the same RUN_ID), `README.md`, and a `results/` directory (gitignored) for run artifacts.
- The scenario models per-VU behavior: staggered login, per-VU submit deadline drawn from a window, and adaptive answer pacing recomputed per question — not a single global submit gate. This shape was a direct fix for an earlier synchronized-burst failure mode (see Known measurements).
- `Unverified` whether the current `loadtest/exam-lifecycle.js` on `main` at this SHA exercises full page loads (SSR + TLS handshake) or only the API — historical notes describe an API-only, connection-reuse harness that a later local branch attempted to change; that branch's status against current `main` was not checked as part of this page and should be re-verified in-repo before quoting it, rather than assumed from history.

## Known measurements

Not re-run for this page. Kept only where the underlying conclusion still guides current design (e.g. replica counts, gzip config, no-rehash-on-login above already reflect these). Treat every number below as a point-in-time measurement on a past VM/code shape, not a current guarantee.

| Date | Measurement | Conclusion still in effect |
|---|---|---|
| 2026-08-14 | bcrypt cost 10 ≈ 52.7ms/op on Apple M3 Pro vs. cost 12 ≈ 234ms/op (~4.45x) | Cost 10 shipped; never independently re-measured on prod hardware |
| 2026-09-06/11 | 5,000-VU load test, final run 4,997/5,000 lifecycles (99.94%) | Certified the API + Postgres write path only, at the harness shape available then |
| 2026-09-12 | IKLC live event, session 1: Cloudflare 525 at ~3,300 concurrent students; `vm-app` load average 7.26/2 cores (363%); nginx TLS 61%, web SSR 56%, api 53% of 200% available CPU | **Scale `web`, never `api`** — Go already uses all cores per process; Next.js SSR is single-threaded per container. This conclusion is now encoded in `production.yml`'s replica counts and comments (above), not just in history |
| 2026-09-12 | IKLC session 2, same fix, denser load (3,289 entering in 11 min vs. 29): load average 1.10–1.65 (14–21%), zero nginx 5xx | The vm-app resize + `--scale web=3 api=2` (now static replicas in the manifest) resolved the session-1 failure mode |

## Open questions

- Current `vm-app` vCPU count and current PgBouncer/Postgres pool settings are infra facts that live outside this repo (GCP + `vm-db`) — no committed file in `platform/` states them as of this SHA. Confirm against the running infra before capacity planning, not against the historical wiki notes this page was checked against.
- Whether `loadtest/exam-lifecycle.js` at this SHA drives real page loads (to exercise nginx TLS + Next.js SSR, the two paths that actually failed at the live event) or still hits the API only.
- Whether anything now monitors PgBouncer `cl_waiting` during a real exam window — the one metric that would directly confirm or rule out pool starvation, as opposed to inferring it from apparent request latency.
