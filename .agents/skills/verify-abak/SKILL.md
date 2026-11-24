---
name: verify-abak
description: Drive the Abak Academy Hub (Next.js web + Go API) the way a student or admin does, on a disposable stack built from the current checkout, and capture proof (screenshots, ARIA snapshots, API reads). Use to prove a web/API change works end to end, not for unit or CI gates (those are in .agents/verification.md).
---

# verify-abak

One command brings up a **throwaway** copy of the platform from this checkout's source. It has its own
compose project (`abak-verify`), its own ports and an empty database, so it never touches the dev
stack on :3000/:8080 or prod. Playwright then drives the real UI in system Chrome.

Helper: `.agents/skills/verify-abak/bin/abak-verify` (run from `platform/`; examples write it as `$V`).

```bash
V=.agents/skills/verify-abak/bin/abak-verify
```

| Surface | URL |
|---|---|
| Web (student + admin) | `http://localhost:53000` |
| API | `http://localhost:58080/api/v1` |
| Postgres / Redis / MinIO / Gotenberg | `:55432` / `:56379` / `:59000` / `:53001` |

## Launch

```bash
$V up        # ~20s warm, ~2 min on a cold Go module cache
```

What it does, in order (each step fails loudly with the log path):

1. Refuses if `~/.cache/abak-verify/state` exists (a live or uncleaned run) or any port above is taken.
2. Copies `backend/config/env/dev/` (gitignored; in a fresh worktree copy it from the main checkout
   first) to the state dir, rewriting compose hostnames to the verify ports and adding `:53000` to
   `cors_origins`.
3. `docker compose -p abak-verify ... up -d --wait` postgres, redis, minio, gotenberg, then
   creates the buckets. Overlay: `compose.verify.yml`.
4. `go build ./cmd/api` from source (with `env -u GOROOT`, because the GOROOT in
   `.agents/verification.md` points at a Cellar version that no longer exists), then starts it.
   Migrations run at boot. Ready = `/health` reports `"status":"ok"`.
5. Seeds two accounts with random passwords (`verify-admin@example.test` as super_admin,
   `verify-student@example.test` as student) and proves the admin can log in over the API.
   Credentials: `~/.cache/abak-verify/state/creds.env`. Never paste them into chat or a PR.
6. rsyncs `web/` into the state dir (symlinked `node_modules`) and runs `next dev -p 53000`
   there, so the user's own `web/.next` is never touched. Ready = `/login` returns 200, then it
   pre-compiles `/`, `/admin`, `/catalog`, `/exam` and `/admin/exam`.

It does **not** start the outbox worker (`cmd/worker`), so notifications and other async side
effects never fire. Start it yourself if a journey depends on them.

**Testing a code change:** the stack runs the code as it was at `up`. After editing, `$V down && $V up`.
`doctor` prints the launched revision next to the current HEAD. If they differ, you are driving stale code.

## Doctor

```bash
$V doctor    # read-only; exit 0 = worth driving
```

It checks the state dir, that the api and web pids are alive, that :58080 is held by our api pid,
that `/health` is ok, that `/login` returns 200, and that the infra containers are up. Run it first
whenever anything looks off.

## Drive

Journeys are Playwright specs in `journeys/`, run against the live stack (no `webServer`; system
Chrome via `channel: "chrome"`, so no browser download is needed):

```bash
$V journey login                    # all tests in journeys/login.spec.ts
$V journey login -g login-admin     # extra args go to playwright test
```

The env each journey gets: `VERIFY_WEB_URL`, `VERIFY_API_URL`, `VERIFY_EVIDENCE_DIR`,
`VERIFY_REVISION`, and the `VERIFY_*_EMAIL/PASSWORD` values from `creds.env`.

Writing a new journey: copy `journeys/login.spec.ts`. Keep its `beforeEach`, which records
console, page errors, failed requests and every API response. Select by role and accessible name. UI
copy is Indonesian by default with English as the alternative, so match both:
`/^(Masuk|Sign in)$/`. `data-testid` handles exist on the admin toolbars (`exam-tests-toolbar`,
`tests-search-input`). For data a journey needs (products, tests, exams), create it through the admin UI
or the admin API with the seeded admin's token. There is no seed script, and `POST /admin/products`
creates a draft that the catalog hides until it is published.

Manual poking: the built-in browser can open `http://localhost:53000` directly. Log in with
`creds.env` values; the dev host is local.

## Evidence

Every `journey` run writes to `~/.cache/abak-verify/evidence/<timestamp>-<journey>/`:
`playwright.log`, `results.json`, per-test `*.before.png`, `*.after.png`, `*.aria.txt`,
`*.browser.log`, the API second-view `*.me.json`, and `test-output/` (trace.zip for every test).
These live outside the repo and survive `down`.

Proof standards:
- Drive the real user path (the login form, the button). Never use internal setters, seeded
  localStorage or test-only endpoints as the proof step.
- Capture the action and the resulting state (before and after), not only the final screen.
- Confirm side effects through a second view: re-read through the API (`/auth/me`, the admin
  list endpoint) or `docker compose -p abak-verify exec postgres psql -U akademi -d akademi`.
- Mocks only where production already isolates a vendor (Midtrans, Biteship, Fazpass, SMTP).
  These have sandbox or empty keys in dev config, so a payment or OTP journey proves the request
  shape, not delivery. Say so in the report.
- Report the revision from `doctor` with the evidence path.

## Cleanup

```bash
$V down
```

This stops only the api and web pids it recorded, plus their children, then runs `docker compose -p abak-verify
down -v` (containers and volumes) and deletes the state dir. Evidence is kept. It warns if a verify
port is still listening. Never kill `next` or `api` processes by name: the user's own dev servers
use the same binaries. Run `down` after every failed `up` too, because a half-started run leaves
the state dir behind, and the next `up` refuses to start.

## Isolation limits

- One run at a time per machine: fixed ports and one state dir. A second `up` refuses rather than share.
- The dev stack (`akademi-bimbel` compose project) and this one can run side by side.

## Feature map

`features/README.md` indexes the user-facing features and how to drive each one. When a
feature lists several entry points, a proof through one of them is partial.
