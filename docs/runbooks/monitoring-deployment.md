# Monitoring rollout — pre & post deployment (issue #98)

Step-by-step for putting the observability stack live. Design decisions and the
metric inventory live in
[`docs/backlog/observability-grafana-cloud.md`](../backlog/observability-grafana-cloud.md);
this file is the operator path.

Two distinct rollouts:

- **Local repro** — no cloud account needed, do it any time.
- **Production** — needs a Grafana Cloud stack, one manual install on vm-db,
  and one alert-fire drill before exam day.

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

1. Create/choose a Grafana Cloud account; create a free **stack** in the region
   nearest to `asia-southeast2` (Singapore).
2. From the stack's details page collect:
   - Prometheus **Remote Write** endpoint + instance id
   - Loki push URL + tenant id
   - One API token (Access policy scoped to both endpoints) used as password
     for both pipelines.
3. Free-tier quotas to keep in mind: 10k active series, 50 GB logs/month.

### 1.2 vm-app credentials

Edit the hand-placed `deploy/.env` on the box (gitignored; pattern in
[deploy/.env.example](../../deploy/.env.example)):

```
GRAFANA_CLOUD_PROM_URL=https://prometheus-prod-xx-xxx.grafana.net/api/prom/push
GRAFANA_CLOUD_PROM_INSTANCE_ID=<numeric>
GRAFANA_CLOUD_PROM_API_KEY=<token>
GRAFANA_CLOUD_LOKI_URL=https://logs-prod-xxx.grafana.net/loki/api/v1/push
GRAFANA_CLOUD_LOKI_TENANT_ID=<numeric>
GRAFANA_CLOUD_LOKI_API_KEY=<token>
```

### 1.3 vm-db: install pgbouncer_exporter (one-time, native)

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

```sh
cd <repo-on-box> && git pull
IMAGE_TAG=<same-as-running> docker compose -f deploy/compose/production.yml up -d
```

Additive by design: compose creates node-exporter, cadvisor, alloy only; the
six production services are untouched (no restart, no downtime). nginx picks
up the JSON access-log format on its next recreate — do that separately at a
quiet moment if desired:

```sh
docker compose -f deploy/compose/production.yml up -d --force-recreate nginx
```

## 3. Post-deployment verification

Work through in order; each gates the next.

1. **Containers**: all nine services `Up`, alloy logs show
   `"finished complete graph evaluation"` and no repeated errors:
   `docker compose -f deploy/compose/production.yml ps`
2. **Targets in Grafana Cloud**: Explore → Prometheus → query `up`.
   Expect 1 for `api`, `worker`, `node_exporter`, `cadvisor`; `pgbouncer` = 1
   only after step 1.3 succeeded (check `PGBOUNCER_EXPORTER_ADDRESS`,
   default `10.20.0.10:9127`).
3. **App series**: `http_requests_total{route="/api/v1/auth/login"}` grows when
   anyone logs in; `exam_sessions_active` exists (worker's DB count).
4. **Logs**: Explore → Loki → `{service="api", env="prod"}` returns enriched
   request lines carrying `route`, `params`, `duration_ms`, `request_id`;
   `{env="prod"} | json | level=~"(?i)error"` is empty-ish but functional.
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
