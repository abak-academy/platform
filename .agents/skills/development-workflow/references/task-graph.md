# Task graph and execution choice

The task graph is local orchestration state. It is derived from the accepted spec and must not duplicate its prose.

## File

```text
.agents/work/<short-name>.json
```

Use the same concise semantic name as the spec. The file is gitignored and lives in the main checkout, not the worktree, so it survives worktree removal.

## Minimal schema

```json
{
  "spec": "docs/specs/composable-exam-format.md",
  "stage": "align",
  "execution": null,
  "worktree": ".worktrees/composable-exam-format",
  "pr": null,
  "tasks": {
    "T1": {
      "outcome": "Stable exam policy contract",
      "depends_on": [],
      "scope": ["backend schema", "backend model", "student API"],
      "done_when": "Focused contract and migration tests pass",
      "status": "pending"
    }
  }
}
```

Allowed stages:

```text
align | build | handoff
```

Allowed task statuses:

```text
pending | in_progress | blocked | done
```

Top-level fields are `spec`, `stage`, `execution`, `worktree`, `pr`, and `tasks`. The task ID is the object key. Each task has exactly five fields:

```text
outcome | depends_on | scope | done_when | status
```

Do not add fields unless real execution proves this contract insufficient. Executor assignment is derived from the human-confirmed execution strategy and does not need to be repeated per task.

## Decomposition rules

Plan around end-to-end outcomes rather than database/backend/frontend phases. A task should be:

- self-contained enough for one executor;
- independently verifiable where practical;
- scoped narrowly enough to avoid accidental overlap;
- explicit about what outcome becomes true;
- large enough to justify delegation overhead.

Keep human interaction, integration judgment, final verification, and tiny coupled changes with the parent even when subagents are allowed.

## Dependencies and parallel frontiers

`depends_on` is the source of logical readiness. A task is ready only when all dependencies are `done`.

Do not store a separate parallel boolean. Derive each parallel frontier from:

1. dependency-ready tasks; and
2. non-overlapping scopes that do not redefine the same shared contract.

No dependency does not prove parallel safety. Two ready tasks may still conflict through the same files, schema, generated types, API contract, migration order, or shared test fixtures. In that case, add the real dependency or execute them sequentially.

Prefer graph shapes that unlock independent work after a small contract task:

```text
T1 shared contract
├── T2 admin flow
├── T3 student flow
└── T4 backend runtime
        ↓
T5 integrated verification
```

Do not split work artificially merely to create more subagents.

## Execution confirmation

Before setting `stage` to `build`, show the human:

- tasks and dependencies;
- derived parallel frontiers;
- proposed parent/subagent assignments;
- maximum useful concurrency;
- available model choices;
- whether the runtime can actually override subagent models and effort;
- recommended effort by task group;
- wall-clock versus token-cost trade-offs.

Ask the human to confirm:

1. parallel subagents, sequential subagents, or parent-only execution;
2. whether subagents may be spawned;
3. model choice using only enforceable runtime options;
4. reasoning effort;
5. branch push and PR creation authorization.

Record only the confirmed choice:

```json
{
  "strategy": "parallel_subagents",
  "model": "inherit",
  "effort": "high",
  "max_concurrency": 3,
  "push_and_pr": true
}
```

If the runtime cannot override model or effort, use `inherit` and say so. Never claim a model choice was applied without a tool-visible enforcement mechanism.

Recommend the smallest safe strategy. Prefer parallel execution when a genuine independent frontier exists, while warning that cold isolated contexts can lower wall-clock time but increase total tokens.

## State ownership

The parent agent is the sole writer of task state.

Before dispatch:

1. re-check dependencies;
2. inspect current working-tree scope collisions;
3. mark selected tasks `in_progress`;
4. provide each executor the relevant spec section, exact scope, and verification command.

After dispatch:

1. inspect changed files and diff;
2. verify real command output;
3. record concerns or blockers;
4. mark `done` only when the task outcome is proven.

Subagents must not edit the JSON. If implementation crosses the declared scope, stop and re-plan rather than allowing concurrent agents to race.