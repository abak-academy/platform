---
name: tdd-implementation
description: Implement one approved task with red-green-refactor discipline.
---

# TDD implementation

Use this skill for an approved task that changes production behavior or code. It operates inside the contract established by `development-workflow`; it does not redesign product behavior, rewrite acceptance criteria, or expand scope.

## Inputs

The executor must receive:

- repository path and branch;
- committed spec path and relevant sections, or for a small change below the size gate, the stated goal and non-goals;
- one task ID and outcome;
- completed dependencies;
- exact allowed scope;
- focused verification requirement;
- repository instructions and relevant context pack.

If these inputs are missing or contradictory, return `blocked` rather than guessing.

For Go implementation or Go unit tests, read and follow:

- [`docs/standards/go-patterns.md`](../../../docs/standards/go-patterns.md)
- [`docs/standards/go-style-and-tests.md`](../../../docs/standards/go-style-and-tests.md)

For `web/` implementation or tests, read and follow [`docs/standards/web-tests.md`](../../../docs/standards/web-tests.md).

Existing package style remains authoritative when it is more specific.

## Rules

- Simple solution first.
- Minimal diff.
- Match existing project style.
- Do not touch unrelated files.
- Do not edit the local task-state JSON.
- Do not introduce abstractions for hypothetical future requirements.
- Preserve data integrity, compatibility, and accepted identifiers.
- A test must prove the behavior, not merely execute the code.

## Red → Green → Refactor

### 1. Inspect the exact slice

Trace the relevant symbol, current test, and nearest established pattern. Read only what is needed for the assigned task. Confirm that the requested outcome fits the accepted spec.

If repository evidence contradicts the spec or requires a product decision, stop and return the contradiction to the parent.

### 2. Red

Add or update the narrowest test that demonstrates the missing or incorrect behavior.

Run it before production implementation and confirm:

- it fails;
- it fails for the expected behavioral reason;
- the failure is not caused by a typo, broken fixture, unavailable dependency, or unrelated setup problem.

For a new Go function, method, or interface method, a compile error is not red. First add a stub with the final signature that returns zero values (or `errors.New("not implemented")`), then run the test and confirm it fails on the behavioral assertion.

A test that passes before implementation is not red evidence. Strengthen or correct it before continuing.

Acceptable exceptions require an explicit reason:

- documentation-only change;
- generated artifact whose source is tested elsewhere;
- mechanical configuration with no executable harness;
- migration where the repository lacks a focused migration-test facility.

For an exception, use the strongest available static validation, integration check, or reversible local probe. Never silently skip the red step.

### 3. Green

Implement the smallest safe change that makes the red test pass.

- Avoid unrelated cleanup.
- Avoid speculative configurability.
- Keep behavior server-authoritative where the existing architecture requires it.
- Enforce invariants at the layer that owns them.
- Handle errors using existing repository conventions.

Run the focused test and confirm it passes. If another test fails, determine whether it exposes a regression or an incorrect assumption; do not weaken tests merely to obtain green.

### 4. Refactor

Refactor only after focused tests are green.

Refactoring is optional. Perform it only when it removes duplication or clarifies the changed behavior without expanding scope.

After every refactor step:

- rerun the focused test;
- keep the diff small;
- preserve the task outcome;
- stop if the refactor starts changing adjacent architecture.

### 5. Verify the task

Run the task's declared verification, then the smallest relevant neighboring suite. Use `.agents/verification.md` for repository commands and environment constraints.

Check:

- accepted behavior;
- negative and edge behavior;
- compatibility with completed dependencies;
- migration or serialization consequences when applicable;
- frontend loading, empty, error, and retry states when applicable;
- no unintended generated or formatting churn.

Do not claim a check passed if it was skipped or unavailable.

## Subagent output contract

Return a compact result:

```text
status: done | blocked
task: <ID>
summary: <outcome implemented>
changed: <paths>
red: <command and expected failure>
green: <command and pass result>
verification: <additional commands and results>
concerns: <none or concrete concern>
```

The parent verifies the diff and evidence before updating task state.

## Stop conditions

Return `blocked` when:

- the spec is ambiguous or contradicted by repository evidence;
- the task requires changing an unapproved contract;
- another active task owns the same files or shared contract;
- the red test cannot be made meaningful without broader design work;
- required infrastructure or credentials are unavailable;
- completing the task would require unrelated changes.

Do not convert a scope or design blocker into an implementation assumption.

## Pitfalls

- Writing implementation first and backfilling a passing test.
- Treating compilation failure as proof of behavioral red when the intended test never ran.
- Mocking away the behavior under test.
- Changing assertions to match incorrect implementation.
- Refactoring adjacent code while the assigned behavior is still red.
- Running a broad suite repeatedly instead of using a tight focused loop.
- Reporting success from a subagent summary without real command evidence.
