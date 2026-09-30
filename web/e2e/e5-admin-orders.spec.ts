import { test, expect, type APIRequestContext } from "@playwright/test";
import { API_BASE, loginRealAdmin, loginRealStudent } from "./helpers/session";

/**
 * FB-19a/b/c acceptance path (spec.md FR-25/FR-26/FR-27/FR-31): an admin opens
 * a payment_pending order, uploads a payment proof, confirms it manually, then
 * reopens the order and sees the "Dikonfirmasi manual" mark with the proof
 * reachable from the detail view. Also exercises FR-33 (buyer name, not a
 * truncated student_id, in the list and the detail view).
 *
 * Real-backend pattern — see helpers/session.ts for the account contract
 * (E2E_ADMIN_IDENTIFIER/PASSWORD, E2E_STUDENT_IDENTIFIER/PASSWORD). The admin
 * account additionally needs the orders:write capability (admin_store or
 * super_admin) — the same capability POST .../confirm and the payment_proof
 * presign both require.
 *
 * Getting a real payment_pending order needs a real checkout, which calls the
 * configured payment gateway (Midtrans sandbox in local/staging). This is an
 * extra environment dependency on top of the usual accounts: if the backend
 * under test has no gateway credentials wired up, checkoutToPaymentPending
 * below fails with a clear "real-backend setup" message rather than a mystery
 * timeout. A course product is used deliberately — it has no shipping/courier
 * prerequisite, unlike a physical product.
 *
 * A passing run leaves the student owning the course it bought, so the spec
 * refunds the order at the end: the refund revokes the enrollment and releases
 * the digital claim, which is what keeps a second run possible. Discovery also
 * pages through the whole catalog, so a run that died before that refund can
 * still find another course to buy.
 */

/** Minimal valid 1x1 PNG — no real payment data, purely a fixture. */
const PNG_1X1 = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
  "base64"
);

async function findPricedCourseProductIds(request: APIRequestContext): Promise<string[]> {
  // The catalog is cursor-paginated, so one page is not the whole catalog: a
  // rerun that already consumed the first page's courses must be able to reach
  // the remaining pages before concluding there is nothing left to buy.
  const ids: string[] = [];
  let cursor = "";
  do {
    const query = new URLSearchParams({ type: "course" });
    if (cursor) query.set("cursor", cursor);
    const res = await request.get(`${API_BASE}/products?${query.toString()}`);
    if (!res.ok()) {
      throw new Error(`real-backend setup: listing course products failed (${res.status()})`);
    }
    const body = (await res.json()) as {
      data?: Array<{ id: string; price: number }>;
      next_cursor?: string | null;
    };
    ids.push(...(body.data ?? []).filter((p) => p.price > 0).map((p) => p.id));
    cursor = body.next_cursor ?? "";
  } while (cursor);

  if (ids.length === 0) {
    throw new Error(
      "real-backend setup: no priced course product exists in the catalog — " +
        "publish one via /admin/products before running this spec."
    );
  }
  return ids;
}

