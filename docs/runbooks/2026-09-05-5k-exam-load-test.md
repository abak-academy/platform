# 5K Exam Load Test Plan

> **For agentic workers:** Execute this operational plan step-by-step and stop at every stated gate.

**Goal:** Measure whether the current `feat/exam-lifecycle-reliability` deployment can preserve exam correctness and operational headroom through 5,000 concurrent student lifecycles, while reporting latency-SLO compliance separately.

**Architecture:** Keep the current production-shaped LT environment unchanged: Cloudflare/nginx, two API containers sharing the 2-vCPU `lt-app`, worker, PgBouncer, and PostgreSQL on `lt-db`. Run a full 2,500-VU safety stage before 5,000 VUs, keeping login arrival near 10.4 users/second and synchronizing submit to exercise the known worst-case burst.

**Tech Stack:** k6, Docker, GCP Compute Engine, nginx, Go API, PgBouncer, PostgreSQL 17, Grafana/Alloy.

**Spec:** `loadtest/README.md`

## Global Constraints

- Load-test environment only: `https://stg.abakacademy.id/api/v1`.
- Production must not be changed or queried by the test harness.
- Deploy SHA remains `021f5a8a1740066a8bf27c4cb6e29650ad0bb212` for both stages.
- Keep exactly two API replicas; no scaling or tuning between stages.
- Use the existing representative standard exam with 25 questions and `REQUIRES_CHECKIN=false`.
- Use a fresh `RUN_ID` per stage.
- Preserve the latency thresholds from `loadtest/.env.example`.
- Do not call a stage certified when any required threshold fails.
- A stage may continue diagnostically when only submit latency fails, but it must be labelled `correctness pass / SLO fail`.

## Baseline

The 1K branch run completed 1,000/1,000 lifecycles with zero lost answers, zero lifecycle failures, and zero container restarts. Login, start, autosave, and reconnect passed their p95 thresholds. Submit p95 was 4,084.66 ms against the 1,500 ms target, so 1K is a correctness pass but not a latency-SLO pass.

### Task 1: Pre-flight and clean baseline

**Evidence:** container images/restarts, endpoint health, database state, generator headroom.

- [ ] Confirm `lt-app` and `lt-db` are running in `asia-southeast2-b`.
- [ ] Confirm both API containers and the worker use SHA `021f5a8a1740066a8bf27c4cb6e29650ad0bb212`.
- [ ] Confirm API, PostgreSQL, and Redis health are `ok`.
- [ ] Record container restart counts, CPU, memory, and network counters.
- [ ] Record PostgreSQL connections, active queries, idle-in-transaction sessions, locks, deadlocks, and available memory.
- [ ] Confirm the local Docker engine has at least 6 GiB free for the 5K generator.
- [ ] Confirm no other local workload will compete with k6.

**Gate:** stop if any container is unhealthy, has unexplained restarts, or the DB has existing blocked/idle-in-transaction work.

### Task 2: Run the 2.5K safety stage

Update only the ignored `loadtest/.env.local` workload values:

```dotenv
RUN_ID=pr164_2500_sync_20260905
USERS=2500
LOGIN_SPREAD_SECONDS=240
ANSWER_INTERVAL_SECONDS=45
ANSWER_JITTER_SECONDS=10
SAVE_RETRIES=3
MAX_QUESTIONS=25
SUBMIT_AT_SECONDS=2100
MAX_DURATION=50m
REQUIRES_CHECKIN=false
REPORT_NAME=pr164_2500_sync_20260905-2500
```

- [ ] Open the temporary local DB tunnel and seed exactly 2,500 synthetic users.
- [ ] Verify the seed reports the expected exam ID, run ID, and 2,500 users.
- [ ] Start monitoring before k6 so the login ramp is captured.
- [ ] Run one full lifecycle per VU.
- [ ] Capture app, DB, PgBouncer, nginx, and generator metrics during login, steady autosave, pre-submit, submit burst, and recovery.
- [ ] At pre-submit, verify 2,500 sessions remain `in_progress` and 62,500 answers exist.
- [ ] After completion, verify status counts and answer counts directly in PostgreSQL.
- [ ] Save the k6 JSON/HTML report and aggregate nginx submit latency/status distribution.

