# Go Pattern Cleanup

This pack defines the target backend shape. It also documents current implementation drift as tech debt, not as permission to keep spreading it.

## Current Tech Debt

The current codebase is useful but loose in several Go patterns:

- Error definitions and mappings are scattered outside a single error boundary.
- Request/response structs and small DTOs appear in many handlers and services.
- Domain-ish structs appear outside `internal/model`.
- Some tests mirror production logic in shims instead of exercising the real service directly.
- External-provider uncertainty is sometimes handled with local comments and fake-shaped tests instead of a captured contract fixture.

When touching existing code for an unrelated task, do not clean all of this opportunistically. Preserve behavior and scope.

## Target Pattern

- Domain models live in `backend/internal/model`.
- Business decisions live in `backend/internal/service`.
- Repositories persist and query data; they should not own business rules.
- Handlers translate HTTP to service calls and service errors to HTTP responses.
- Shared service errors should be centralized and mapped consistently.
- Provider adapters should expose typed service ports and keep provider-specific wire shapes inside `backend/internal/adapter`.
- Migrations must be paired up/down files and covered when behavior depends on schema details.

## Error Direction

Preferred shape:

- Define reusable service errors in `backend/internal/service/errors.go`.
- Map service errors to HTTP responses in `backend/internal/handler/errors.go`.
- Keep provider-specific wrapped errors in adapters only when the service layer does not need to branch on them.
- Avoid one-off string matching across layers.

Existing scattered errors are cleanup candidates. New code should not add more unless the error is truly local and unshared.

## Model And DTO Direction

Preferred shape:

- Persistent/domain concepts go in `internal/model`.
- Handler-only request bodies may stay local to handlers.
- Response structs shared across handlers, services, or frontend contracts should be named and placed deliberately.
- Do not create a new package for one struct or one helper.

If a struct starts being reused across files or represents stored business state, move it toward `internal/model` in a scoped cleanup.

## Cleanup Workflow

1. Pick one pattern debt only: errors, model placement, DTO shape, repository logic, or provider contract.
2. Write or identify tests that pin current behavior.
3. Move the structure without changing behavior.
4. Run focused package tests.
5. Run broader gates if the cleanup crosses handler, service, repository, or migration boundaries.

## Non-Goals

- Do not rewrite the whole backend into a new architecture.
- Do not split small logic into many packages.
- Do not move structs only for aesthetics.
- Do not hide behavior changes inside a cleanup PR.
