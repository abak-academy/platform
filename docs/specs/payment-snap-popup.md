---
title: Pay in a Snap popup first, fall back to a new tab
issue: none (asked in Discord by Isaac Panca, 2026-10-11)
status: accepted
created: 2026-10-11
sources: ["web/components/cart/SnapCheckout.tsx", "web/app/(student)/orders/[id]/page.tsx", "web/app/(student)/layout.tsx", "web/lib/snap.ts", "web/types/midtrans.d.ts", "web/lib/hooks/orders.ts", "backend/internal/handler/order.go", "backend/internal/adapter/midtrans.go"]
---

# Pay in a Snap popup first, fall back to a new tab

## Problem and outcome

Paying today always runs `window.open(payment_url, "_blank")`, so the student leaves the page and has to come back and refresh. Outcome: the first attempt opens the Midtrans Snap popup on the same page; only when the popup cannot be used does the flow open the payment page in a new tab, as it does now.

## Verified facts (read from `origin/main`, not run in a browser)

- Checkout and retry already return both `snap_token` and `payment_url` (one Midtrans Snap response: `Token`, `RedirectURL`). No backend change.
- The student layout already loads `snap.js` with the client key from `/config/payment-client-key`; `lib/snap.ts` has `waitForSnap()` and `types/midtrans.d.ts` types `window.snap.pay` with `onSuccess/onPending/onError/onClose`. No call to `snap.pay` exists today.
- Two call sites open the payment page: `SnapCheckout` (student cart; also used by the admin bulk exam order page) and the pay-again action in `orders/[id]/page.tsx`. A popup-blocked branch already sends the student to the order page, where the payment link is shown (PR #227).
- The backend reuses the payment session for the same order, so the same `snap_token` can open the popup again.

## Agreed behavior

- **First touch = popup.** On a successful checkout or retry that returns a `snap_token`, if `window.snap` or a snap.js script tag is on the page, wait up to 3 s for `window.snap` (`waitForSnap`); with neither, fall back at once (admin pages never load snap.js, so waiting there only delays the tab), then call `window.snap.pay(snap_token, callbacks)`. The student stays on the page.
- **Trouble = new page.** If any of these hold, open `payment_url` in a new tab exactly as today (including the existing popup-blocked branch to the order page): `snap_token` is empty, `window.snap` is not there after the wait, or `snap.pay` throws.
- **Callbacks refresh data, never decide status.** `onSuccess` and `onPending` invalidate the order queries, show a toast ("Pembayaran diterima, menunggu konfirmasi" / pending hint) and, on the cart page, send the student to the order page; `onClose` re-enables the button and invalidates the order queries. The paid state still comes only from the server webhook.
- **Two phases.** Before the popup opens, a failure is ours (see the fallback rule above) and goes to the new tab. After it opens, the student picks the payment method inside the popup (the backend sends no `enabled_payments`, so the methods are whatever the Midtrans dashboard has activated; this repo cannot tell which). Whatever the callbacks then report, including `onError`, stays on the page: show an error toast and keep the pay button so the student can retry or pick another method in the popup. It does not open a new tab.
- **Free orders and the `payment_url`-only case are unchanged.** One shared helper in `lib/snap.ts` (`payWithSnap`) is used by both call sites so the rules live in one place.
- The admin bulk exam order page uses `SnapCheckout` too; if `snap.js` is not loaded there (it is loaded by the student layout), it takes the new-tab path. No admin layout change.

## Technical contract

- `web/lib/snap.ts`: add `payWithSnap({ snapToken, paymentUrl, onFinished })` returning which path it took (`"popup"`, `"new-tab"` or `"blocked"`). It throws when the popup is not used and `payment_url` is empty; both callers catch that and show their error toast. Callers keep their own toasts via i18n.
- `SnapCheckout.tsx` and `orders/[id]/page.tsx`: replace the inline `window.open` blocks with the helper; the button label "Bayar di Tab Baru" becomes "Bayar" (id) / "Pay" (en); strings written separately in `lib/i18n.ts`.
- No backend, API, migration or config change.

## Acceptance and proof (vitest, mocked `window.snap`)

1. Token present and `window.snap` ready: `snap.pay` is called with the token, `window.open` is not.
2. No token, or `window.snap` missing after the wait, or `snap.pay` throws: `window.open(payment_url, "_blank")` is called; a blocked tab keeps the order-page fallback.
3. `onSuccess`, `onPending`, `onClose` each invalidate the order queries; none of them sets a paid state.
4. `onError` (fired after the popup opened) shows an error and leaves the button enabled; no new tab.
5. Both call sites (cart and pay-again) behave the same; free order path unchanged.
6. `npx tsc --noEmit` and `npx vitest run` pass; `npm run build` passes.
7. Not provable without a browser and Midtrans sandbox: that the popup renders, and e-wallet deep links and 3-D Secure inside the popup on mobile or in-app browsers. The PR says so.

## Out of scope

Backend or webhook changes, order status rules, the Snap embedded mode (`snap.embed`), other payment gateways, any admin layout change.

## Decided by Panca (2026-10-11)

- "Trouble" is only "the popup cannot open" (token missing, script not loaded, throws). Anything reported after it opened stays in the popup and does not open a new tab.
- The 3 s wait for a still-loading snap.js stands.
- After the popup opened, `onError` shows "Pembayaran gagal. Coba lagi atau pilih metode pembayaran lain." (`payment_failed`); `payment_start_failed` is only for failures before the popup.

- After `onSuccess` / `onPending` on the cart page (`basePath` `/orders`) the student is sent to `/orders/:id`, as the blocked-tab path does. `onClose` and `onError` stay put; the admin bulk order page is never redirected.
- The mobile sandbox test is Panca's.

## Open questions for Panca

- The sandbox test must settle two unknowns: which events fire `onError` (failed charge after choosing a method, expired or invalid token), and whether the transaction's `callbacks.finish` URL (set by the backend when a callback URL exists) redirects the page after payment in popup mode, which would change "stays on the page".
- If snap.js is in the page but failed to load (ad blocker, network), each payment still waits 3 s before the tab, which may then be blocked. Accept, or mark the script's failure in the layout and skip the wait?
