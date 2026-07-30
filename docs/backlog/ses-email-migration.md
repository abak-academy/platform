# Backlog: move transactional email to Amazon SES

**Raised:** 2026-07-30 · **Status:** required before any exam event · **Epic:** [E1](e1-foundation-unblock.md) (F-6)

Detail doc for F-6. No GitHub issue of its own — it is scope inside E1 (#56).

---

## Why

The Hostinger mailbox caps at **100 emails/day** and OTP rides the same SMTP channel
(`newNotifyProviders` in `cmd/api/main.go:94` — SMTP wins whenever `smtp_host` is set, and prod sets
it). Three things break at the cap, in order of severity:

1. **Registration dies at email 101.** No OTP, no account. Not "exam cards are late" — people cannot
   sign up at all.
2. **Announcement broadcast is unbounded.** `sendAnnouncement` (`internal/service/announcement.go:206`)
   loops `ListActiveUserEmails` and sends **one email per address**, no batching, no rate limit,
   best-effort per address. One announcement to 5,000 students is 5,000 sends in a single loop.
3. Password reset (`admin_users.go:295`, `auth.go:304`).

A 5,000-participant event is 50× the daily cap before anyone opens an exam.

**Cost is not the issue.** SES is roughly **$0.10 per 1,000 emails** — a 5,000-person event is about
**$0.50**. What cannot be compressed is the AWS sandbox review and DNS propagation, which is why this
is scheduled rather than done on the day.

---

## What changes in the code

**Nothing.** `adapter/smtp.go` is plain SMTP — `smtp.PlainAuth` + `smtp.SendMail` (`:45`, `:64`) — and
SES exposes an SMTP endpoint. This is a configuration swap, not an integration.

---

## Step by step

### Phase 0 — decide before touching anything

- [ ] **Which AWS account?** Who owns it, who pays, who has console access. If the client owns it, the
      DKIM records still have to be published in *our* Cloudflare zone, so both sides must be available
      on the same day.
- [ ] **Region.** `ap-southeast-3` (Jakarta) or `ap-southeast-1` (Singapore). Either is fine; it only
      changes the SMTP hostname. Pick one and write it here.
- [ ] **Where do the current SMTP credentials come from?** `config/env/prod/config.yaml` carries only
      `smtp_host`, `smtp_port`, `smtp_from`, `smtp_from_name` — **not** username/password. Find how
      those are injected today before swapping, because the same channel has to carry the SES ones.

### Phase 1 — SES identity + DKIM *(free, no disruption, can be done today)*

- [ ] Create a **domain identity** for `abakacademy.id` in SES.
- [ ] Enable **Easy DKIM**. SES emits **3 CNAME records**. This single step both verifies the domain
      and turns on DKIM signing — they are not two separate jobs.
- [ ] Copy the 3 records out; they are needed in Phase 2.

Hostinger keeps working untouched throughout this phase.

### Phase 2 — Cloudflare DNS *(zone `abakacademy.id`)*

- [ ] Add the 3 CNAMEs: `<token>._domainkey.abakacademy.id` → `<token>.dkim.amazonses.com`.
      **Set them to DNS only — grey cloud, not orange.** A proxied record will not verify.
- [ ] Add **DMARC**: a TXT record at `_dmarc.abakacademy.id`. Gmail and Yahoo require DMARC for bulk
      senders, and 5,000/day is squarely in that bracket. Start at `p=none` to observe, tighten later.
- [ ] *Recommended:* a **custom MAIL FROM** subdomain (e.g. `mail.abakacademy.id`) with its MX and SPF
      TXT records, so SPF aligns for DMARC rather than relying on DKIM alone.
- [ ] Wait for SES to flip the identity to **Verified**. Cloudflare usually propagates in minutes; SES
      polls on its own.

### Phase 3 — leave the sandbox *(the queue you cannot compress)*

- [ ] File the **production access** request. A new SES account is sandboxed: sends only to verified
      addresses, ~200/day, 1 message/second.
- [ ] The form asks for the use case and how bounces are handled — answer it properly, a thin answer
      invites a round trip.
- [ ] Request a **sending quota** that covers the event. Ask for the **per-second rate** too, not just
      the daily total: 5,000 emails released in one burst is a rate problem before it is a volume one.
- [ ] Typical turnaround is ~24 hours, **but it is not guaranteed** and AWS may come back with
      questions. **Do this immediately after Phase 2, not the week of the event.**

### Phase 4 — cut over

- [ ] Generate **SES SMTP credentials** (a dedicated username/password pair, distinct from any AWS
      access key).
- [ ] Point config at SES:

      smtp_host: "email-smtp.<region>.amazonaws.com"
      smtp_port: "587"
      smtp_from: "noreply@abakacademy.id"     # unchanged — the domain is what is verified

- [ ] **Rebuild and redeploy api *and* worker.** `config.go` has **no env-var override** — the YAML in
      the repo is the only source, so it is baked into the image. This is a deploy, not a restart, and
      with F-5 still open it is a manual `IMAGE_TAG` edit on both VMs.

### Phase 5 — verify

- [ ] A registration OTP arrives at an external mailbox (Gmail, not a company address).
- [ ] Check the received message's headers show **DKIM pass** and **DMARC pass**.
- [ ] Send a small announcement to a handful of real addresses before any mass send.
- [ ] Confirm the SES console reports the expected send count.

---

## Bounce handling — the part that is easy to skip

SES suspends accounts whose bounce or complaint rate goes too high. Five thousand school-supplied
addresses will contain typos and dead mailboxes.

**Our code cannot currently see any of it.** Bounces arrive asynchronously, not as SMTP errors, so
`sendAnnouncement` logging a failure and continuing tells us nothing — the sends it counts as
successful include every address that will bounce later.

Minimum before the first mass send: watch the bounce rate in the SES console. Better: an SNS topic for
bounces and complaints. Not required to cut over, but required before a 5,000-address blast.

---

## Rollback

Reverting `smtp_host` to `smtp.hostinger.com` and redeploying restores the old path — the DNS records
are additive and harmless if SES is unused. Rollback is a rebuild, same as the cutover.

---

## Open

- Region not chosen.
- AWS account ownership not settled.
- Source of the current SMTP username/password not identified.
- No bounce/complaint handling anywhere in the codebase.
