# Exam lifecycle load test

This suite runs one complete student lifecycle per virtual user:

`login -> token refresh as needed -> registration lookup -> optional check-in -> start -> cumulative autosaves -> section advance -> reconnect verification -> submit`

Each VU keeps its refresh token and rotates the access and refresh tokens before the access token expires. This keeps long-running exams on the same authenticated session and stable `X-Forwarded-For` identity.

It refuses to run unless `NON_PRODUCTION_CONFIRM=loadtest`. Use an isolated database with synthetic data only.

## Current evidence

The harness has passed a 1-VU local smoke test against a mock API, including seed verification against disposable PostgreSQL. This proves the script plumbing only. No 100-, 500-, or higher-user application capacity test has run yet.

## Prerequisites

- Docker is running locally.
- The non-production API and PostgreSQL database belong to the same environment.
- The target exam has representative tests and questions.
- Use a standard exam with `requires_checkin=false` for the first baseline.
- The API and database URLs are reachable from Docker. For services on the Mac, use `host.docker.internal` instead of `localhost`.

Start from the repository root and create the local configuration:

```bash
cd /path/to/platform
cp loadtest/.env.example loadtest/.env.local
```

`loadtest/.env.local` is ignored by Git. Edit it with the database, API, exam, and workload values. Shell-quote values containing spaces or special characters.

The smoke-test defaults are:

```dotenv
RUN_ID=smoke_20260830
USERS=1
LOGIN_SPREAD_SECONDS=0
ANSWER_INTERVAL_SECONDS=0
ANSWER_JITTER_SECONDS=0
SUBMIT_AT_SECONDS=0
```

`CONFIRM_DB` must exactly match `PGDATABASE`. The SQL seed also verifies it against PostgreSQL `current_database()` before writing anything.

For a laptop without working IPv6, set `K6_DNS='ttl=inf,select=roundRobin,policy=onlyIPv4'`. If the laptop network cannot maintain thousands of idle connections, `K6_NO_CONNECTION_REUSE=true` closes each request's connection. Record this setting with the result: it adds connection/TLS overhead between the generator and the target, so client latency is not directly comparable to keep-alive runs.

`CONTINUE_TRANSPORT_ERRORS=true` enables at most three retries (four attempts total) for GET requests with transport status 0, with 1/2/4-second backoff plus jitter. Login also has at most three transport retries, regardless of this flag. Autosave uses only `SAVE_RETRIES` (default three), replays the same answer payload and position, and adds jitter to its existing backoff; there is no nested request-level retry. HTTP error responses are not retried by these transport loops. The existing one-time token refresh on HTTP 401 remains in place.

Refresh, start, check-in, section advance, and submit are not automatically retried after transport failure. Refresh rotates and deletes the old token; a lost response makes replay unsafe. Start can create an attempt, and submit may have committed even when its response was lost. A failed refresh stops the current operation rather than replaying the token through autosave retries. Recovering these ambiguous mutations requires separate reconciliation logic, not blind retry.

Every failed HTTP attempt remains in k6 metrics, and `transport_retries` counts scheduled retries by phase. Retries do not reset the submit clock or `MAX_DURATION`; time spent in requests still consumes the run budget. This can recover a lifecycle despite a failed request-error gate. A transport failure does not identify its cause, and a VU retrying a request does not prove an active server session. No load-test capacity is certified by the local harness tests.

## Seed a run

Every stage needs a fresh `RUN_ID`. The seed creates one synthetic account and exam registration per VU:

```bash
./loadtest/run.sh seed
```

The output ends with the confirmed `run_id`, `exam_id`, and `users_seeded` count. Re-running the seed with the same `RUN_ID` resets that run's sessions and registrations.

## Run a smoke test

Run one user with no human pacing:

```bash
./loadtest/run.sh test
```

Open the reports:

```bash
open loadtest/results/smoke_20260830-1.html
jq . loadtest/results/smoke_20260830-1.json
```

## Run the 100-user baseline

Edit `loadtest/.env.local` for a new run:

```dotenv
RUN_ID=baseline100_20260830
USERS=100
LOGIN_SPREAD_SECONDS=60
ANSWER_INTERVAL_SECONDS=45
ANSWER_JITTER_SECONDS=10
SUBMIT_AT_SECONDS=3600
```

Then seed and run:

```bash
./loadtest/run.sh seed
./loadtest/run.sh test
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
- Token refresh error rate is below 1%.
- Login, start, reconnect, and submit thresholds pass.
- The k6 generator is not CPU, memory, or network saturated.
- API and database evidence still show headroom.

Each VU sends one stable synthetic `X-Forwarded-For` address for its whole lifecycle, including check-in. This keeps the shared generator IP from collapsing the login test into the per-IP limiter while preserving device identity.

## Interpret the result

The HTML report is for human review. The JSON summary is for comparison and automation. Neither can identify the server bottleneck alone.

A capacity report must pair k6 results with API CPU, database CPU, connection-pool pressure, restarts, and network throughput. Name the saturated resource rather than reporting p95 alone.

`run.sh test` prints both intended report paths and returns k6's original exit status, including exit 99 when thresholds fail.

For a check-in exam, its configured window must be open and `REQUIRES_CHECKIN=true` must be passed to k6.
