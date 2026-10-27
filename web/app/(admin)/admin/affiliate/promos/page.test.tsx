import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import AffiliatePromosPage from "./page";
import type { PromoCode } from "@/lib/types";

vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: () => true }));
vi.mock("@/components/admin/AffiliatorNotice", () => ({ AffiliatorNotice: () => null }));

let promos: PromoCode[] = [];
vi.mock("@/lib/hooks/affiliates", () => ({
  useAffiliatorPromos: () => ({ data: promos, isLoading: false }),
  useAffiliatorProducts: () => ({ data: [{ id: "p1", name: "Try Out UTBK" }] }),
}));

describe("AffiliatePromosPage", () => {
  it("shows the discount, conditions, validity and product scope of each promo", () => {
    promos = [
      {
        id: "1", code: "AFF10", discount_percent: 10, max_discount_amount: 50000, min_qty: 5, min_order_amount: 100000,
        used_count: 0, starts_at: "2026-11-01T00:00:00Z", expires_at: "2026-12-31T00:00:00Z", product_ids: ["p1"],
      },
      { id: "2", code: "FLAT", discount_amount: 20000, used_count: 0, product_ids: [] },
    ];
    render(<AffiliatePromosPage />);

    expect(screen.getByText("AFF10")).toBeInTheDocument();
    expect(screen.getByText(/10% \(maks\. Rp50\.000\)/)).toBeInTheDocument();
    expect(screen.getByText("Min. 5 unit · Min. belanja Rp100.000")).toBeInTheDocument();
    expect(screen.getByText("Try Out UTBK")).toBeInTheDocument();
    expect(screen.getByText("Rp20.000")).toBeInTheDocument();
    expect(screen.getByText("Tanpa batas")).toBeInTheDocument();
    expect(screen.getByText("Semua produk")).toBeInTheDocument();
  });

  it("says so when no promo is available", () => {
    promos = [];
    render(<AffiliatePromosPage />);

    expect(screen.getByText("Belum ada promo yang tersedia.")).toBeInTheDocument();
  });
});
