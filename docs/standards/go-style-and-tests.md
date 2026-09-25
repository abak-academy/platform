# Go Style and Tests

Repository-wide Go implementation and unit-test guidance. This document is durable engineering documentation; agent skills reference it but do not own it.

Source inspiration: `https://bitbucket.org/Amartha/loan-platform/src/main/docs/wiki/standards/style/go-style-guide.md`

Authority order:

1. Existing style in the package being changed.
2. `AGENTS.md` and [Go Patterns](go-patterns.md).
3. This document.
4. External source conventions.

Do not import an Amartha-specific rule when it conflicts with this repository.

## Code style

- Prefer clarity and stability over cleverness.
- Match established package style before applying a general preference.
- Pass interfaces as values; do not use pointers to interfaces.
- Introduce an interface only at a real substitution or dependency boundary, preferably owned by the consumer.
- Use pointer receivers when state changes or receiver consistency requires them; use value receivers for small immutable values.
- Copy slices or maps when they expose mutable internal state.
- Use `time.Time` for instants and `time.Duration` for periods.
- Do not shadow built-ins such as `error`, `string`, or `int`.
- Avoid embedding types in public contracts when it leaks implementation details.
- Use explicit serialization tags for persisted or transported structs.
- Avoid `init()`; initialization should be explicit and testable.
- Clean up resources with `defer` at the acquisition boundary.
- Keep new functions small and single-purpose. Extract proven decisions, not arbitrary line counts.

## Errors and failure behavior

- Use sentinel errors when callers need `errors.Is`; use typed errors when callers need `errors.As`.
- Wrap inspectable causes with `%w` and add concise context.
- Do not log an error and return the same error. Handle it once at the appropriate boundary.
- Do not panic in request, worker, repository, or service paths. Return an error.
- Use comma-ok type assertions.
- Centralize reusable service errors and map them consistently at the handler boundary.
- Do not branch on error strings when a typed or sentinel error can express the contract.

## Unit-test style

- Test observable behavior and invariants, not private implementation steps.
- Use table-driven tests when one behavior has multiple cases, branches, invalid inputs, or edge conditions.
- A single focused case does not need an artificial one-row table.
- Give each case a behavioral name and run it with `t.Run`.
- Keep unit tests beside the code in the same package unless black-box package boundaries are intentional.
- Prefer pure domain/service tests without mocks.
- Mock only real external boundaries; avoid long expectation chains that mirror implementation order.
- Exercise the real service method instead of recreating production logic in a test shim.
- Verify returned values, errors, and state changes. A test that only checks that code executed is insufficient.
- Use `errors.Is` or `errors.As` for error assertions when the contract supports them.
- Inject clocks or deterministic values instead of asserting around direct `time.Now()` behavior.
- Use `t.Parallel()` only when fixtures, environment, database rows, and mutable globals are isolated.
- For repository or state-transition behavior, follow the repository's existing Postgres testcontainer pattern and verify persisted side effects.
- Migration-dependent behavior requires paired up/down migrations and the strongest available integration coverage.
- For an external provider, test against a captured real response fixture, not a hand-invented payload shape; if none is available, mark the gap `TODO: uncertain` and say so in the PR.

## Red-proof quality

A valid red test:

1. compiles and reaches the intended assertion;
2. fails because the behavior is missing or wrong;
3. would pass after the smallest correct implementation;
4. is not failing because of setup, unavailable infrastructure, or a broken mock;
5. remains meaningful if internal implementation details change.

Before reporting green, run the focused test, the relevant package suite, and the broader commands required by `.agents/verification.md`.