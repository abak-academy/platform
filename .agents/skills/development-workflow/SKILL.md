---
name: development-workflow
description: Run one issue from shared understanding to a reviewed PR.
---

# Development workflow

Use this skill for a feature, bugfix, refactor, or GitHub issue that changes the repository. It aligns the human and agent, persists the agreed specification, coordinates a dependency-aware task graph, opens a verified PR, and then stops for human review.

The GitHub issue is initial intent, not the final implementation contract.

## Size gate

Use the full workflow only when at least one is true:

- more than one valid product behavior or technical approach exists;
- the change crosses layers (migration, API contract, frontend + backend);
- it touches money, auth, exam scoring, or student data;
- it needs more than one task or any subagent.

Otherwise it is a small change: skip `align`, the spec, and the task JSON. State the goal and verification, create the worktree (see Build step 0), follow [TDD implementation](../tdd-implementation/SKILL.md), and open the PR only when the human asks. If a small change grows past the gate, stop and start `align`.

## Living issue map — both paths

For **every PR that works on a GitHub issue**, read the issue and comments plus [`docs/backlog/open-issue-map.md`](../../../docs/backlog/open-issue-map.md) before editing. If its entry is missing, follow manual intake in [`.agents/workflows.md`](../../workflows.md): add an `unassessed` entry to the issue PR, then assess the affected code before handoff. Revalidate the affected entry when issue intent or relevant code has changed since its pinned timestamps/commit. In the **same branch and PR**, add or update that issue's entry with observed branch behavior, code/test evidence, remaining work, issue `updatedAt`, and checked commit. If all agreed criteria are proven, remove the entry in that PR and explain its removal and evidence in the PR body. Do not change unrelated entries. This applies to the small-change path as well as the full workflow; an issue-related PR is not ready for review without its map change. A branch/PR result is not evidence of merge, issue closure, or production rollout.

## Artifact contract

Use two artifacts with different lifecycles and locations.

### Committed brief/spec

```text
docs/specs/<short-semantic-name>.md
```

This is the durable shared understanding: problem, user outcome, current behavior, agreed decisions, acceptance criteria, non-goals, technical contract, and verification strategy. Commit it after alignment and keep it as context for future development.

Choose the name only after shared understanding. Use a short domain/outcome phrase, normally two to five words. Do not copy the full GitHub issue title or use only its number.

Example:

```text
docs/specs/composable-exam-format.md
```

### Local task state

```text
.agents/work/<same-short-name>.json
```

This is the local orchestration state: current stage, confirmed execution choice, dependency graph, scopes, completion checks, and task statuses. It is gitignored and must never enter the PR.

Keep it while the PR is under human review. Remove it only after merge, abandonment, or an explicit human request.

Read [`references/align.md`](references/align.md) for the spec contract and [`references/task-graph.md`](references/task-graph.md) for the JSON contract.

## Workflow

The workflow has three stages:

```text
align → build → handoff
```

`blocked` is a task condition, not another stage.

### 1. Align

Read and follow:

- [`references/align.md`](references/align.md)
- [`references/task-graph.md`](references/task-graph.md)

Required outcome:

1. Gather targeted evidence without scanning the whole repository.
2. Grill product behavior, user flow, technical design, failure handling, compatibility, and proof.
3. Rewrite acceptance criteria when shared understanding changes them.
4. Obtain explicit human approval of the brief/spec.
5. Choose the short semantic name, create the worktree (Build step 0), and write `docs/specs/<name>.md` inside it.
6. Build `.agents/work/<name>.json` in the main checkout from the approved spec.
7. Present task dependencies, derived parallel frontiers, model/effort options, token-cost trade-offs, and proposed subagents.
8. Obtain explicit human approval of execution strategy, model, effort, subagent use, push, and PR creation.
9. Commit the approved spec on the feature branch before implementation code.
10. Set JSON `stage` to `build`.

Do not change production code before both approvals are recorded.

### 2. Build

Read and follow [TDD implementation](../tdd-implementation/SKILL.md) for every task that changes behavior or production code.

0. Work in an isolated worktree, never by switching branches in the shared checkout; other sessions use it concurrently:

   ```bash
   git worktree add .worktrees/<name> -b <type>/<name> origin/main
   ```

   Symlink `node_modules` into the worktree only after `diff -q` shows identical lockfiles. Task state stays in the main checkout at `.agents/work/<name>.json` so it survives worktree removal.

The parent agent owns orchestration:

1. Select tasks whose dependencies are complete.
2. Derive the largest safe parallel frontier from dependency readiness and non-overlapping scopes.
3. Dispatch only according to the human-confirmed strategy, model, effort, and concurrency.
4. Give each executor the approved spec section, exact task, scope, and verification requirement.
5. Inspect the resulting diff and real command output before marking a task `done`.
6. Stop and return to alignment if implementation evidence invalidates an agreed decision or changes scope.
7. After all tasks, run integration verification, browser E2E when relevant, and a complete local review.
8. Fix all `blocker` and `changes_needed` findings and rerun affected checks.
9. Set JSON `stage` to `handoff` only when every acceptance criterion has evidence.

Subagents must not edit the task-state JSON or expand their assigned scope. The parent is the sole task-state writer.

### 3. Handoff

Read and follow [`references/handoff.md`](references/handoff.md).

Required outcome:

1. Complete knowledge reflection and include durable documentation changes in the branch.
2. Run repository hygiene and required verification.
3. Commit and push only scoped files; exclude `.agents/work/`.
4. Open and read back the PR.
5. Run the repo-owned `pr-review` skill read-only against the opened PR.
6. Fix and push `blocker` or `changes_needed` findings until the review is `green` or accepted `nits`.
7. Record the PR URL in local task state.
8. Stop for human review.

Do not merge the PR, close the issue, deploy, or remove task state during this workflow.

## Completion

Final response:

```text
PR: <url>
Spec: docs/specs/<name>.md
Branch: <branch>

Verification:
- <gate>: passed

Review: green | nits
Knowledge reflection: <classification and paths>
Residual risk: <none or concise list>

Not performed:
- PR merge
- issue closure
- deployment

Waiting for human review.
```

## Pitfalls

- Treating the issue body as an immutable specification.
- Asking only blocker questions instead of aligning the whole problem and solution.
- Loading the whole repository during evidence gathering.
- Storing durable shared understanding only in ignored local state.
- Duplicating full spec prose inside the task JSON.
- Assuming dependency-free tasks are parallel-safe while their scopes collide.
- Spawning subagents before human cost and execution confirmation.
- Writing production code without a demonstrated red test when the behavior is testable.
- Trusting subagent summaries without inspecting their changes and evidence.
- Merging or closing the issue before human PR review.
- Running the full workflow for a change below the size gate.
- Switching branches in the shared checkout instead of using a worktree.