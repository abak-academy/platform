# Observability — metrics and logs on Grafana Cloud

| | |
|---|---|
| **Issue** | [#98](https://github.com/abak-academy/platform/issues/98) |
| **Objective** | When an exam slows down, one dashboard answers which of these it is: API CPU, app-side connection pool, Postgres/PgBouncer, or the student network — instead of today's zero signals. |
| **Depends on** | None hard. Ideally lands before [#95](https://github.com/abak-academy/platform/issues/95), because a load test without metrics produces p95s with no causes. |
| **Verified against** | `main`, 2026-08-25 (local repro run end-to-end: scrape, remote-write, log shipping, dashboard provisioning) |
| **Ops guide** | [`docs/runbooks/monitoring-deployment.md`](../runbooks/monitoring-deployment.md) |

---

## Decisions of record

These were argued and settled during design review; change them through discussion, not silently.

1. **Grafana Cloud free tier**, not self-hosted Prometheus/Grafana. `vm-app` divides
   2 vCPU across six containers that are already the thing we are trying to protect;
   running the monitoring stack on it would tax the patient exactly at exam time. The
   free tier's quotas (10k active series, 50 GB logs/month) sit above our projected
   usage — but not comfortably: **measured ≈7k active series** with defaults
   (cadvisor ~1.8k, node-exporter ~1.3k, `http_request_duration_seconds` ~2.6k,
   `http_requests_total` ~560, go/process collectors ~57 per process). 10k is a hard
   ceiling — **Grafana Cloud rejects writes past it**, which would drop data on exam
   day specifically — so cadvisor ships with `--docker_only` and a `--disable_metrics`
   list (see production.yml), and if the measured post-trim total still creeps toward
   10k the next lever is a `write_relabel_config` keep-list in `config.alloy`.
2. **One collector process: Grafana Alloy.** It scrapes every local source and pushes
   both metrics (remote_write) and logs out over HTTPS. No inbound ports are opened;
   the firewall stays 443-only.
3. **Sources stay separate daemons** (cAdvisor, node_exporter, pgbouncer_exporter).
   Alloy is a pipeline, not a source: it cannot read cgroups, `/proc`, or speak
   PgBouncer's admin protocol. Consolidating into Alloy's embedded exporters would
   trade canonical metric names (and working reference dashboards) for ~100–150 MB of
   RAM. If consolidation is ever wanted, absorbing node_exporter via
   `prometheus.exporter.unix` is trivial and reversible; cAdvisor and PgBouncer have
   no lossless equivalent.
4. **Logs go to Loki as the system of record**; the docker `json-file` driver stays
   as a small transport buffer (`10m × 3`). The decisive argument: redeploying during
   an exam recreates containers and destroys json-file logs — the evidence would
   vanish precisely in the hour we need it. Shipped-out logs survive any deploy.
   Retention limits stay unchanged for that reason.
5. **Logging convention: invoked-params, then errors only.** One Info line per HTTP
   request (all requests — per-request status codes are what correlate a slow window
   with its cause, and Loki makes volume cheap) enriched with route template, path
   params, duration, request id. Worker jobs/outbox events log once at claim with
   their ids/types; everything else is error-level only. No future phase should add
   per-module logging by default — middleware and claim loops already cover every
   module generically. Deeper instrumentation (business events, PII redaction,
   tracing) waits for an incident that demands it.
6. **Local & production only; staging gets nothing.** The instrumented images are
   shared, so staging technically serves `/metrics` on the internal port, but no
   collector runs there.
7. **`/metrics` never touches nginx.** api and worker each serve a second, internal
   HTTP server bound to the compose network (`:9102`, override with `METRICS_ADDR`);
   unreachable from the public internet by construction.

## Topology

```
                     GRAFANA CLOUD (SG region)
        ┌──────────────────────────────────────────────┐
        │  Grafana UI ◀─ PromQL/LogQL ─▶ Prometheus    │
        │        ▲                        Loki         │
        └────────▲──────────────────────────▲──────────┘
                 │ HTTPS outbound only      │
═════ vm-app ════╧══════════════════════════╧══════════════
  [new] alloy ──scrape──▶ api:9102, worker:9102
              ├──scrape──▶ cadvisor, node-exporter
              ├──tail────▶ docker logs (every container)
              └──push─────▶ Grafana Cloud
  [new] cadvisor (§7), node-exporter (VM context)
  [existing] nginx · api · worker · web · redis · gotenberg

═════ vm-db ═══════════════════════════════════════════════
  [new] pgbouncer_exporter (§6, reads SHOW POOLS) — installed
        natively next to Postgres/PgBouncer, scraped by alloy
        over the internal network
```

The same three new services exist in `local.yml` behind the compose profile
`observability`. The Alloy config file is byte-for-byte identical between local and
production; environment enters only through URL/credential variables. Local Grafana
provisions datasources under the SAME UIDs Grafana Cloud uses
(`grafanacloud-prom`, `grafanacloud-loki`), so the dashboard JSON in
`deploy/grafana/dashboards/` provisions unchanged in both places — dashboards are
iterated locally, then imported to Cloud.

## What got instrumented

Mapped to issue #98 §1–§8:

| § | Metric family | Source |
|---|---|---|
| 1 | `dbpool_acquired_conns`, `dbpool_total_conns`, `dbpool_max_conns`, `dbpool_idle_conns`, `dbpool_acquire_total`, `dbpool_empty_acquire_total`, `dbpool_acquire_duration_seconds_total`, `dbpool_canceled_acquire_total` | `pgxpool.Stat()` read at scrape time via GaugeFunc/CounterFunc callbacks — no polling goroutine. `EmptyAcquireCount > 0` rising is THE detector for the #96 condition. Note pgx v5 exposes cumulative totals, not per-acquire samples, so p95 wait must be approximated by rate ratios; a true histogram would require wrapping `pool.Acquire` everywhere — deferred until proven necessary. |
| 2 | `http_requests_total{route,method,status}`, `http_request_duration_seconds{route,method}` | echo middleware in `internal/server/metrics.go`. The route label carries the TEMPLATE (`/api/v1/exam/sessions/:id/answers`); raw URIs would explode cardinality one series per session id. Status fallback handles Echo's error handler running outside the middleware chain. |
| 3 | `login_bcrypt_seconds{op}` | wraps `CompareHashAndPassword` in Login and ChangePassword (`internal/service/auth.go`), labelled `op="login"` / `op="change_password"`. Validates the modeled ~234 ms/op against real VM load; drives the concurrent-login ceiling. The capacity model quotes the `login` series only. |
| 4 | `exam_sessions_active` | worker counts sessions whose exam is inside its live window (scheduled start → duration+grace past the end; unscheduled exams: started < 3h ago) every 30s, served by the partial index `idx_examsession_active`. Registered **only in the worker process** — an api-exported gauge would sit at 0 forever. Counted in SQL, not in-process, because "active" has one authoritative answer. |
| 5 | `go_*`, `process_*` | client_golang runtime collectors, registered alongside app metrics. |
| 6 | `pgbouncer_pools_*`, `pgbouncer_stats_*` | pgbouncer_exporter on vm-db (manual install — see runbook). Dashboard places it side-by-side with §1 because `cl_waiting = 0` proves nothing about the app-side pool. |
| 7 | `container_cpu_usage_seconds_total`, `container_memory_usage_bytes{name}` | cAdvisor. Answers "did gotenberg or worker steal CPU from api". |
| 8 | nginx access logs → LogQL | JSON `log_format` added to `production.conf` (request_time, upstream_time, bytes_sent vs body_bytes_sent for gzip ratio). No stub_status/exporter needed — the issue's §8 asks all come from access logs. |

Request logs gained `route`, `params` (path ids like `session_id=...`),
`duration_ms`, `request_id`; worker claims log `job_id`+`type` / `event_id`+`type`.
Alloy promotes slog's `level` field to an indexed label, so
`{service="api", level=~"(?i)error"}` needs no parse at query time.

## Resource budget

Bounded by `mem_limit`: node-exporter 64m, cadvisor 200m, alloy 384m (~650 MB worst
case, realistically 250–400 MB). CPU cost is flat regardless of traffic — scrape
intervals don't scale with load; parsing ~100 MB/h of peak exam logs is negligible.
**Measured post-deploy numbers belong in the runbook** — issue #98 explicitly
requires recorded figures, not assumptions.

Known accepted gaps:

- The `pgbouncer` scrape target is permanently DOWN in the local profile (no
  PgBouncer locally) — cosmetic target noise, documented in the config header.
- Two series can linger briefly after container recreation (old + new instance);
  Prometheus staleness settles them within minutes.

## Rollout order

Implemented in this change:

1. Go instrumentation (`internal/metrics`, internal `:9102` servers, enriched
   request logger, worker claim logs, session gauge) — verified locally.
2. Local observability stack (`local.yml`, profile `observability`) +
   `deploy/alloy/config.alloy` + Grafana provisioning + "Exam day" dashboard —
   verified end-to-end offline.
3. Production compose additions (node-exporter, cadvisor, alloy) + nginx JSON
   access log — additive only; `up -d` creates the three services without touching
   existing ones.

Remaining, requiring human/cloud actions — see the runbook:

4. Grafana Cloud stack creation, credentials into `deploy/.env`, import dashboard.
5. pgbouncer_exporter install on vm-db.
6. Alert rules (pool-empty acquires, p95 answers threshold, sustained api CPU, 5xx
   rate) configured in Cloud AND deliberately triggered once — #98 rejects
   alerts that have only ever been configured.
