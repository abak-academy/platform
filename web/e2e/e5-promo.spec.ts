import { test, expect, type APIRequestContext } from "@playwright/test";
import { API_BASE, loginRealAdmin, loginRealStudent } from "./helpers/session";

/**
 * B-1 acceptance path (spec.md FR-4, FR-5): apply a promo on /cart, save the
 * shipping address, then select a courier — the promo must still be on the
 * order afterward. This is the only thing that actually proves the
 * production bug (PatchCart dropping promo_code_id/discount on any patch
 * that didn't re-send promo_code) is dead; the vitest suite mocks
 * usePatchCart entirely and cannot see the real tri-state wire contract.
 *
 * Real-backend pattern — see helpers/session.ts for the student credentials
 * contract (E2E_STUDENT_IDENTIFIER/PASSWORD).
 *
 * Setup also needs an admin session that can create a promo code
 * (POST /admin/promo-codes, capability `promos:write`). That capability
 * belongs to `admin_store` / `super_admin`, not `admin_exam` — the role the
 * other specs in this suite document using for E2E_ADMIN_IDENTIFIER/PASSWORD
 * (see question-editor.spec.ts). Point E2E_ADMIN_IDENTIFIER/PASSWORD at a
 * store/super admin to run this spec.
 *
 * The physical product used to build the cart is read from the live catalog
 * (GET /products, no auth) rather than created here, so this spec does not
 * additionally need products:write — it only needs at least one book,
 * merchandise or medal product to already exist.
 */

async function createPromoCode(
  request: APIRequestContext,
  adminToken: string,
  code: string,
  opts: { isPublic?: boolean } = {}
): Promise<void> {
  const res = await request.post(`${API_BASE}/admin/promo-codes`, {
    headers: { Authorization: `Bearer ${adminToken}` },
    data: {
      code,
      discount_amount: 10000,
      min_order_amount: 0,
      is_public: opts.isPublic ?? false,
    },
  });
  if (!res.ok()) {
    throw new Error(
      `real-backend setup: creating promo code ${code} failed (${res.status()}) — ` +
        "the admin account needs the promos:write capability (admin_store or super_admin), " +
        `not admin_exam. Body: ${await res.text()}`
    );
  }
}

async function findPhysicalProduct(request: APIRequestContext): Promise<{ id: string; price: number }> {
  for (const type of ["book", "merchandise", "medal"]) {
    const res = await request.get(`${API_BASE}/products?type=${type}`);
    if (!res.ok()) continue;
    const body = (await res.json()) as { data?: Array<{ id: string; price: number }> };
    const found = body.data?.find((p) => p.price > 0);
    if (found) return { id: found.id, price: found.price };
  }
  throw new Error(
    "real-backend setup: no physical product (book/merchandise/medal) exists in the catalog — " +
      "create and publish one via /admin/products before running this spec."
  );
}

async function mintCartWithItem(
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

  // POST /orders returns the student's existing open cart, so a promo attached
  // by an earlier test is still on it — and the active-promo list is hidden
  // while one is applied, which silently breaks the FR-14 case. Clear it with
  // the tri-state remove ("" = remove) so every test starts promo-free.
  const clearRes = await request.patch(`${API_BASE}/orders/${cart.id}`, {
    headers: { Authorization: `Bearer ${studentToken}` },
    data: { promo_code: "" },
  });
  expect(clearRes.ok(), "clearing any carried-over promo should succeed").toBeTruthy();

  return cart.id;
}

interface OrderSnapshot {
  promo_code_id: string | null;
  discount: number;
  subtotal: number;
  shipping_cost: number;
  total: number;
}

async function fetchOrder(request: APIRequestContext, studentToken: string, orderId: string): Promise<OrderSnapshot> {
  const res = await request.get(`${API_BASE}/orders/${orderId}`, {
    headers: { Authorization: `Bearer ${studentToken}` },
  });
  expect(res.ok(), "fetching the order should succeed").toBeTruthy();
  return res.json();
}

