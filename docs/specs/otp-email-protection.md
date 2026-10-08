---
title: OTP & email send protection (pending-key, layered limits, HMAC)
issue: none
status: accepted
created: 2026-10-08
sources:
  - konsep-otp.md (repo root, concept doc by the owner)
  - backend/internal/service/auth.go
  - backend/internal/adapter/smtp.go
  - docs/standards/go-patterns.md
---

# OTP & email send protection

## Problem and outcome

The account-verification OTP flow stores codes plaintext in Redis, overwrites the
live code before the email send, burns the resend ladder before the send, and
locks the user's channel for 12 hours after 3 provider failures — failures of
the relay punish the user. There are no per-hour/per-day/per-IP send quotas, no
provider-global quota, and no verification-failure budget that survives resend.
Forgot-password is unthrottled (relay burn + multiple live reset tokens).

Outcome: OTP discipline follows `konsep-otp.md` — only successful sends count,
failed sends never destroy the previous code, layered limits bound abuse per
email/IP/day, verification guessing is bounded across resends, and the code at
rest is a hash, not plaintext.

## Agreed behavior

Send (register/login/resend via `OTPProvider`):

- Pending key `otp:new:<uid>` holds the new code hash; on provider success it is
  promoted to `otp:<uid>` (full TTL) and the old code is overwritten. On
  provider failure the pending key is discarded and the previous code stays
  valid.
- Only successful sends consume the ladder, hour/day/IP quotas and level. The
  atomic claim runs first (check all, then increment all); on provider failure
  the quota increments are rolled back (DECR), the level is not raised, a
  fixed short pause (30s) replaces the cooldown, and the code hash is kept in
  the pending key path only.
- The 12-hour channel lockout (`otp:lock`) is removed. Replaced by
  `otp:sendfail:<uid>:<ch>` (limit N per 1h window); exceeding it rejects
  sends with 429 until the window expires.
- Cooldown ladder: 60s, 2m, 5m, 15m, 1h (free tier starts at 60s). Cap at 1h.
- Rejected requests do not consume quotas; they count in `otp:abuse:<uid>` /
  `otp:abuse:ip:<ip>` (1h, signal only).
- A still-live code (pending account) is reused: re-register/login does not
  re-send and reports the code as active (existing behavior, kept).
- Redis errors during the live-code check fail closed (return error, do not
  send).

Verify:

- Attempt budget increments BEFORE comparison, atomically (Lua), and the code
  is deleted when the budget is exhausted (behavior kept, ordering fixed).
- New counters `otp:vfail:<uid>` (1h), `otp:vfail:day:<uid>` (24h),
  `otp:vfail:ip:<ip>` (1h) gate verification and are NOT reset by resend.
- Successful verification deletes the OTP keys, level, cooldown, sendfail and
  vfail counters.

Forgot/reset password (option B — token flow and API contract unchanged):

- `ForgotPassword`, `ResetPassword`, `TriggerAccountPasswordReset` gain the
  same layered limits (cooldown ladder, hour/day quotas per target email,
  global SMTP quota, sendfail handling, vfail/attempts budgets on reset
  verify). Reset attempts budget is stored inside the `reset:<token>` value
  (key format unchanged). Token and code continue to be emailed together.
- All public send paths share one global provider quota counter
  (`smtp:quota:<yyyymmdd>`, EXPIREAT midnight WIB). Announcement fan-out is
  explicitly out of scope and not counted.

API contract:

- Unchanged endpoints and bodies. Error mapping changes: `otp_send_locked` is
  removed (error no longer exists); `email_delivery_failed` (503) gains a
  short `Retry-After`. Codes remain 422 for invalid/expired per existing
  mapping.

## Technical contract

- Redis key names exist today and are kept: `otp:<uid>`, `pending:<token>`,
  `otp:cooldown:<uid>`, `otp:count:<uid>`, `otp:verify:<uid>`. New sibling
  keys follow the existing style: `otp:new:<uid>`, `otp:hr:<uid>`,
  `otp:day:<uid>`, `otp:ip:<ip>`, `otp:sendfail:<uid>:<ch>`, `otp:vfail*`,
  `otp:abuse*`, `smtp:quota:<yyyymmdd>`; reset-purpose counters get a
  `:reset:` segment (`otp:cooldown:reset:<email>` etc.). `otp:lock` key is
  removed.
- The OTP is stored as hex `HMAC-SHA256(purpose+":"+code, OTPSecret)` inside
  `otp:<uid>`; comparison is constant-time. Purpose binding lives in the
  HMAC input, not the key name, so `otp:<uid>` format is unchanged. `reset:`
  values store `UID:codeHash:attempts` (key unchanged).
- Quota/ladder values are configurable: `otp_attempts` (3),
  `otp_quota_hour` (3), `otp_quota_day` (8), `otp_quota_ip_hour`,
  `otp_sendfail_max` (3), `otp_sendfail_pause` (30s), `smtp_quota_day` (500).
  Zero config falls back to these defaults in the service.
- Client IP reaches the service (`SendOTP`, `VerifyOTP`, `ForgotPassword`,
  `ResetPassword`) as `c.RealIP()`. nginx front door overwrites `X-Real-IP`
  from `$remote_addr` (production resolves the true client IP from
  Cloudflare via the real_ip module), so the value is trusted at the plumbing
  layer; direct-API spoofing is compose-internal only.
- `OTPSecret` empty is a configuration error: reject at boot.
- Observability: counters for send outcome, verify outcome, and per-layer
  rejections; no OTP or hash values in logs/metric labels.

## Acceptance and proof

1. Resend after a failed provider send: the previously delivered code still
   verifies; the ladder/quotas were not consumed.
2. Resend after a successful send: a new code is generated, TTL is full, the
   old code is rejected; live-code reuse still suppresses duplicate sends.
3. Limits: hour/day/IP quotas reject with `Retry-After` = the largest active
   TTL; rejected requests do not consume quota; fixed windows do not refresh
   on repeated requests.
4. sendfail: 3 consecutive delivery failures stop sending for the remaining
   1h window instead of locking the channel for 12h.
5. Verify: parallel wrong guesses are counted before comparison (no burst
   over the attempt budget); vfail hour/day counters survive resend; per-code
   budget still resets on a new code.
6. Plaintext code is never present in Redis (`otp:<uid>` holds only the HMAC
   hex); integration tests read the code by computing the HMAC.
7. Forgot-password respects the per-email ladder and quotas; reset verify is
   bounded by its attempts budget and vfail.
8. Whole-suite gates: `./deploy/pipeline/backend.sh` (build, vet, race tests).
   Integration tests update `integration/auth_test.go` Redis reads to HMAC.

## Scope

- In: account-verification OTP, forgot/reset flows, OTP metrics, handler
  plumbing for IP and `Retry-After`, frontend cleanup of the dead
  `otp_send_locked` mapping.
- Non-goals: WhatsApp OTP (Fazpass stays dormant), email templates/HTML,
  announcement broadcast pipeline (as-is), CAPTCHA/abuse penalties beyond
  counting, async send queue, webhook bounce handling, token-format change
  (`newToken` stays deterministic; flagged residual risk).
- Residual risk: SMTP acceptance is not final delivery (async bounces are
  invisible); free-tier Redis restart resets day counters; single-level bot
  mitigation remains IP-count based.
