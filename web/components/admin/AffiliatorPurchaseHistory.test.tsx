import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { AffiliatorPurchaseHistory } from "./AffiliatorPurchaseHistory";

const usePurchases = vi.fn();
vi.mock("@/lib/hooks/affiliates", () => ({
  useAffiliatorPurchases: (p: unknown) => usePurchases(p),
  useAffiliatorMe: () => ({ data: { schools: [{ id: "s1", name: "SMAN 1" }] } }),
}));
vi.mock("@/lib/hooks/use-debounced-value", () => ({ useDebouncedValue: (v: string) => v }));
vi.mock("@/components/ui/select", async () => await import("@/__tests__/helpers/native-select"));

const row = {
  order_id: "o1", order_status: "paid", created_at: "2026-10-20T03:00:00Z", student_id: "st1", student_name: "Budi Santoso",
  student_username: "budi01", school_name: "SMAN 1", product_name: "Try Out UTBK", product_type: "exam", unit_price: 50000, promo_code: "AFF10",
};

describe("AffiliatorPurchaseHistory", () => {
  beforeEach(() => {
    usePurchases.mockReset();
    usePurchases.mockReturnValue({ data: { data: [row, { ...row, order_id: "o2", student_id: "st2", student_name: "Tono", student_username: "tono01", school_name: null, promo_code: null }], total: 2 }, isLoading: false, isError: false });
  });

  it("lists each student with school, product, price, promo and order status", () => {
    render(<AffiliatorPurchaseHistory />);

    expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    expect(screen.getByText("budi01")).toBeInTheDocument();
    expect(screen.getAllByText("Try Out UTBK")).toHaveLength(2);
    expect(screen.getAllByText("Rp50.000")).toHaveLength(2);
    expect(screen.getByText("AFF10")).toBeInTheDocument();
    expect(screen.getByText("Tanpa sekolah")).toBeInTheDocument();
    expect(screen.getAllByText("Dibayar")).toHaveLength(2);
  });

  it("sends the search text to the query", () => {
    render(<AffiliatorPurchaseHistory />);

    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "budi" } });

    expect(usePurchases).toHaveBeenLastCalledWith(expect.objectContaining({ q: "budi", offset: 0 }));
  });

  it("shows the empty message", () => {
    usePurchases.mockReturnValue({ data: { data: [], total: 0 }, isLoading: false, isError: false });
    render(<AffiliatorPurchaseHistory />);

    expect(screen.getByText("Belum ada siswa yang dibelikan.")).toBeInTheDocument();
  });
});
