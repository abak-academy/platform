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
 */

async function findPricedCourseProduct(request: APIRequestContext): Promise<{ id: string }> {
  const res = await request.get(`${API_BASE}/products?type=course`);
  if (!res.ok()) {
    throw new Error(`real-backend setup: listing course products failed (${res.status()})`);
  }
  const body = (await res.json()) as { data?: Array<{ id: string; price: number }> };
  const found = body.data?.find((p) => p.price > 0);
  if (!found) {
    throw new Error(
      "real-backend setup: no priced course product exists in the catalog — " +
        "publish one via /admin/products before running this spec."
    );
  }
  return { id: found.id };
}

async function checkoutToPaymentPending(
  request: APIRequestContext,
  studentToken: string,
  productId: string
): Promise<string> {
  const cartRes = await request.post(`${API_BASE}/orders`, {
    headers: { Authorization: `Bearer ${studentToken}` },
  });
  expect(cartRes.ok(), "minting the cart should succeed").toBeTruthy();
  const cart = (await cartRes.json()) as { id: string };

  const itemRes = await request.post(`${API_BASE}/orders/${cart.id}/items`, {
    headers: { Authorization: `Bearer ${studentToken}` },
    data: { product_id: productId, qty: 1 },
  });
  expect(itemRes.ok(), "adding the item to the cart should succeed").toBeTruthy();

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

test.describe("FB-19a/b/c — admin manual confirm with proof, FR-33 buyer name", () => {
  test("upload a proof, confirm manually, reopen and see the mark and the proof", async ({ page, context, request }) => {
    await loginRealAdmin(context, request);
    const studentToken = await loginRealStudent(context, request);

    const product = await findPricedCourseProduct(request);
    const orderId = await checkoutToPaymentPending(request, studentToken, product.id);
    const orderNumber = `#${orderId.slice(-8)}`;

    await page.goto("/admin/orders");

    const orderRow = page.getByRole("button", { name: new RegExp(`Lihat detail pesanan ${orderNumber}`) });
    await expect(orderRow).toBeVisible({ timeout: 15000 });

    // FR-33: the buyer's name is the primary label in the list row, and the
    // student_id (a UUID, not the old "...<last12chars>" label) is present too.
    const rowText = (await orderRow.innerText()) ?? "";
    expect(rowText).not.toMatch(/^\.\.\./m);

    await orderRow.getByRole("button", { name: "Konfirmasi" }).click();

    const confirmDialog = page.getByRole("dialog", { name: "Konfirmasi Pembayaran Manual" });
    await expect(confirmDialog).toBeVisible();

    // Konfirmasi must stay disabled until a proof is uploaded (FR-25).
    await expect(confirmDialog.getByRole("button", { name: "Konfirmasi" })).toBeDisabled();

    await confirmDialog.locator('input[data-testid="confirm-order-proof-input"]').setInputFiles({
      name: "bukti-transfer.png",
      mimeType: "image/png",
      // Minimal valid 1x1 PNG — no real payment data, purely a fixture.
      buffer: Buffer.from(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
        "base64"
      ),
    });

    await expect(confirmDialog.getByRole("link", { name: "Lihat bukti yang diunggah" })).toBeVisible({
      timeout: 10000,
    });
    await expect(confirmDialog.getByRole("button", { name: "Konfirmasi" })).toBeEnabled();
    await confirmDialog.getByRole("button", { name: "Konfirmasi" }).click();

    await expect(page.getByText("Pesanan dikonfirmasi.")).toBeVisible({ timeout: 10000 });
    await expect(confirmDialog).toBeHidden();

    // Reopen the order detail and check the manual-confirm mark + proof link.
    await orderRow.click();
    const detailDialog = page.getByRole("dialog").filter({ hasText: orderNumber });
    await expect(detailDialog).toBeVisible();
    await expect(detailDialog.getByText("Dikonfirmasi manual")).toBeVisible();

    const proofLink = detailDialog.getByRole("link", { name: "Lihat bukti" });
    await expect(proofLink).toBeVisible();
    const href = await proofLink.getAttribute("href");
    expect(href, "the proof link must carry the object key as an href").toBeTruthy();

    const proofFileRes = await request.get(href!);
    expect(proofFileRes.ok(), "the payment proof must be reachable from the detail view").toBeTruthy();
  });
});
