---
title: Bulk print packing slips from the admin orders list
issue: none (requested in Discord by Isaac Panca, 2026-10-10)
status: accepted
created: 2026-10-10
sources: ["backend/internal/service/shipping_label_html.go", "backend/internal/service/shipment_status.go", "backend/internal/service/pdf_generator.go", "backend/internal/handler/admin_order.go", "backend/internal/handler/errors.go", "backend/internal/server/routes.go", "web/app/(admin)/admin/orders/page.tsx", "web/components/admin/OrderDetailModal.tsx", "web/lib/hooks/shipping-label.ts", "web/lib/i18n.ts", "docs/wiki/architecture/shipping-biteship.md", "deploy/pipeline/backend-render-gate.sh"]
---

# Bulk print packing slips

## Problem and outcome

"Cetak Resi" prints one packing slip per order (`GET /admin/orders/:id/label`, 100 x 150 mm), so preparing many parcels means opening each order. Outcome: an admin ticks orders on the current list page, sees which can and cannot be printed and why, and downloads **one PDF with exactly one slip per printable order**.

The document stays Abak's packing slip, not the courier's label (footer unchanged). No evidence yet of admin volume or printer type; the Biteship screenshot is a UX reference only.

## Agreed behavior

Decisions taken from Panca's "go ahead" on 2026-10-10 against the recommendations in the thread; each is cheap to overrule before code is written.

- **Scope of selection:** rows on the active list page only. Header checkbox selects the visible rows. Changing filter, search, sort, page size or page clears the selection (the cursor stack already resets on those).
- **Size picker (requirement, Panca 2026-10-10):** two sizes. `100x150`: a fixed slip, one 100 x 150 mm page per order. `roll80`: thermal roll, PDF page 80 mm wide (the paper), slip content 78 mm wide (the printable area, 1 mm each side), page height fitted to each slip's own content. A4 is not requested and not specified (2 x 2 slips need 200 x 300 mm, A4 is 210 x 297 mm).
- **`roll80` is a new layout, not a shrunk copy:** the original slip was built for 100 mm (78 mm barcode, two-column address blocks of about 39 mm at 78 mm). Expect a single-column stack and a narrower barcode, which needs its own scan test.
- **Per-slip height, no substitute:** each `roll80` slip is its own page with its own height in the same PDF. If the real renderer cannot do this, the work stops and the trade-off goes to Panca; it is not replaced silently by a fixed height or one long page for the batch.
- **Single-order print gets the same picker (Panca 2026-10-10):** `GET /admin/orders/:id/label?size=`; absent `size` means `100x150`, so existing callers keep the default page geometry. "Cetak Resi" closes `OrderDetailModal` and opens the shared review dialog in single-order mode, default `100x150`, not persisted (per-admin defaults stay out of scope).
- **Eligibility** (bulk only; single-order print keeps its tracking-number-only rule):
  - the order exists;
  - `tracking_number` is non-empty (manual or Biteship);
  - `shipment_status` is not a failure status (`isShipmentFailureStatus`: rejected, courier_not_found, cancelled, return_in_transit, returned, disposed).
  Divergence to note: single print today refuses only a missing tracking number, so it still prints a slip for a failed shipment.
- **Review before print:** the dialog counts selected / printable / skipped from the row data already loaded, lists skipped orders with the reason, and sends **only the printable ids**. Zero printable disables the action.
- **Fail closed on the server:** the server re-reads every order and re-checks eligibility. If any id is not printable it returns 409 with a per-id reason list and renders nothing; the dialog then re-shows the review with the fresh result. No silent partial PDF.
- **Limit:** 1..100 ids per request (the maximum list page size), duplicates rejected, enforced in the service.
- **Output:** one PDF, slips in request order (= row order on screen), one slip per page.
- **Synchronous:** one request, one Gotenberg call. No job, no progress bar. Revisit if measured render time approaches the 30 s `defaultGotenbergTimeout`.
- **No side effects:** printing changes no order or shipment state.

## Technical contract

- `POST /api/v1/admin/orders/labels`, body `{"order_ids": ["<uuid>", ...], "size": "100x150" | "roll80"}` (`size` required; unknown value is 400), in the existing `orders:write` group (same gate as single print). Registered where it cannot be read as `/:id`.
  - 200 `application/pdf`.
  - 400 `invalid_request`: empty list, more than 100, duplicate or malformed id.
  - 409 `labels_not_printable` with `details: [{"order_id": "...", "reason": "not_found" | "no_tracking_number" | "shipment_failed"}]` (existing `APIError.Details`).
