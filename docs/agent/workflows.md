# Agent Workflows

## Broad Scan

Goal: produce a read-only verdict with concrete findings, verification status, and residual risk.

Steps:

1. Pin git state: branch, head SHA, and dirty status.
2. Map relevant backend, frontend, deploy, and migration surfaces.
3. Run mechanical gates when feasible.
4. Inspect high-risk paths by behavior, not only by filenames.
5. Report findings ordered by severity with file and line evidence.

Do not edit files during a scan.

## Feature Or Bugfix

Goal: make the smallest code change that proves the requested behavior.

Steps:

1. Restate assumptions and non-goals.
2. Identify the affected context pack.
3. Add or update the narrowest test that fails for the current behavior.
4. Implement the smallest passing change.
5. Run focused verification, then broaden only when risk warrants it.

Stop before implementation if the behavior can be interpreted in more than one product-valid way.

## Refactor Or Cleanup

Goal: improve structure without changing behavior.

Steps:

1. Name the exact structural smell being cleaned.
2. Name the behavior that must remain unchanged.
3. Run existing tests first if the area is risky.
4. Move code in small steps.
5. Run the same tests after the change.

No opportunistic cleanup. Flag nearby debt separately.

## Review

Goal: find correctness, security, regression, and missing-test risks in the requested scope.

Steps:

1. Pin exact head and compare against base.
2. Inspect changed production paths and tests.
3. Validate claims with local checks where useful.
4. Report only actionable findings with evidence.

Use compact buckets: `blocker`, `changes_needed`, `nits`, `green`.
