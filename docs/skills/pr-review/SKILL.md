---
name: pr-review
description: Review a pull request in this repo and emit findings as JSON bucketed into blocker / changes_needed / nits / green. Use when asked to review a PR, audit a diff, or check a branch before merge. Read-only — never posts to GitHub without an explicit ask.
---

# PR review

Read-only. Emit JSON. Never post to GitHub unless the user says to.

## 1. Fetch without checking out

Concurrent sessions share this working tree. Do not check out the PR branch.

```bash
gh pr view <N> --json number,title,author,state,baseRefName,headRefName,headRefOid,additions,deletions,changedFiles,body,mergeable
gh pr diff <N> --name-only
gh pr diff <N> > /tmp/pr<N>.diff
gh pr checks <N>
```

Read the whole diff. Not a summary, not the file list.

`gh pr diff` gives the net diff; `--patch` shows per-commit churn. Files added then deleted appear only in `--patch` — check both before claiming a file was touched.

## 2. Verify every finding against the repo

A diff alone cannot tell you severity. Before writing a finding, go look:

| Claim | What to check |
|---|---|
| "this will exhaust the pool" | `backend/config/env/prod/config.yaml` → `db_max_conns` |
| "this poll is too frequent" | same file → `worker_poll_interval` |
| "this query has no index" | `grep "CREATE INDEX" backend/db/migrations/*.up.sql` |
| "this is a regression" | read the pre-image in the diff, or `git show <base>:<path>` |
| "this repo does X" | find the existing named type or helper; match it |

Say so when a finding is reasoning rather than measurement. Label it.

## 3. Bug classes this repo keeps producing

Check each one explicitly.

**Backend**

- **Nested pool acquisition.** A function inside an open `tx` calling a repo method that uses `r.pool`. With `db_max_conns: 10`, ten concurrent callers hold ten conns and all wait for an eleventh. Trace every call between `BeginTx` and `Commit`.
- **New predicate, no index.** A new polling query filtering on a column with no index. Migration `0063_exam_session_active_index` documents this exact class.
- **Poison row starves a batch.** `ORDER BY x LIMIT n` where a row that always errors stays at the head forever.
- **New sentinel, no `mapServiceError` case.** Else 500 + `slog.Error` on every call.
- **Guard from an unlocked pre-read.** A condition checked before the lock is not an invariant. Put it on the write.
- **Struct shape.** Functions returning more than ~4 unnamed values. Inline anonymous interfaces where the package already has a named one (`execer`, `querier`).

**Frontend**

- **Client storage schema change with no migration.** A new `localStorage` shape that silently drops the old one. Prod browsers outlive a deploy.
- **Side effects inside a state updater.** `queueX()` or `scheduleY()` inside `setState(prev => …)`. StrictMode double-invokes.
- **Scoped vs unscoped reads.** A gate that reads the whole queue when it should read the active section's slice.
- **Silent catch.** `catch { setState(false); return; }` with no user-visible error. The button re-enables and nothing happens.
- **Fail-closed regressions.** A change that turns graceful degradation into a hard block. Ask what happens in Safari private mode, offline, and at quota.
- **Write amplification.** Full-snapshot writes where a delta would do — especially on submit and section transitions, when everyone acts at once.

**Tests**

- Does the assertion fail if you invert the behaviour? If not, it is vacuous.
- Was a test file rewritten wholesale? Diff test names against the base — coverage silently drops.
- Was a behaviour flipped in a unit test but not its `e2e/*.spec.ts` twin?

## 4. Severity

| Bucket | Rule |
|---|---|
| `blocker` | Data loss, an outage under expected load, or a user who cannot complete the core action. Merge is gated. |
| `changes_needed` | Real defect or a missing migration/index. Fix in this PR, not a follow-up. |
| `nits` | Style, naming, structure, a small avoidable cost. Author's call. |
| `green` | Worth naming so it does not get refactored away. Include the reason it is good. |

A regression outranks a pre-existing flaw. If the base branch already had it, say so and drop it a bucket — or leave it out of scope.

## 5. Output

One JSON object. Short phrases, not sentences. Include `fix` on blockers and changes_needed.

```json
{
  "pr": 0,
  "repo": "abak-academy/platform",
  "title": "",
  "head": "",
  "base": "main",
  "files": 0,
  "additions": 0,
  "deletions": 0,
  "ci": "green | red | pending",
  "mergeable": true,
  "reviews": 0,
  "reviewed": "YYYY-MM-DD",
  "verdict": "approve | request_changes | comment",
  "note": "state what was reasoning rather than measurement",
  "blocker":         [{ "id": 1, "where": "path or symbol", "what": "", "fix": "", "regression": true }],
  "changes_needed":  [{ "id": 2, "where": "", "what": "", "fix": "" }],
  "nits":            [{ "id": 3, "where": "", "what": "" }],
  "green":           [{ "id": 4, "where": "", "what": "" }]
}
```

`id` is continuous across all four buckets so a finding can be cited as "#7" in follow-up.

## 6. Posting

Only when the user explicitly asks.

The PR author cannot submit `APPROVE` or `REQUEST_CHANGES` on their own PR — GitHub rejects it. For a self-authored PR, `COMMENT` is the only review event available. Say this rather than letting the call fail.
