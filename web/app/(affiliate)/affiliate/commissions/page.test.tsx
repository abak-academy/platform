import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import CommissionsPage from "./page";

const h = vi.hoisted(() => ({ commissions: vi.fn(), search: { value: "" } }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  usePathname: () => "/affiliate/commissions",
  useSearchParams: () => new URLSearchParams(h.search.value),
}));
vi.mock("@/lib/hooks/affiliate", () => ({ useAffiliateCommissions: h.commissions }));

const row = {
  id: "c1",
  student_name: "Sari W.",
  product_name: "Kursus Matematika Dasar",
  product_type: "course",
  base_amount: 2000000,
  percentage: 10,
  amount: 200000,
  status: "available",
  wait_until: "2026-09-23T00:00:00Z",
  paid_at: "2026-09-16T00:00:00Z",
  created_at: "2026-09-16T00:00:00Z",
};
const ok = (data: unknown[]) => ({ isLoading: false, isError: false, data: { data, page: 1, per_page: 20, total: data.length } });

describe("AffiliateCommissionsPage", () => {
  beforeEach(() => {
    h.commissions.mockReset();
    h.search.value = "";
  });

  it("shows which student and product each commission came from", () => {
    h.commissions.mockReturnValue(ok([row]));
    render(<CommissionsPage />);

    expect(screen.getByText("Sari W.")).toBeInTheDocument();
    expect(screen.getByText("Kursus Matematika Dasar")).toBeInTheDocument();
    expect(screen.getByText("Rp200.000")).toBeInTheDocument();
    expect(screen.getByText("Tersedia")).toBeInTheDocument();
  });

  it("explains a commission that waits for the item to arrive", () => {
    h.commissions.mockReturnValue(ok([{ ...row, status: "pending", wait_until: null, product_type: "book" }]));
    render(<CommissionsPage />);

    expect(screen.getByText("Setelah barang diterima")).toBeInTheDocument();
  });

  it("sends the category filter to the API as product_type", () => {
    h.search.value = "q=sari&status=pending&type=book&to=2026-10-31";
    h.commissions.mockReturnValue(ok([]));
    render(<CommissionsPage />);

    expect(h.commissions).toHaveBeenCalledWith({
      page: 1,
      per_page: 20,
      q: "sari",
      status: "pending",
      product_type: "book",
      from: "",
      to: "2026-10-31",
    });
  });
});
