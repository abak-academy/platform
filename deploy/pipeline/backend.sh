#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../../backend"

go build ./...
go vet ./...
# -timeout: Go defaults to 10m PER PACKAGE, which internal/service now exceeds on CI.
# It runs ~361s locally with -race and the runners are slower with no warm image cache.
# The cost is testcontainers: region_papua_migration_test.go starts five Postgres
# containers (one per test, each replaying all 45 migrations) because those tests need
# different migration states and -shuffle=on rules out sharing mutable state. Sharing a
# container across the three read-only ones is the real optimisation -- tracked, not done here.
go test -race -shuffle=on -timeout 20m ./...
