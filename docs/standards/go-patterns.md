# Go Patterns

Target backend structure for new code and scoped cleanup. Existing drift is technical debt, not permission to spread it. Do not refactor adjacent code unless the task explicitly includes that cleanup.

## Layer ownership

- Domain and persistent concepts live in `backend/internal/model`.
- Business decisions and orchestration live in `backend/internal/service`.
- Repositories query and persist data; they do not own business policy.
- Handlers translate HTTP input/output and map service errors to responses.
- Provider-specific wire shapes stay in `backend/internal/adapter` behind typed service boundaries.
- Migrations use paired up/down files and receive coverage when behavior depends on schema details.

Review question: if one business rule changes, is its decision concentrated in one clear layer?

## Error direction

- Define reusable service errors in `backend/internal/service/errors.go`.
- Map service errors to HTTP responses in `backend/internal/handler/errors.go`.
- Keep provider-specific wrapped errors in adapters unless the service must branch on them.
- Avoid one-off error-string matching across layers.

Existing scattered errors are cleanup candidates. New code should not add more unless the error is truly local and unshared.

## Model and DTO direction

- Persistent and domain concepts belong in `internal/model`.
- Handler-only request bodies may stay local to handlers.
- Response structs shared across handlers, services, or frontend contracts should be named and placed deliberately.
- Do not create a new package for one struct or helper.
- If a struct becomes reused or represents stored business state, move it toward `internal/model` in a scoped cleanup.

## Interface and abstraction restraint

- Add an interface only at a proven seam or external dependency boundary.
- Prefer consumer-owned, narrow interfaces.
- Do not create abstractions for hypothetical variants.
- Do not split small logic into many packages.
- Preserve the local package shape when changing unrelated behavior.

## Cleanup workflow

1. Name one structural problem: errors, model placement, DTO shape, repository logic, or provider contract.
2. Add or identify tests that pin current behavior.
3. Move structure without hiding behavior changes.
4. Run focused package tests.
5. Run broader gates when the change crosses handler, service, repository, adapter, or migration boundaries.

## Non-goals

- Do not rewrite the backend into a new architecture during feature work.
- Do not move structs or create interfaces only for aesthetics.
- Do not hide a behavior change inside a cleanup task.
- Do not normalize unrelated legacy code while touching one path.