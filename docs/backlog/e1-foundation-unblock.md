# E1 — Foundation & unblock

| | |
|---|---|
| **Issue** | [#56](https://github.com/abak-academy/platform/issues/56) |
| **Objective** | Remove the four things that block other epics. Nothing here is client-visible. |
| **Source IDs** | F-6, B-8, D-1 + one verification-debt item |
| **Client items** | none — this is enabling work |
| **Blocks** | E3, E4, E6, E7 (hard) · E2 (quality gate) — **not** E5, which is free to start |
| **Depends on** | — |
| **Verified against** | `main` @ `211b7b1`, 2026-07-29 |

> **This epic is not a vertical slice.** It has no UI and, apart from one column-free sanitiser, no
> schema. It exists because four separate pieces of later work each stall on something here. Grouping
> them buys one round of setup instead of four.

---

## 1. F-6 — Amazon SES

**The Hostinger mailbox caps at 100 emails/day, and OTP goes over SMTP.** A 5000-participant event
does not merely fail to deliver exam cards — **registration itself dies at email 101**. Every student
who signs up after the cap gets no OTP and cannot complete registration.

SES needs domain verification plus DKIM records in Cloudflare, so it carries DNS propagation lead
time that cannot be compressed on the day of an event. This is why it leads the epic rather than
sitting in E7 where the load test lives.

Swap happens behind the existing notification port — the provider is already pluggable. In fact **no
code changes at all**: `adapter/smtp.go` is plain SMTP (`smtp.PlainAuth` + `smtp.SendMail`) and SES
exposes an SMTP endpoint, so this is a configuration swap.

**Step-by-step runbook, gotchas and rollback:
[`ses-email-migration.md`](ses-email-migration.md).** Two things from it worth knowing up front: the
sandbox-exit review is the only step whose timing cannot be compressed, and `config.go` has no env-var
override, so the cutover is a rebuild-and-redeploy of api *and* worker rather than a restart.

**Acceptance**
- A test send through SES arrives at an external address.
- The daily cap is demonstrably no longer 100 (send 150 in a scripted run, or show the SES quota).
- OTP registration succeeds through the new provider end to end.

---

## 2. B-8 — CSV formula injection, three writers

No sanitiser exists anywhere in `backend/`. Three writers emit attacker-supplied student names
straight into CSV:

| Writer | File |
|---|---|
| Exam results export | [`admin_results.go:135`](../../backend/internal/service/admin_results.go) |
| Bulk credentials | [`bulk_credentials.go:96`](../../backend/internal/service/bulk_credentials.go) |
| Student bulk import report | [`student_bulk.go:338`](../../backend/internal/service/student_bulk.go) |

Student names are attacker-supplied at registration. The frontend roster export was fixed long ago
(C-08); the backend never was.

**Why it gates two epics.** `admin_results.go` is the writer behind the results tab **E3** builds a UI
for — shipping the tab first puts a nicer front door on the hole. And **E4**'s school importer is
explicitly "copy the student bulk pattern", which means copying `student_bulk.go` verbatim,
vulnerability included.

**Fix** — mirror C-08: prefix `'` and force-quote any field starting with `= + - @ \t \r`.

**Acceptance**
- A student named `=cmd|' /C calc'!A0` exports as inert text from all three writers.
- One test per writer, each mutation-checked (breaks when the sanitiser is removed).

---

## 3. D-1 — Storage seam, then delete the shims

`internal/service/ports_storage.go` defines a `StorageClient` port that nothing uses. `Service.storage`
is still a concrete `*minio.Client` ([`service.go:28`](../../backend/internal/service/service.go)), and
`storeRepo` is concrete too. With no seam, service tests hand-copy production logic into shims and
assert against the copy:

| File | shim methods |
|---|---|
| `course_test.go` | 18 |
| `store_test.go` | 14 |
| `exam_session_test.go` | 10 |
| `certificate_test.go` | 5 |
| `exam_result_test.go` | 4 |
| `exam_test.go` | 4 |
| `admin_results_test.go`, `exam_leaderboard_test.go` | 3 each |
| `student_test.go` | 1 |
| **total** | **62** |

**26 of those 62 sit in the exact files E2 and E3 touch.** Build the regrade engine on top and its
tests pass against hand-copied logic, not shipping code. Regrade decides scores; a tautological test
there is worse than no test, because it reads as coverage.

**Honest framing:** this is a *quality* gate, not a compile-time one. E2's regrade can physically be
built without the seam. What you lose is any reason to trust its tests.

**Two PRs, in order:** wire the seam, then delete the shims. Do not graft either onto a feature
branch — mixing a mechanical refactor with behavioural change makes both unreviewable.

**Acceptance**
- Shim count is zero.
- Full backend suite green.
- At least one previously shimmed test is mutation-checked to prove it now fails when production code
  breaks.

---

## 4. Prove the Gotenberg sidecar renders

The sidecar is up on staging and health-checks report chromium `up`, but **it has never actually
rendered a document end to end**. Cached certificates masked its total absence for weeks and the api
logs stayed silent throughout.

**E6** renders the shipping label through it. Building on infrastructure with zero proof is how that
epic discovers the problem late.

Not a byte assertion — an actual PDF, opened and looked at. This project has already shipped a
certificate that rendered fully upside-down while byte-level tests stayed green.

**Acceptance**
- A certificate or exam card produced by the staging sidecar, attached to the session notes, visually
  checked.

---

## Out of scope

- Anything client-visible. If a change here shows up in the UI, it belongs to another epic.
- D-5 (gofmt gate) — tempting while touching Go, but 37 files must be reformatted in one commit first
  or the gate lands red. Separate change.