test.describe("B-1 — promo survives the address and courier patches", () => {
  test("apply promo -> save address -> select courier -> discount is still on the order", async ({
    page,
    context,
    request,
  }) => {
    const adminToken = await loginRealAdmin(context, request);
    const studentToken = await loginRealStudent(context, request);

    const promoCode = `E2EPROMO${Date.now()}`;
    await createPromoCode(request, adminToken, promoCode);

    const product = await findPhysicalProduct(request);
    const orderId = await mintCartWithItem(request, studentToken, product.id);

    await page.goto("/cart");

    // Apply the promo through the real UI path (FR-5): validate, then the
    // dedicated patch attaches promo_code to the order.
    await page.getByPlaceholder("Masukkan kode promo").fill(promoCode);
    await page.getByRole("button", { name: "Pakai" }).click();
    await expect(page.getByText(/Promo diterapkan/)).toBeVisible({ timeout: 10000 });

    let order = await fetchOrder(request, studentToken, orderId);
    expect(order.promo_code_id, "promo should be attached right after apply").toBeTruthy();
    expect(order.discount, "discount should be > 0 right after apply").toBeGreaterThan(0);
    const appliedDiscount = order.discount;

    // Save the shipping address. Per FR-5 this patch deliberately omits
    // promo_code — if the omission ever regressed into COALESCE-less
    // clobbering, this is where the promo would silently disappear.
    await page.getByLabel("Nama penerima").fill("Budi Tester");
    await page.getByLabel("Nomor telepon").fill("081200000001");
    await page.getByLabel("Alamat lengkap").fill("Jl. Contoh Pengujian No. 1");

    const provinces = (await (await request.get(`${API_BASE}/provinces`)).json()) as Array<{
      id: string;
      name: string;
    }>;
    expect(provinces.length, "seed at least one province locally").toBeGreaterThan(0);
    const province = provinces[0];
    await page.locator("#provinsi").click();
    await page.getByRole("option", { name: province.name, exact: true }).click();

    const cities = (await (await request.get(`${API_BASE}/provinces/${province.id}/cities`)).json()) as Array<{
      id: string;
      name: string;
    }>;
    expect(cities.length, "seed at least one city for the chosen province").toBeGreaterThan(0);
    const city = cities[0];
    await expect(page.locator("#kota")).toBeEnabled();
    await page.locator("#kota").click();
    await page.getByRole("option", { name: city.name, exact: true }).click();

    const districts = (await (await request.get(`${API_BASE}/cities/${city.id}/districts`)).json()) as Array<{
      id: string;
      name: string;
    }>;
    expect(districts.length, "seed at least one district for the chosen city").toBeGreaterThan(0);
    const district = districts[0];
    await expect(page.locator("#kecamatan")).toBeEnabled();
    await page.locator("#kecamatan").click();
    await page.getByRole("option", { name: district.name, exact: true }).click();

    await page.getByLabel("Kode Pos").fill("40123");
    await page.getByRole("button", { name: "Simpan Alamat" }).click();

    order = await fetchOrder(request, studentToken, orderId);
    expect(order.promo_code_id, "promo must survive the address patch (FR-4)").toBeTruthy();
    expect(order.discount).toBe(appliedDiscount);

    // Select a courier — a second, unrelated patch. Before B-1, this is
    // exactly the patch that wiped promo_code_id because PatchCart never
    // seeded it from the carried-forward order.
    // Saving the address already kicks off the rate fetch (handleSaveAddress
    // calls handleCheckShipping), so "Cek Ongkir" is disabled and then replaced
    // by the picker — clicking it races the swap. Wait for the picker instead.
    //
    // Name the toggle explicitly. A bare getByRole("button", { expanded: false })
    // also matches the Next.js Dev Tools button, so it is a strict-mode
    // violation against any dev server — this spec could never have passed.
    const courierToggle = page.getByRole("button", { name: "Pilih jasa pengiriman" });
    await expect(courierToggle).toBeVisible({ timeout: 15000 });
    await courierToggle.click();
    await page.getByRole("radio").first().click();

    await expect(page.getByText(/Diskon/)).toBeVisible();

    order = await fetchOrder(request, studentToken, orderId);
    expect(
      order.promo_code_id,
      "promo must survive the courier patch too — this is the production bug (FR-4)"
    ).toBeTruthy();
    expect(order.discount).toBe(appliedDiscount);
    expect(order.total).toBe(order.subtotal - order.discount + order.shipping_cost);
  });
});

// FR-14: the active-promo list next to PromoInput is a picker into the same
// apply path FR-5 built, not a second mechanism — this proves that end to
// end against the real backend rather than a mocked usePatchCart.
test.describe("FR-14 — picking a listed active promo applies it and the discount survives to checkout", () => {
  test("select a listed promo from /cart -> discount lands on the order", async ({ page, context, request }) => {
    const adminToken = await loginRealAdmin(context, request);
    const studentToken = await loginRealStudent(context, request);

    const promoCode = `E2EPUBLICPROMO${Date.now()}`;
    await createPromoCode(request, adminToken, promoCode, { isPublic: true });

    const product = await findPhysicalProduct(request);
    const orderId = await mintCartWithItem(request, studentToken, product.id);

    await page.goto("/cart");

    // The listed promo is picked, not typed — this is the case that
    // distinguishes FR-14 from the FR-5 manual-entry path above.
    await page.getByRole("button", { name: new RegExp(promoCode) }).click();
    await expect(page.getByText(/Promo diterapkan/)).toBeVisible({ timeout: 10000 });

    const order = await fetchOrder(request, studentToken, orderId);
    expect(order.promo_code_id, "promo picked from the list must attach to the order").toBeTruthy();
    expect(order.discount, "discount should be > 0 after picking the listed promo").toBeGreaterThan(0);
    expect(order.total).toBe(order.subtotal - order.discount + order.shipping_cost);
  });
});