async function checkoutToPaymentPending(
  request: APIRequestContext,
  studentToken: string,
  productIds: string[]
): Promise<string> {
  const cartRes = await request.post(`${API_BASE}/orders`, {
    headers: { Authorization: `Bearer ${studentToken}` },
  });
  expect(cartRes.ok(), "minting the cart should succeed").toBeTruthy();
  const cart = (await cartRes.json()) as { id: string; items?: Array<{ id: string }> };

  // POST /orders returns the student's existing open cart, not a fresh one.
  // Anything another spec left in it comes along — and a leftover physical item
  // makes this checkout demand a shipping selection the digital-only flow never
  // makes. Empty it so the cart contains exactly what this test puts there.
  for (const leftover of cart.items ?? []) {
    const del = await request.delete(`${API_BASE}/orders/${cart.id}/items/${leftover.id}`, {
      headers: { Authorization: `Bearer ${studentToken}` },
    });
    expect(del.ok(), "clearing a leftover cart item should succeed").toBeTruthy();
  }

  // A digital product the student already owns or holds a live claim on is
  // refused with 409 digital_target_conflict, so a rerun (or a run that died
  // before confirm) cannot reuse the same course. Take the first one that adds.
  let added = false;
  let lastFailure = "no candidate product was tried";
  for (const productId of productIds) {
    const itemRes = await request.post(`${API_BASE}/orders/${cart.id}/items`, {
      headers: { Authorization: `Bearer ${studentToken}` },
      data: { product_id: productId, qty: 1 },
    });
    if (itemRes.ok()) {
      added = true;
      break;
    }
    const failure = (await itemRes.json().catch(() => ({}))) as { code?: string };
    lastFailure = `${itemRes.status()} ${failure.code ?? "unknown"}`;
    // 409 is the expected rerun case: the student owns the course or holds a
    // live claim on it, so try the next one. 429 is the limiter, not ownership —
    // fail here rather than drain the catalog and blame ownership for it.
    if (itemRes.status() === 409) continue;
    if (itemRes.status() === 429) {
      throw new Error("real-backend setup: adding a course to the cart hit the rate limiter (429)");
    }
    expect(
      itemRes.ok(),
      `adding a priced course to the cart failed unexpectedly (${lastFailure})`
    ).toBeTruthy();
  }
  if (!added) {
    throw new Error(
      `real-backend setup: no priced course could be added to the cart (last failure: ${lastFailure}) — ` +
        "the student already owns or holds a live claim on every one of them: " +
        "refund the previous run's order, or publish another priced course via /admin/products."
    );
  }

  const checkoutRes = await request.post(`${API_BASE}/orders/${cart.id}/checkout`, {
    headers: {
      Authorization: `Bearer ${studentToken}`,
      "Idempotency-Key": `e2e-admin-orders-${Date.now()}`,
    },
  });
  if (!checkoutRes.ok()) {
    throw new Error(
      `real-backend setup: checkout failed (${checkoutRes.status()}) — the backend needs a working ` +
        `payment gateway (Midtrans sandbox) to reach payment_pending. Body: ${await checkoutRes.text()}`
    );
  }

  const studentOrderRes = await request.get(`${API_BASE}/orders/${cart.id}`, {
    headers: { Authorization: `Bearer ${studentToken}` },
  });
  expect(studentOrderRes.ok(), "fetching the order after checkout should succeed").toBeTruthy();
  const order = (await studentOrderRes.json()) as { status: string };
  expect(order.status, "checkout should leave the order payment_pending, ready for manual confirm").toBe(
    "payment_pending"
  );

  return cart.id;
}

// Set once the order and an admin token exist, so cleanup runs even when an
// assertion fails: a broken run must not silently consume a course.
let cleanup: { adminToken: string; orderId: string } | null = null;

/** Refund a confirmed order, so the course it consumed goes back to the catalog. */
async function refundOrder(
  request: APIRequestContext,
  adminToken: string,
  orderId: string
): Promise<void> {
  const auth = { Authorization: `Bearer ${adminToken}` };
  // POST, not GET: the presign route is registered as POST and reads the query string.
  const presignRes = await request.post(
    `${API_BASE}/uploads/presign?kind=refund_proof&filename=e2e-refund.png&content_type=image/png`,
    { headers: auth }
  );
  if (!presignRes.ok()) {
    throw new Error(`presigning the refund proof failed (${presignRes.status()})`);
  }
  const presigned = (await presignRes.json()) as { url: string; key: string };

  // A well-formed key is not enough: the refund path checks the object exists.
  const putRes = await request.put(presigned.url, {
    headers: { "Content-Type": "image/png" },
    data: PNG_1X1,
  });
  if (!putRes.ok()) {
    throw new Error(`uploading the refund proof failed (${putRes.status()})`);
  }

  const refundRes = await request.post(`${API_BASE}/admin/orders/${orderId}/refund`, {
    headers: auth,
    data: { refund_proof_url: presigned.key },
  });
  if (!refundRes.ok()) {
    throw new Error(`refunding failed (${refundRes.status()}) ${await refundRes.text()}`);
  }
}

