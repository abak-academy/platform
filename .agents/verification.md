# Verification

Use the smallest verification that proves the change. For broad repo claims, use the full gates below.

## Go Environment

This machine's default `GOROOT` can be wrong. Prefix Go commands:

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec
```

When Go may write build cache from a sandboxed or frontend-spawned process, also set:

```bash
export GOCACHE=/private/tmp/akademi-go-cache
```

Testcontainers does not read the docker CLI context. For any test that starts Postgres locally (OrbStack):

```bash
export DOCKER_HOST=unix:///Users/Panca/.orbstack/run/docker.sock
```

An instant (~2s) `rootless Docker not found` failure means this is missing, not a broken test.

## Backend Gates

The CI gate is the script itself; run it rather than a hand-picked subset:

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && ./deploy/pipeline/backend.sh
```

It runs `go build ./...`, `go vet ./...`, and `go test -race -shuffle=on -timeout 20m ./...` from `backend/`. It takes over 10 minutes locally, so run it in the background and check its exit code. Use `./...`, never `./internal/...`; `backend/integration/` is a sibling of `internal/` and holds stubs that break on interface changes.

For focused loops, run the package under change with `-run`, then its whole package.

## Frontend Gates

CI runs `deploy/pipeline/frontend.sh` (`npm ci`, `npx tsc --noEmit`, `npx vitest run`). Locally, from `web/`, after the last edit, including test-file edits:

```bash
npx tsc --noEmit
npx vitest run
npm run build
```

- `tsc --noEmit` is the only gate that type-checks `*.test.tsx`; vitest never type-checks.
- `npm run build` is the only gate for Next page-export rules; CI does not run it on branches.
- `npm run lint` cannot run (no eslint config); do not use it as a gate.
- Layout, overflow, and clipping need a real browser; jsdom reports zero geometry.

For the full frontend suite on this machine, prefer:

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && export GOCACHE=/private/tmp/akademi-web-go-cache && npm run test:run
```

## Repo Hygiene

```bash
git status --short
git diff --check
```

Do not call the repo clean or verified if required commands were skipped. State exactly what was and was not run.