**Immediate abort conditions:**

- Any lost answer is detected.
- HTTP 5xx reaches 1% for two consecutive one-minute windows.
- API or DB CPU stays above 85% for five minutes.
- Memory stays above 80%, swap grows continuously, or an OOM/restart occurs.
- PgBouncer waiting clients or DB lock waits persist for more than 30 seconds.
- Generator CPU exceeds 75% or Docker memory exceeds 70% for five minutes.

**Gate to 5K:**

- `completed_lifecycles = 2500`.
- `lifecycle_failed < 1%`.
- `lost_answers = 0` and PostgreSQL contains 62,500 answers.
- Autosave p95 remains below 300 ms.
- Login, start, reconnect, refresh-error, and request-error gates pass.
- No OOM, restart, deadlock, sustained wait, or generator saturation.
- If submit p95 alone exceeds 1,500 ms, continue only as a diagnostic 5K capacity run and keep the final status explicitly `SLO fail`.

### Task 3: Run the 5K stage

Use a new seed and preserve the same login arrival rate:

```dotenv
RUN_ID=pr164_5000_sync_20260905
USERS=5000
LOGIN_SPREAD_SECONDS=480
ANSWER_INTERVAL_SECONDS=45
ANSWER_JITTER_SECONDS=10
SAVE_RETRIES=3
MAX_QUESTIONS=25
SUBMIT_AT_SECONDS=2400
MAX_DURATION=60m
REQUIRES_CHECKIN=false
REPORT_NAME=pr164_5000_sync_20260905-5000
```

- [ ] Re-run the complete pre-flight without changing container count or limits.
- [ ] Seed exactly 5,000 new synthetic users.
- [ ] Start k6 and continuous monitoring before the first login.
- [ ] During the eight-minute login ramp, verify the generator and API are not the bottleneck.
- [ ] During steady autosave, capture snapshots at minutes 10, 20, and 30.
- [ ] At minute 39, verify 5,000 `in_progress` sessions and 125,000 answers.
- [ ] From minute 40 through recovery, capture submit latency, status distribution, app/DB CPU, pool waits, locks, and container restarts.
- [ ] After k6 exits, verify 5,000 submitted sessions and 125,000 answers directly in PostgreSQL.
- [ ] Confirm endpoint health and zero unexplained restarts after recovery.

**5K certification gate:**

- `completed_lifecycles = 5000`.
- `lifecycle_failed < 1%`.
- `lost_answers = 0`.
- PostgreSQL confirms 5,000 submitted sessions and 125,000 answers.
- Login p95 below 1,000 ms.
- Start and reconnect p95 below 1,500 ms.
- Autosave p95 below 300 ms.
- Submit p95 below 1,500 ms.
- Refresh and per-phase HTTP error rates below 1%.
- No sustained resource saturation, pool/lock waits, OOM, restart, deadlock, or generator bottleneck.

### Task 4: Produce the capacity verdict

- [ ] Compare 1K, 2.5K, and 5K in one table: completions, lifecycle failures, lost answers, per-phase p95, HTTP failures, API/DB peak resources, pool waits, and restarts.
- [ ] Label each stage independently as `pass`, `correctness pass / SLO fail`, or `fail`.
- [ ] Name the highest certified stage; do not extrapolate beyond it.
- [ ] If submit is the only failed gate, report its median/p95/max and correlate them with nginx upstream time, DB waits, and API pool pressure before proposing a code change.
- [ ] Keep the generated reports and exact workload configuration as evidence.

## Optional realism run

The synchronized-submit test is deliberately harsher than a normal exam. After the capacity run, a separate realistic 5K test may randomize submits across 5–10 minutes. That requires a small loadtest-only `SUBMIT_SPREAD_SECONDS` addition and separate approval; its result must not replace the synchronized-burst result.
