# Monitoring rollout — pre & post deployment (issue #98)

Step-by-step for putting the observability stack live. Design decisions and the
metric inventory live in
[`docs/backlog/observability-grafana-cloud.md`](../backlog/observability-grafana-cloud.md);
this file is the operator path.

Two distinct rollouts:

- **Local repro** — no cloud account needed, do it any time.
- **Production** — needs a Grafana Cloud stack (done 2026-08-29), the
  instrumented api/worker image bump, and one alert-fire drill before exam
  day. The vm-db pgbouncer_exporter install is deferred (§1.3).

---

## 0. Local repro (no deploy, offline)

```sh
docker compose -f deploy/compose/local.yml --profile observability up -d \
  postgres redis minio createbuckets gotenberg api worker \
  node-exporter cadvisor prometheus loki grafana alloy
```

Notes:

- First `up` builds api/worker images; after backend code changes run with
  `--build api worker` or the metrics endpoint changes won't be in the image
  (this exact trap was hit once already).
- The profile keeps ~800 MB of containers out of everyday `up` runs.

Verify (each step has been proven working):

1. All scrape targets up except pgbouncer (expected down locally):
   `curl -s http://localhost:9090/api/v1/query?query=up`
2. Route-labelled series exist:
   `curl -s "http://localhost:9090/api/v1/query?query=http_requests_total"`
3. Logs flow with label parity (`service`, `env=local`, `level`):
   `curl -s 'http://localhost:3100/loki/api/v1/labels'`
4. Dashboard provisioned: open **http://localhost:3002** → Abak folder →
   *"Exam day — where is the time going?"* (anonymous admin is enabled locally).
5. bcrypt counter moves on a real login attempt against an existing user
   (nonexistent identifiers short-circuit before bcrypt).

---

## 1. Pre-deployment checklist

### 1.1 Grafana Cloud stack

1. Create/choose a Grafana Cloud account; create a free **stack**. Indonesia is
   available on the free tier — the live stack runs in `prod-ap-southeast-2`
   (created 2026-08-29), closer to the VMs than Singapore.
2. From the stack's details page collect:
   - Prometheus **Remote Write** endpoint + instance id
   - Loki push URL + tenant id
   - One API token (Access policy scoped to both endpoints) used as password
     for both pipelines.
3. Free-tier quotas to keep in mind: 10k active series, 50 GB logs/month.

### 1.2 vm-app credentials

vm-app has no git checkout — the deploy dir `/home/<deploy-user>/abak-app` is
hand-placed, owned by a dedicated deploy user (ssh in as your admin account,
then `sudo -iu <deploy-user>`; docker works without sudo via the `docker`
group). The compose `.env` sits at the
the ROOT of that directory, not `deploy/.env` (gitignored; pattern in
[deploy/.env.example](../../deploy/.env.example)):

`/home/<deploy-user>/abak-app/.env`:

```
GRAFANA_CLOUD_PROM_URL=https://prometheus-prod-xx-xxx.grafana.net/api/prom/push
GRAFANA_CLOUD_PROM_INSTANCE_ID=<numeric>
GRAFANA_CLOUD_PROM_API_KEY=<token>
GRAFANA_CLOUD_LOKI_URL=https://logs-prod-xxx.grafana.net/loki/api/v1/push
GRAFANA_CLOUD_LOKI_TENANT_ID=<numeric>
GRAFANA_CLOUD_LOKI_API_KEY=<token>
```

Compose resolves `${…}` from this file when run from `~/abak-app`; the §2
commands pass `--env-file` explicitly so nothing depends on the default lookup.

### 1.3 vm-db: install pgbouncer_exporter — DEFERRED (no vm-db access)

Status 2026-08-29: deferred — no operator access to vm-db. Consequences while
deferred: the alloy `pgbouncer` scrape target stays DOWN (cosmetic), the
dashboard's pgbouncer panels stay empty, and the §3.6 pool-starvation alert
leans solely on the app-side `dbpool_empty_acquire_total` — which is the #96
detector anyway. The snippet below is preserved for whoever gets vm-db access.

PgBouncer runs natively there, so does its exporter — outside any compose
manifest:

