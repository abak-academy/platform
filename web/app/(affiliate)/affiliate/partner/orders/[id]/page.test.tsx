import { act, render, screen } from "@testing-library/react";
import { Suspense } from "react";
import { describe, expect, it, vi } from "vitest";
import PartnerOrderDetailPage from "./page";

const hook = vi.hoisted(() => ({ state: {} as Record<string, unknown> }));

vi.mock("@/lib/hooks/partner", () => ({
  usePartnerOrder: () => hook.state,
  partnerKeys: { order: (id: string) => ["partner", "order", id] },
}));
vi.mock("@/components/partner/RetryPaymentButton", () => ({ RetryPaymentButton: () => <button>Bayar</button> }));
vi.mock("@/components/cart/SnapCheckout", () => ({ SnapCheckout: () => <button>Bayar</button> }));

async function renderPage() {
  await act(async () => {
    render(
      <Suspense fallback={null}>
        <PartnerOrderDetailPage params={Promise.resolve({ id: "order-12345678" })} />
      </Suspense>,
    );
  });
}

const order = {
  id: "order-12345678",
  status: "payment_pending",
  subtotal: 200000,
  discount: 20000,
  total: 180000,
  created_at: "2026-11-09T10:00:00Z",
  items: [{ product_id: "p1", name: "Tryout UTBK", qty: 2, unit_price: 100000 }],
  students: [{ id: "s1", name: "Ani", school_name: "SMA Contoh" }, { id: "s2", name: "Budi", school_name: null }],
  promo_code: "DEMO-ORDER10",
};

describe("PartnerOrderDetailPage", () => {
  it("shows what was bought, the voucher code used, the students, and a way to pay a pending order", async () => {
    hook.state = { isLoading: false, isError: false, data: order };
    await renderPage();

    expect(screen.getByText("Tryout UTBK")).toBeInTheDocument();
    expect(screen.getByText("DEMO-ORDER10")).toBeInTheDocument();
    expect(screen.getByText("Ani")).toBeInTheDocument();
    expect(screen.getByText("Budi")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bayar" })).toBeInTheDocument();
  });

  it("offers a way to pay a freshly created, unpaid order", async () => {
    hook.state = { isLoading: false, isError: false, data: { ...order, status: "cart" } };
    await renderPage();

    expect(screen.getByRole("button", { name: "Bayar" })).toBeInTheDocument();
  });

  it("says no voucher was used, and offers no payment for a paid order", async () => {
    hook.state = { isLoading: false, isError: false, data: { ...order, status: "paid", promo_code: undefined } };
    await renderPage();

    expect(screen.getByText("Tanpa voucher")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Bayar" })).toBeNull();
  });
});
