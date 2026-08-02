#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

# NFR-R4/FR-30 (async redesign 2026-08-02): certificates and cards render
# exclusively through GenerateCertificatePDF / generateExamCardPDF now — the
# FE-authored, {{token}}-carrying template substituted with verified DB
# values and posted directly to Gotenberg's /forms/chromium/convert/html.
# Nothing here asks Gotenberg to fetch a URL, so this gate needs only
# Postgres, MinIO, and Gotenberg — no `api`, `web`, or Redis container.
# See certificate_render_gate_test.go for the test side of this gate.
#
# backend/config/env/dev/ is gitignored (.gitignore:20) — bootstrap it from
# the committed example templates, whose values already match
# deploy/compose/local.yml, exactly like a fresh local dev setup would.
# mkdir -p is load-bearing on CI: the directory is ignored, so it does not
# exist in a fresh checkout at all and the cp below fails with "No such file
# or directory". A dev machine has it already, which is why this passed
# locally and failed on the first CI run.
mkdir -p backend/config/env/dev
if [ ! -f backend/config/env/dev/config.yaml ]; then
  cp backend/config/env/config.example.yaml backend/config/env/dev/config.yaml
fi
if [ ! -f backend/config/env/dev/secrets.yaml ]; then
  cp backend/config/env/secrets.example.yaml backend/config/env/dev/secrets.yaml
fi

docker compose -f deploy/compose/local.yml up -d postgres minio createbuckets gotenberg

wait_for() {
  local url="$1" name="$2"
  for _ in $(seq 1 60); do
    if curl -s -o /dev/null "$url"; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $name at $url" >&2
  return 1
}
wait_for http://localhost:9000/minio/health/live minio
wait_for http://localhost:3001/health gotenberg

cd backend

# FR-6/FR-30 acceptance gate: render a certificate/card through a real
# Gotenberg, not the fake renderer the unit tests use. Kept out of backend.sh
# so the main suite is not slowed by pulling the Chromium image. Matches
# every gate test by prefix (TestCertificateRender_*, TestCardRender_*), not
# one by name — a new gate test must not silently sit unrun.
go test -tags gotenberg_integration -run 'TestCertificateRender_|TestCardRender_' -count=1 -v ./internal/service/
