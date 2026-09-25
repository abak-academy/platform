# Align stage

The align stage produces a committed brief/spec that represents shared understanding, not an agent-authored guess.

## Round 0 — targeted evidence

Gather enough evidence to ask informed questions without loading the whole repository.

1. Pin branch, head SHA, dirty state, and the GitHub issue.
2. Read repository instructions and relevant context packs when not already loaded.
3. Read documents directly linked by the issue: backlog design, ADR, prototype, runbook, or prior PR.
4. Search for the current behavior before opening implementation files.
5. Trace only the relevant definitions, usages, migrations, tests, and user surfaces.
6. Record compact findings as:
   - verified fact;
   - human decision required;
   - implementation detail that can wait for its task.

### Token discipline

- Search first and read only shortlisted files or relevant sections.
- Do not run broad scans when a symbol, route, migration, or test name can narrow the surface.
- Do not deep-read every layer during alignment. Defer task-specific internals until build.
- Do not copy full issues, documents, diffs, or command output into the spec.
- Capture evidence once and reuse the summary rather than reopening unchanged sources.
- Prefer one parent-agent evidence pass over several cold discovery subagents.
- Delegate discovery only after human approval and only for genuinely independent surfaces.
- Label unresolved evidence instead of filling gaps with assumptions.

Round 0 is complete when the agent can explain current behavior, the affected user journey, likely system surfaces, and which choices require human intent.

## Grilling rounds

Create a decision tree covering all material branches, not only implementation blockers:

```text
Problem and outcome
├── affected users
├── current pain
├── successful outcome
└── explicit non-goals

Product behavior
├── primary user flow
├── edge and failure behavior
├── compatibility
└── migration or rollout

Technical solution
├── domain ownership
├── API and persistence contracts
├── state and concurrency
├── security and data integrity
└── observability and rollback

Proof
├── acceptance criteria
├── automated tests
├── browser or operational evidence
└── residual risk
```

Ask the currently answerable frontier in each round. Batch independent questions and postpone questions whose prerequisites remain undecided.

For each question:

- Explain why the decision matters.
- Present up to three viable options when meaningful.
- Put the preferred recommendation first and explain its trade-off.
- More than one option may be recommended for different priorities; state which priority each optimizes.
- If evidence does not justify a preference, say the choices are neutral.
- Investigate facts from the repository; ask the human for intent, priorities, acceptable trade-offs, and product decisions.

Do not stop because coding is merely unblocked. Continue until the human and agent agree on user behavior, technical boundaries, failure handling, compatibility, proof, and non-goals.

After every round:

1. Update agreed decisions and remaining questions.
2. Rewrite affected acceptance criteria.
3. Record additions, removals, refinements, and deferrals from the GitHub issue.
4. Expand the decision tree with follow-up questions created by the answers.

## Alignment check

When no material branch remains unresolved, present:

```text
Shared understanding
- Problem:
- User outcome:
- Technical approach:
- Important invariants:
- Acceptance changes:
- Deferred scope:
- Remaining risks:

Proceed with this implementation contract?
```

Do not write production code until the human explicitly confirms it. Do not silently rewrite the GitHub issue; synchronization is a separate external write requiring approval.

## Brief/spec contract

After approval, choose a concise semantic name and write:

```text
docs/specs/<short-name>.md
```

Use this structure:

```markdown
---
title: <concise title>
issue: <number or URL>
status: accepted
created: YYYY-MM-DD
sources: [<links or repo paths>]
---

# <Title>

## Problem and outcome
Current pain, affected users, and the observable outcome.

## Agreed behavior
Primary flow, edge/failure behavior, and compatibility or migration rules.

## Technical contract
Only durable boundaries, invariants, ownership, and meaningful decisions.

## Acceptance and proof
Agreed criteria, changes from the issue, and how each outcome will be verified.

## Scope
Non-goals, deferred decisions, and residual risks.
```

Keep these five sections. Use short subsections only when the feature genuinely needs them.

The spec should be dense and durable:

- Write agreed truths, not the transcript of the grilling session.
- Link evidence instead of copying large source material.
- Make acceptance criteria observable and independently checkable.
- Preserve why a meaningful issue criterion changed.
- Omit implementation detail unless future work would make a wrong decision without it.
- Prefer tables and short bullets over narrative history.

Create the worktree and feature branch, then commit the accepted spec before production implementation. This gives later agents and reviewers a stable contract in Git history while keeping the execution task state local.