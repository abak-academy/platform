# Handoff stage

Handoff turns verified implementation into a reviewable PR. It does not merge, close the issue, or deploy.

## Knowledge reflection

Classify the durable result:

```text
decision | architecture-update | domain-update | runbook-update | none
```

Create or update documentation only when system truth changed.

- `decision`: a meaningful choice among alternatives with lasting trade-offs.
- `architecture-update`: component boundaries, contracts, data ownership, or runtime behavior changed.
- `domain-update`: business rules, user process, or invariants changed.
- `runbook-update`: operating, migration, incident, or maintenance procedure changed.
- `none`: implementation restored or refactored existing documented behavior without changing durable truth.

If the result is `none`, record the reason. Do not create a decision page for routine implementation detail or use the wiki as a changelog.

Write the result into the repo wiki per [`docs/wiki/index.md`](../../../../docs/wiki/index.md):

| Classification | Location |
|---|---|
| `decision` | `docs/wiki/decisions/` |
| `architecture-update` | `docs/wiki/architecture/` |
| `domain-update` | `docs/wiki/domain/` |
| `runbook-update` | `docs/runbooks/` |

Update an existing page before creating a new one. Add every new page to `docs/wiki/index.md`. Include the changes in the same branch.

## Pre-PR gate

Before opening the PR:

1. Every accepted criterion has evidence.
2. Every task is `done`.
3. For an issue-related PR, the affected entry in `docs/backlog/open-issue-map.md` is added, updated, or removed with evidence in the same diff; no unrelated issue entries change. This also applies to the small-change path.
4. Focused and required broad checks pass.
5. Browser E2E or operational evidence passes when relevant.
6. Local review has no unresolved `blocker` or `changes_needed` finding.
7. `git diff --check` passes.
8. `git status --short` contains only scoped files and known pre-existing work.
9. The committed spec still matches the implementation; update it if an approved implementation discovery changed the contract.

Do not include `.agents/work/` in commits.

## PR body

Open the PR with:

- problem and agreed user outcome;
- spec path;
- meaningful acceptance changes from the original issue;
- implementation summary by user outcome;
- affected open-issue-map entries added/updated/removed, with evidence for any removal;
- verification commands and observed results;
- migration and rollback notes when relevant;
- knowledge reflection result;
- residual risks and deferred work.

Read the PR back and verify its base, branch, title, body, and URL.

## Final agent review

Run the repo-owned `pr-review` skill read-only against the opened PR.

- Fix `blocker` and `changes_needed` findings.
- Rerun affected verification.
- Push the fixes.
- Re-review until the verdict is `green` or contains only accepted `nits`, at most 5 review rounds.
- If round 5 still has a `blocker` or `changes_needed`, stop and hand the open findings to the human instead of continuing.

Do not post review comments unless the human explicitly asks.

## Stop boundary

Record the PR URL in the task-state `pr` field, then stop for human review.

Do not:

- merge the PR;
- close the GitHub issue;
- deploy;
- delete the local task state or worktree while review is active.

After the human accepts or merges the PR, a later continuation may verify the merge, remove the worktree and local task state, and close the issue when explicitly requested.