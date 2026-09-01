#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
env_file=${2:-"$repo_root/loadtest/.env.local"}

if [ ! -f "$env_file" ]; then
  echo "missing env file: $env_file" >&2
  echo "copy loadtest/.env.example to loadtest/.env.local" >&2
  exit 2
fi

set -a
. "$env_file"
set +a

: "${EXAM_ID:?set EXAM_ID in $env_file}"
: "${RUN_ID:?set RUN_ID in $env_file}"
: "${USERS:?set USERS in $env_file}"
: "${LOADTEST_PASSWORD:?set LOADTEST_PASSWORD in $env_file}"

case "${1:-}" in
  seed)
    : "${PGHOST:?set PGHOST in $env_file}"
    : "${PGPORT:?set PGPORT in $env_file}"
    : "${PGDATABASE:?set PGDATABASE in $env_file}"
    : "${PGUSER:?set PGUSER in $env_file}"
    : "${PGPASSWORD:?set PGPASSWORD in $env_file}"
    : "${CONFIRM_DB:?set CONFIRM_DB in $env_file}"

    docker run --rm \
      -e PGHOST \
      -e PGPORT \
      -e PGDATABASE \
      -e PGUSER \
      -e PGPASSWORD \
      -e LOADTEST_PASSWORD \
      -v "$repo_root:/repo:ro" \
      postgres:16-alpine \
      psql \
      -v confirm_db="$CONFIRM_DB" \
      -v exam_id="$EXAM_ID" \
      -v run_id="$RUN_ID" \
      -v user_count="$USERS" \
      -f /repo/loadtest/seed.sql
    ;;
  test)
    : "${BASE_URL:?set BASE_URL in $env_file}"
    : "${NON_PRODUCTION_CONFIRM:?set NON_PRODUCTION_CONFIRM in $env_file}"

    report_name=${REPORT_NAME:-"$RUN_ID-$USERS"}
    mkdir -p "$repo_root/loadtest/results"

    set +e
    docker run --rm \
      -v "$repo_root/loadtest:/scripts:ro" \
      -v "$repo_root/loadtest/results:/results" \
      -e NON_PRODUCTION_CONFIRM \
      -e BASE_URL \
      -e EXAM_ID \
      -e RUN_ID \
      -e LOADTEST_PASSWORD \
      -e USERS \
      -e LOGIN_SPREAD_SECONDS \
      -e ANSWER_INTERVAL_SECONDS \
      -e ANSWER_JITTER_SECONDS \
      -e SAVE_RETRIES \
      -e MAX_QUESTIONS \
      -e SUBMIT_AT_SECONDS \
      -e MAX_DURATION \
      -e REQUIRES_CHECKIN \
      -e LOGIN_P95_MS \
      -e START_P95_MS \
      -e AUTOSAVE_P95_MS \
      -e RECONNECT_P95_MS \
      -e SUBMIT_P95_MS \
      -e K6_WEB_DASHBOARD=true \
      -e K6_WEB_DASHBOARD_PORT=-1 \
      -e K6_WEB_DASHBOARD_EXPORT="/results/$report_name.html" \
      grafana/k6:latest run \
      --summary-export="/results/$report_name.json" \
      /scripts/exam-lifecycle.js
    k6_status=$?
    set -e

    echo "HTML: loadtest/results/$report_name.html"
    echo "JSON: loadtest/results/$report_name.json"
    exit "$k6_status"
    ;;
  *)
    echo "usage: loadtest/run.sh <seed|test> [env-file]" >&2
    exit 2
    ;;
esac