```sh
# on vm-db, as root
curl -LO https://github.com/prometheus-community/pgbouncer_exporter/releases/download/v0.12.1/pgbouncer_exporter-0.12.1.linux-amd64.tar.gz
tar xzf pgbouncer_exporter-*.tar.gz && install pgbouncer_exporter*/pgbouncer_exporter /usr/local/bin/

useradd -rs /usr/sbin/nologin pgbouncer_exporter
cat >/etc/systemd/system/pgbouncer_exporter.service <<'EOF'
[Unit]
Description=pgbouncer_exporter
After=network-online.target

[Service]
User=pgbouncer_exporter
Environment=DATA_SOURCE_NAME=postgresql://<stats_user>:<password>@127.0.0.1:6432/pgbouncer?sslmode=disable
ExecStart=/usr/local/bin/pgbouncer_exporter
Restart=always

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload && systemctl enable --now pgbouncer_exporter
curl -s localhost:9127/metrics | head   # sanity
```

The connect user must exist in `pgbouncer.ini`'s auth file and be allowed in
the admin/users list so `SHOW POOLS` works. It connects to PgBouncer itself
(port 6432), not Postgres.

### 1.4 Firewall

Nothing to open. Alloy only makes outbound HTTPS; scrapes of vm-db happen over
the existing internal network. Confirm the internal route vm-app → vm-db:9127
is reachable (default GCP VPC allows intra-VPC).

### 1.5 Image freshness

The api/worker images must contain the instrumentation (merge that added
`internal/metrics`). Verify by checking the running image tag matches the
commit carrying this change.

## 2. Deployment (vm-app)

vm-app does NOT run the repo from a git checkout. `/home/<deploy-user>/abak-app` is
hand-placed (§1.2) and contains:

```
app-production.yaml   live manifest — a FORK of deploy/compose/production.yml
                      with mount paths adapted to this layout (./alloy/…,
                      ./secrets/…, ./nginx/…)
alloy/config.alloy    same file as deploy/alloy/config.alloy
secrets/              prod-secrets.yaml (hand-placed, gitignored)
nginx/                certs + current nginx conf
.env                  IMAGE_TAG + GRAFANA_CLOUD_* (§1.2)
```

**The repo manifest cannot be copied onto the box verbatim** — its relative
mounts assume the repo layout (`deploy/compose/` as base). Port any upstream
change to `deploy/compose/production.yml` into `app-production.yaml` by hand,
with paths adjusted. After ANY manifest edit, validate before `up -d`:

```sh
docker compose -f app-production.yaml config --quiet && echo OK
```

Don't rewrite YAML with `sed` patterns containing `.` wildcards — one slipped
edit turned `/etc/alloy/…` into `/e./alloy/…`. Read the diff after every edit.

The rollout was staged, not single-step:

1. **2026-08-29 — monitoring trio live.** `app-production.yaml` gained
   node-exporter/cadvisor/alloy; `docker compose -f app-production.yaml up -d`
   created exactly those three, six production services untouched (no restart).
   App images are still `9556e98e…` (pre-instrumentation), so
   `up{job="api",job="worker"}` is 0 until step 2 — expected.
2. **Pending — image bump.** Gate: `images-backend` + `images-web-prod` green
   for `28f165b9…`, `.env` pointing at that SHA. Then:
   ```sh
   docker compose -f app-production.yaml up -d api worker web
   ```
   Brief restart of the three; nginx/redis/gotenberg untouched.
3. **Pending — nginx JSON access logs**, separately at a quiet moment:
   ```sh
   docker compose -f app-production.yaml up -d --force-recreate nginx
   ```

## 3. Post-deployment verification

Work through in order; each gates the next.

1. **Containers**: all nine services `Up`, alloy logs show
   `"finished complete graph evaluation"` and no repeated errors:
   `docker compose -f deploy/compose/production.yml ps`
2. **Targets in Grafana Cloud**: Explore → Prometheus → query `up`.
   `node_exporter` and `cadvisor` = 1 immediately; `api`/`worker` = 1 only
   after the §2 image bump (they are 0 before it — the old image has no
   `:9102` server); `pgbouncer` stays 0 (§1.3 deferred;
   `PGBOUNCER_EXPORTER_ADDRESS` default `10.20.0.10:9127`).