test.describe("FB-19a/b/c — admin manual confirm with proof, FR-33 buyer name", () => {
  test.afterEach(async ({ request }) => {
    const pending = cleanup;
    cleanup = null;
    if (!pending) return;
    try {
      await refundOrder(request, pending.adminToken, pending.orderId);
    } catch (err) {
      const message = `[e5-admin-orders] could not refund ${pending.orderId}: ${(err as Error).message}`;
      // A clean run that cannot hand the course back is a real failure; after an
      // earlier failure it stays a warning, so the original error is reported.
      if (test.info().errors.length === 0) throw new Error(message);
      console.warn(message);
    }
  });

  test("upload a proof, confirm manually, reopen and see the mark and the proof", async ({ page, context, request }) => {
    // The student session is only needed for the API setup below, but both
    // helpers seed the SAME localStorage key — so the last login wins in the
    // browser. Do the student work first, then re-seed admin, or /admin/orders
    // redirects to the student dashboard and the row is never found.
    const studentToken = await loginRealStudent(context, request);

    const productIds = await findPricedCourseProductIds(request);
    const orderId = await checkoutToPaymentPending(request, studentToken, productIds);
    const orderNumber = `#${orderId.slice(-8)}`;

    const adminToken = await loginRealAdmin(context, request);
    cleanup = { adminToken, orderId };

    await page.goto("/admin/orders");

    // The row is a <tr>, not a button: it used to carry role="button" while
    // containing buttons, which lied to screen readers. The accessible name
    // moved onto the order number inside it, so the row is located by role=row
    // and the open control is addressed separately.
    const orderRow = page.getByRole("row").filter({ hasText: orderNumber });
    await expect(orderRow).toBeVisible({ timeout: 15000 });

    const openDetail = orderRow.getByRole("button", {
      name: new RegExp(`Lihat detail pesanan ${orderNumber}`),
    });
    await expect(openDetail).toBeVisible();

    // FR-33: the buyer's name is the primary label in the list row, and the
    // student_id (a UUID, not the old "...<last12chars>" label) is present too.
    const rowText = (await orderRow.innerText()) ?? "";
    expect(rowText).not.toMatch(/^\.\.\./m);

    // Confirm is the primary action for a payment_pending order, so it stays a
    // button in the row rather than moving into the overflow menu.
    await orderRow.getByRole("button", { name: "Konfirmasi" }).click();

    const confirmDialog = page.getByRole("dialog", { name: "Konfirmasi Pembayaran Manual" });
    await expect(confirmDialog).toBeVisible();

    // Konfirmasi must stay disabled until a proof is uploaded (FR-25).
    await expect(confirmDialog.getByRole("button", { name: "Konfirmasi" })).toBeDisabled();

    await confirmDialog.locator('input[data-testid="confirm-order-proof-input"]').setInputFiles({
      name: "bukti-transfer.png",
      mimeType: "image/png",
      buffer: PNG_1X1,
    });

    await expect(confirmDialog.getByRole("link", { name: "Lihat bukti yang diunggah" })).toBeVisible({
      timeout: 10000,
    });
    await expect(confirmDialog.getByRole("button", { name: "Konfirmasi" })).toBeEnabled();
    await confirmDialog.getByRole("button", { name: "Konfirmasi" }).click();

    await expect(page.getByText("Pesanan dikonfirmasi.")).toBeVisible({ timeout: 10000 });
    await expect(confirmDialog).toBeHidden();

    // Reopen the order detail and check the manual-confirm mark + proof access.
    await openDetail.click();
    const detailDialog = page.getByRole("dialog").filter({ hasText: orderNumber });
    await expect(detailDialog).toBeVisible();
    await expect(detailDialog.getByText("Dikonfirmasi manual")).toBeVisible();

    // Deliberately a button, not an <a href>: payment_proof is no longer served
    // by the unauthenticated /files/* proxy, so the object key must not be
    // rendered into the page at all. The URL is minted per click, behind
    // orders:write, and expires.
    const proofButton = detailDialog.getByRole("button", { name: "Lihat bukti" });
    await expect(proofButton).toBeVisible();
    await expect(detailDialog.getByRole("link", { name: "Lihat bukti" })).toHaveCount(0);
    expect(
      await detailDialog.innerHTML(),
      "the proof object key must never reach the DOM"
    ).not.toContain("payment_proof/");

    // Clicking opens the minted URL in a new tab; assert it actually resolves.
    const [proofTab] = await Promise.all([
      context.waitForEvent("page"),
      proofButton.click(),
    ]);
    const proofURL = proofTab.url();
    expect(proofURL, "the minted proof URL must be a presigned storage link").toContain("payment_proof/");
    const proofFileRes = await request.get(proofURL);
    expect(proofFileRes.ok(), "the payment proof must be reachable from the detail view").toBeTruthy();
    await proofTab.close();
  });
});
