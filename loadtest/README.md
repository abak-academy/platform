# Exam lifecycle load test

This suite runs one complete student lifecycle per virtual user:

`login -> registration lookup -> optional check-in -> start -> cumulative autosaves -> section advance -> reconnect verification -> submit`

It refuses to run unless `NON_PRODUCTION_CONFIRM=loadtest`. Use an isolated database with synthetic data only.

## Current evidence

The harness has passed a 1-VU local smoke test against a mock API, including seed verification against disposable PostgreSQL. This proves the script plumbing only. No 100-, 500-, or higher-user application capacity test has run yet.

## Prerequisites

- Docker is running locally.
- The non-production API and PostgreSQL database belong to the same environment.
- The target exam has representative tests and questions.
- Use a standard exam with `requires_checkin=false` for the first baseline.
- The API and database URLs are reachable from Docker. For services on the Mac, use `host.docker.internal` instead of `localhost`.

Start from the repository root:

```bash
cd /path/to/platform
mkdir -p loadtest/results
```

Set the target values:

```bash
export LOADTEST_DATABASE_URL='postgres://user:password@db-host:5432/abak_loadtest'
export LOADTEST_BASE_URL='https://loadtest.example.com'
export EXAM_ID='00000000-0000-0000-0000-000000000000'
export LOADTEST_PASSWORD='replace-with-a-test-only-password'
```

Confirm the database name before seeding:

```bash
docker run --rm postgres:16-alpine \
  psql "$LOADTEST_DATABASE_URL" -Atc 'select current_database()'
```

Use the returned name as `confirm_db` below. The seed refuses to run when it does not exactly match PostgreSQL `current_database()`.

## Seed a run

Every stage needs a fresh `RUN_ID`. The seed creates one synthetic account and exam registration per VU.

```bash
export RUN_ID='smoke_20260830'
export USERS=1

docker run --rm \
  -e LOADTEST_PASSWORD \
  -v "$PWD:/repo:ro" \
  postgres:16-alpine \
  psql "$LOADTEST_DATABASE_URL" \
  -v confirm_db=abak_loadtest \
  -v exam_id="$EXAM_ID" \
  -v run_id="$RUN_ID" \
  -v user_count="$USERS" \
  -f /repo/loadtest/seed.sql
```

Re-running the seed with the same `RUN_ID` resets that run's sessions and registrations. This is useful after a failed or interrupted test.

## Run a smoke test

Run one user with no human pacing:

```bash
docker run --rm \
  -v "$PWD/loadtest:/scripts:ro" \
  -v "$PWD/loadtest/results:/results" \
  -e NON_PRODUCTION_CONFIRM=loadtest \
  -e BASE_URL="$LOADTEST_BASE_URL/api/v1" \
  -e EXAM_ID \
  -e RUN_ID \
  -e LOADTEST_PASSWORD \
  -e USERS \
  -e LOGIN_SPREAD_SECONDS=0 \
  -e ANSWER_INTERVAL_SECONDS=0 \
  -e ANSWER_JITTER_SECONDS=0 \
  -e K6_WEB_DASHBOARD=true \
  -e K6_WEB_DASHBOARD_PORT=-1 \
  -e K6_WEB_DASHBOARD_EXPORT="/results/${RUN_ID}-${USERS}.html" \
  grafana/k6:latest run \
  --summary-export="/results/${RUN_ID}-${USERS}.json" \
  /scripts/exam-lifecycle.js
```

Open the reports:

```bash
open "loadtest/results/${RUN_ID}-${USERS}.html"
jq . "loadtest/results/${RUN_ID}-${USERS}.json"
```

## Run the 100-user baseline

Use a new run and repeat the seed command with 100 users:

```bash
export RUN_ID='baseline100_20260830'
export USERS=100
```

After seeding, run with human pacing and a synchronized submit time:

```bash
docker run --rm \
  -v "$PWD/loadtest:/scripts:ro" \
  -v "$PWD/loadtest/results:/results" \
  -e NON_PRODUCTION_CONFIRM=loadtest \
  -e BASE_URL="$LOADTEST_BASE_URL/api/v1" \
  -e EXAM_ID \
  -e RUN_ID \
  -e LOADTEST_PASSWORD \
  -e USERS \
  -e LOGIN_SPREAD_SECONDS=60 \
  -e ANSWER_INTERVAL_SECONDS=45 \
  -e ANSWER_JITTER_SECONDS=10 \
  -e SUBMIT_AT_SECONDS=3600 \
  -e K6_WEB_DASHBOARD=true \
  -e K6_WEB_DASHBOARD_PORT=-1 \
  -e K6_WEB_DASHBOARD_EXPORT="/results/${RUN_ID}-${USERS}.html" \
  grafana/k6:latest run \
  --summary-export="/results/${RUN_ID}-${USERS}.json" \
  /scripts/exam-lifecycle.js
```

`SUBMIT_AT_SECONDS` is measured from test setup. Set it longer than:

`LOGIN_SPREAD_SECONDS + question count * (ANSWER_INTERVAL_SECONDS + ANSWER_JITTER_SECONDS) + buffer`

Set it to `0` when synchronized submit is not part of the test.

## Increase the load

Use a fresh `RUN_ID`, seed, run, and review at each stage:

`1 smoke -> 100 -> 500 -> 1000 -> 2500 -> 5000 -> 7000`

Do not continue to the next stage unless all of these are true:

- `completed_lifecycles` equals `USERS`.
- `lifecycle_failed` is below 1%.
- `lost_answers` is zero.
- Autosave p95 is below 300 ms.
- Login, start, reconnect, and submit thresholds pass.
- The k6 generator is not CPU, memory, or network saturated.
- API and database evidence still show headroom.

The current shared-IP login limiter may make the 100-user baseline fail during login. That result confirms the known blocker; it is not a k6 failure.

## Interpret the result

The HTML report is for human review. The JSON summary is for comparison and automation. Neither can identify the server bottleneck alone.

A capacity report must pair k6 results with API CPU, database CPU, connection-pool pressure, restarts, and network throughput. Name the saturated resource rather than reporting p95 alone.

For a check-in exam, its configured window must be open and `REQUIRES_CHECKIN=true` must be passed to k6.