3. **App series**: `http_requests_total{route="/api/v1/auth/login"}` grows when
   anyone logs in; `exam_sessions_active` exists (worker's DB count).
4. **Logs**: Explore → Loki → `{service="api", env="prod"}` returns lines
   from day one (`env`/`service` labels come from alloy, not the image); the
   enriched fields `route`, `params`, `duration_ms`, `request_id` appear only
   after the §2 image bump. `{env="prod"} | json | level=~"(?i)error"` is
   empty-ish but functional.
5. **Dashboard**: import `deploy/grafana/dashboards/exam-day.json`
   (Dashboards → Import), pick the Cloud datasources (UIDs match the local
   provisioning, panels bind automatically), set the `env` variable to `prod`.
6. **Alerts** — configure in Grafana Cloud Alerting, then FIRE EACH ONE ONCE
   deliberately (issue #98 rejects never-tested alerts):

   | Alert | Condition | How to trigger deliberately |
   |---|---|---|
   | Pool starvation | `rate(dbpool_empty_acquire_total[5m]) > 0` sustained during an active exam window | temporarily set `DB_MAX_CONNS=2` env on api + restart during a quiet period with light traffic, watch it fire |
   | Answers p95 too slow | `histogram_quantile(0.95, sum by (le)(rate(http_request_duration_seconds_bucket{route="/api/v1/exam/sessions/:id/answers"}[5m]))) > <agreed threshold>` for 10m | threshold agreed with team first; trigger via load test (#95) |
   | API CPU pinned | container_cpu for `api` > 80% of one core for 15m | stress the answers endpoint in staging-like traffic |
   | 5xx rate | `sum(rate(http_requests_total{status=~"5.."}[5m])) > X` | point a test request at a route that 500s in dev parity |

7. **Record the footprint** — issue #98 acceptance requires measured numbers,
   not assumptions. After ~24h of runtime append them here:

   ```
   ## Footprint on vm-app (measured)
   date:        YYYY-MM-DD
   alloy:       RSS ____ MB, cpu ____
   cadvisor:    RSS ____ MB, cpu ____
   node-exporter: RSS ____ MB
   free RAM delta before/after: ____ GB
   outbound bytes/day to Grafana Cloud: ____ MB
   ```

   (`docker stats --no-stream` per container.)

## 4. Rollback

Monitoring-only rollback is additive-inverse:

```sh
docker compose -f deploy/compose/production.yml stop alloy cadvisor node-exporter
docker compose -f deploy/compose/production.yml rm -f alloy cadvisor node-exporter
```

Nothing else references these services; app behaviour is unaffected. Reverting
the instrumented api/worker image is the normal IMAGE_TAG rollback.

## 5. Routine care

- Bump pinned upstream images occasionally (alloy/cadvisor/node-exporter) via
  a PR changing the tags in `deploy/compose/production.yml`; verify locally
  through the same profile first.
- Watch Grafana Cloud usage vs free tier (series count grows mainly if new
  high-cardinality labels appear — route templates are safe, raw URIs are not).
- Log retention lives in Grafana Cloud; the local json-file buffers stay at
  30 MB/container by design.
- Rotate the Grafana Cloud tokens periodically — procedure in §6.

## 6. Security notes & token rotation

`docker compose config` interpolates every `${…}` — printing or pasting its
output exposes the `GRAFANA_CLOUD_*` secrets verbatim. Before sharing, redact:

```sh
docker compose -f app-production.yaml config --no-interpolate
# or list only the variable NAMES:
grep -E '^(IMAGE_TAG|GRAFANA_CLOUD_[A-Z_]+)=' .env | cut -d= -f1
```

Incident 2026-08-29: a Loki token (`glc_…`) leaked through a pasted `config`
output and was rotated the same day (revoke + regenerate in the portal, `.env`
updated, `alloy` recreated). Portal tokens are shown only once at creation —
store them in a secrets manager, never in chat or tickets.

Rotation procedure (routine rotation uses the same steps):

1. Grafana Cloud portal → Security → Access policies/tokens → revoke the old
   token, create a new one (scopes `metrics:write` + `logs:write`).
2. Update `GRAFANA_CLOUD_PROM_API_KEY` / `GRAFANA_CLOUD_LOKI_API_KEY` in the
   box `.env` (§1.2).
3. Recreate the collector: `docker compose -f app-production.yaml up -d alloy`
4. Verify: alloy logs show no auth errors; Explore returns fresh data.

## 7. Deployment log

- **2026-08-29** — monitoring trio live on vm-app: node-exporter v1.12.1,
  cadvisor v0.60.5, alloy v1.19.0, via the hand-placed `app-production.yaml`
  (§2). Six production services untouched; app images remain `9556e98e…`
  (pre-instrumentation). pgbouncer_exporter deferred (§1.3). Loki API token
  rotated after a `docker compose config` paste leak (§6).
- **Pending**: image bump `28f165b9…` (§2 step 2), nginx JSON access-log
  recreate (§2 step 3), four alert rules + one deliberate fire each (§3.6),
  24 h footprint numbers into §3.7.
