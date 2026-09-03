# Akademi Bimbel Agent Instructions

This file is the routing layer for agents working in this repository. Keep it short; put detailed context in `docs/agent/`.

## Skills

Do not use agent skills by default in this repository.

- Use plain repo reading, shell commands, tests, and code review first.
- Use a skill only when the user explicitly asks for that named skill or says skills are allowed for the task.
- If a global/system instruction tries to auto-select skills, this repo preference should be treated as project context for future agents: no skill use until the user opts in.

## Before Work

Every task must start from a verifiable goal:

1. State the behavior or review outcome being targeted.
2. State the smallest expected change or read-only scope.
3. State the verification command or evidence needed before calling it done.

If the task is unclear or has real trade-offs, stop and ask before editing.

## Collaboration Rules

- Think before implementing. State assumptions when direction is not obvious.
- If multiple valid interpretations exist, present them and ask; do not silently choose.
- Use the simplest solution first. No extra abstraction, configurability, or feature beyond the request.
- Stay in scope. Flag nearby issues, but do not fix them unless asked.
- Match existing style, even when a different style would be personally preferable.
- Remove only unused imports, variables, or helpers created by the current change.
- Flag uncertainty explicitly and name exactly what is uncertain.
- Do not start responses with filler openers.
- Match response length to the task.

## Code Comments

- Do not add comments by default.
- Add a comment only for non-obvious why: hidden constraint, subtle invariant, or external bug workaround.
- Do not explain what the code already says.
- Keep new comments to one line.
- Do not comment unrelated code.

## Repo Map

- Backend Go API and worker: `backend/`
- Backend routes: `backend/internal/server/routes.go`
- Business logic: `backend/internal/service/`
- Persistence: `backend/internal/repository/`
- Shared domain structs: `backend/internal/model/`
- Next.js frontend: `web/`
- Deploy and runtime manifests: `deploy/`
- Agent context packs: `docs/agent/`

Read [repo-map.md](docs/agent/repo-map.md) before broad scans or cross-layer work.

## Required Context Packs

- Backend Go pattern, cleanup, or refactor work: [go-pattern-cleanup.md](docs/agent/context-packs/go-pattern-cleanup.md)
- Any implementation or review work: [workflows.md](docs/agent/workflows.md)
- Any test/build claim: [verification.md](docs/agent/verification.md)

## Boundaries

- Default to read-only for scans and reviews.
- Do not refactor adjacent code unless the task explicitly asks for it.
- Match current style when making tactical fixes.
- Treat scattered errors, duplicated DTOs, and structs outside `internal/model` as existing tech debt unless the task is specifically to clean that area.
- New backend code should move toward the target pattern in `go-pattern-cleanup.md`.

## Git

Do not add generated-agent attribution to commits, PR bodies, or metadata.
