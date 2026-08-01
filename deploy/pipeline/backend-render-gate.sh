#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

# NFR-R4/FR-30: certificates render exclusively through the print route now
# (Task 13 deleted the Go HTML-builder call site) — the gate must exercise
# that route for real, which means a real `web` (serves /documents/certificate
# and /documents/card), `api` (mints/redeems the print token, resolves
# print-data) and `gotenberg` (fetches and rasterizes the route), wired
# together exactly as deploy/compose/local.yml runs them everywhere else.
# Standing the stack up here — not inside the Go test — is what makes it a
# hard CI precondition instead of a machine-specific nicety; see
# certificate_printroute_gate_test.go for the test side of this gate.
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

docker compose -f deploy/compose/local.yml up -d --build \
  postgres redis minio createbuckets gotenberg api web

# api/web carry no compose healthcheck of their own (only postgres/redis/
# gotenberg do), so poll their HTTP ports directly rather than let the test
# race container start. No -f: this only needs to know the server is
# listening, not that a bare request to it succeeds — since the NFR-R1 fix,
# a tokenless GET on /documents/certificate correctly 404s (see
# documents/certificate/page.tsx's notFound() call), which -f would have
# reported as "not ready" forever.
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
wait_for http://localhost:8080/api/v1/health api
wait_for http://localhost:3000/documents/certificate web

cd backend

# FR-6/FR-30 acceptance gate: render a certificate through a real Gotenberg
# fetching the real running print route, not the fake renderer the unit tests
# use, and not buildCertificateHTML called in-process. Kept out of backend.sh
# so the main suite is not slowed by pulling the Chromium image or building
# the web/api images. Matches every gate test by prefix (TestCertificateRender_*,
# TestCardRender_*), not one by name — a new gate test must not silently sit
# unrun.
go test -tags gotenberg_integration -run 'TestCertificateRender_|TestCardRender_' -count=1 -v ./internal/service/