- `service/shipping_label_html.go`: extract the per-order data building out of `renderShippingLabel` and let the template loop over a slice of slips. Single and bulk print share the same renderer (test: identical one-slip output for the same size and order). `.slip` gets `break-after: page` except the last; the fixed `html,body` height must not clip a multi-page document. `labelMaxPrintedItems` and the one-page `overflow:hidden` guarantee apply per slip.
- `roll80` page heights: each slip is a named CSS page (`@page sN { size: 80mm <h>mm; margin: 0 }`, `page: sN`). `<h>` comes from measuring the slip in the browser with a small inline script before print (the address wraps, so Go cannot know the height). Spiked 2026-10-10 in headless Chromium (not Gotenberg) with the real slip HTML at 80 mm pages and 78 mm content: five slips (1, 3 and 8 items, short and long addresses) gave five pages 80.1 mm wide of 154.2 / 172.1 / 177.1 / 172.1 / 154.2 mm, footer on every page, and the page rendered to PNG showed nothing clipped. That spike used a single-column stack and a 70 mm barcode, which the implementation should start from. Still to prove on Gotenberg (Panca: best effort, CI gate is the proof); whether Gotenberg runs the script and applies `@page` set at that point is the main unknown. Fallback if it does not: height computed in Go from item count and address length, to be agreed with Panca first.
- New service method (usecase owns the rule, not the repository): validates the list, loads each order with `GetOrderByID` (no new repository query; up to 100 sequential reads, kept deliberately to avoid a new query), collects every ineligible order before rendering, then one `RenderHTML` call. New sentinel error carrying the per-id reasons; `handler/errors.go` maps it to 409.
- Concurrency: at most one bulk render at a time per API process (the Gotenberg sidecar is 768 MB and shared with certificate rendering); a second concurrent request waits or is refused (choose the smaller code and say which in the PR).
- `web/`: checkbox column and header checkbox in the orders table (`orders/page.tsx`), a selection bar (count, clear, "Cetak resi"), a review dialog component, and a download helper next to `downloadShippingLabel` that POSTs and, on 409, returns the reason list instead of a blob. `i18n.ts` gets id and en strings. The dialog traps and restores focus, is usable at narrow widths, and disables the confirm button while the request is in flight (no duplicate download).
- `deploy/pipeline/backend-render-gate.sh`: add the new render gate test to the `-run` pattern so it cannot sit unrun.
- `docs/wiki/architecture/shipping-biteship.md`: document the endpoint and the eligibility rule.

## Acceptance and proof

1. Service, fake renderer: three printable orders produce one `RenderHTML` call whose HTML holds three slips in request order, each with its own waybill.
2. Service: an order with no tracking number, one with a failure `shipment_status` (both spellings, e.g. `courier_not_found` and `courierNotFound`), and an unknown id each yield the 409 error with the right reason, and the renderer is never called. A mix of valid and invalid ids also renders nothing.
3. Service: empty list, 101 ids and duplicate ids are rejected; 100 ids are accepted.
4. Single print: absent `size` and explicit `100x150` use the same renderer and content, on one 100 × 150 mm page; `size=roll80` yields one fitted 80 mm page; unknown `size` is 400. Both sizes retain every populated slip field and the waybill barcode. Single and bulk use the same one-slip renderer; single eligibility still requires only a tracking number. The shared review passes the chosen size.
5. Handler: 200 returns `application/pdf`; 409 body matches the contract; 400 cases; a role without `orders:write` is refused (route gate, as for `/:id/label`).
6. Real Gotenberg gate (`gotenberg_integration`): `100x150`: N slips yield exactly N pages at 100 x 150 mm; item-capped slip still one page. `roll80`: N slips yield exactly N pages, every page 80 mm wide, heights differ between a short slip and one with many items and a long address, no page shorter than its content (no clipped text, barcode or footer) and no blank page. The test reads the page boxes from the PDF. Runs in CI via `backend-render-gate.sh`; not runnable on the dev box without a container runtime.
7. Web: header checkbox selects visible rows only; filter/search/page change clears the selection; the dialog lists skipped rows with reasons and disables confirm at zero printable; a 409 re-shows the review; confirm is disabled while in flight. `npx tsc --noEmit` and `npx vitest run` pass.
8. Real-browser proof with a mocked API covers bulk normal, mixed, 409, zero printable, pending, retry, and single print in both locales at desktop and 320 px width, including reachable footer actions and focus return. Physical barcode scanning, feed/cut, and Gotenberg output require their own proof.

## Popup and layout revision (Panca, 2026-10-10)

The review starts with selected / ready / skipped counts, explains fixed sticker and fitted thermal-roll sizes in radio cards, and lists buyer names, short order ids, printable waybills, and specific skipped reasons. Skipped orders appear first; the download action states the printable count. A 409 keeps the dialog open with fresh reasons and an explicit review-again message. Zero printable disables download; pending prevents close and duplicate downloads; errors offer retry. Single print uses this same dialog after closing the detail modal.

Both sizes share one template with thin rules, whitespace, and an explicit Liberation Sans / Arial / Helvetica / sans-serif stack. Reading order is courier/service, waybill text and barcode, recipient, sender, full order UUID as text with quantity/weight, items, shipping cost, and the packing-slip footer with optional sender email. Fixed pages distribute spare space between sections; roll80 keeps its 78 mm printable container with inner padding giving 73 mm text width and its existing per-slip named-page measurement. Panca's v3 decision removes the UUID barcode because its space is more useful for the recipient address and header; the full UUID remains text. The header embeds the courier's own logo from backend copies of the five web courier assets, mapped by lowercased CourierCode, with courier-name fallback for nil, empty, unknown, or missing assets. A test resolves paths from its source file and checks every backend copy byte-for-byte against web/public/couriers. The canonical Abak mark accompanies hardcoded `abak academy` on a single line (Panca: never two lines) and hub.abakacademy.id, centred below the wordmark. Navy/teal colours supplement size, weight, and position; print CSS darkens the two images for thresholded thermal legibility without changing their source files. Recipient name/phone/address use 12/9.5/10 pt (13 pt address leading); sender name and phone use 8.2 pt bold, with 7.8 pt address (10 pt leading). The intentional visual revision replaces legacy HTML hash equality with content, shared-renderer, and geometry checks. Single-print eligibility and the absent-size API default remain unchanged.

## Out of scope

Select across pages or filters, A4 or any size beyond `100x150` and `roll80`, hiding or masking slip fields, courier-official labels, per-admin defaults, async jobs, changing single-print eligibility, any order or shipment state change.

## Open questions left for Panca

- Should single print also refuse failed shipments (consistent rules, but changes today's behavior)?
- `roll80` printer facts are unproven: model, driver feed/cut behavior, whether 1 mm side margin is enough, barcode scan at the narrower width. Needs a real print of a short, a long and a mixed batch.
- Cap of 100 is a limit, not a measured safe size: render time and Gotenberg memory for 20/50/100 slips are unmeasured.
