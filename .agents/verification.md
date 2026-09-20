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

## Backend Gates

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go build ./...
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go vet ./...
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test ./...
```

CI uses the heavier race gate:

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && go test -race -shuffle=on -timeout 20m ./...
```

## Frontend Gates

```bash
npx tsc --noEmit
npm run test:run
npm run build
```

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
