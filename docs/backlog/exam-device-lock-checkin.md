# Per-exam device lock (check-in exams)

| | |
|---|---|
| **Status** | Fixed — fingerprint stored on the registration; in-exam APIs reject a mismatch |
| **Date** | 2026-08-20 |
| **Surface** | Student check-in, `POST /api/v1/exam/sessions`, reconnect / save / submit / advance / violations |
| **Not this change** | Login/session tokens; exams with `requires_checkin=false`; result and leaderboard; admin reopen / force-submit |

A student who checks in for Exam A on device A must finish Exam A on that device. The same student may check in for Exam B on device B and take Exam B there. Device A then cannot take Exam B.

Login is still allowed on both devices. “Device” is SHA-256(`IP|User-Agent`), unchanged from the old Redis lock.

---

## What the student sees

- Check-in or Start on the wrong device: toast using `exam_device_mismatch` (continue on the check-in device).
- Opening `/exam/sessions/:id` on the wrong device: blocked card (`exam_device_mismatch_title`), no Retry (Retry would re-fetch questions).
- Autosave that gets `403 device_mismatch` does not backoff-retry.

Exams that do not require check-in behave as before.

---

## Four cases

One student, two check-in exams, two fingerprints (`fp-a` / `fp-b`):

| | Exam A (bound to A) | Exam B (bound to B) |
|---|---|---|
| Device A | Start and continue | `device_mismatch` |
| Device B | `device_mismatch` | Start and continue |

Each lock is the **registration**, not the user. Binding Exam A does not bind Exam B.

---

## Why Postgres, not Redis-only

The old lock was Redis `exam:device:{registration_id}`, written on check-in and checked only in `StartSession`.

- Check-in `SET` overwrote the key, so a second device could steal the bind.
- Reconnect, save, submit, section advance, and violations never compared the fingerprint, so a started sitting could continue on another device.
- Redis TTL could expire mid-session (extensions, sectioned papers). Empty key then either locked out the original device or left the sitting unbound.

Source of truth is `exam_registration.device_fingerprint` (migration `0063`). Redis remains a write-through cache and a grandfather path for in-flight Redis keys that were never persisted.

---

## Behaviour

`assertCheckinDevice`:

- `requires_checkin=false` → no-op.
- Stored fingerprint empty → bind on first check-in or first start (if Redis still has a matching key, copy it into Postgres first).
- Stored fingerprint set and different → `ErrDeviceMismatch` → `403` `device_mismatch`.
- Never overwrite a set fingerprint. Same-device check-in is idempotent.

In-exam handlers pass `fingerprint(RealIP, User-Agent)` into reconnect, save, submit, advance, and log-violation. The compare is skipped when the session is already `submitted`, so a stray GET cannot block result navigation. Result and leaderboard handlers are unchanged.

The FR-36 overlay comment (server `current_position`, not localStorage) now says same-device reconnect after a tab close, not “a different device”.

---

## Acceptance

- Check-in on device A, check-in again on A → OK; check-in on B → `device_mismatch`; stored fingerprint stays A.
- Start Exam A with A’s fingerprint → OK; start Exam A with B’s → `device_mismatch`.
- Same student, Exam B bound to B: start B on B → OK; start B on A → `device_mismatch`.
- After start on A: reconnect / save / submit on B → `device_mismatch`; same ops on A → OK.
- `requires_checkin=false`: start and reconnect ignore fingerprint.
- Student UI: mismatch toast on check-in/start; session page blocked card without Retry; save does not retry on `device_mismatch`.
