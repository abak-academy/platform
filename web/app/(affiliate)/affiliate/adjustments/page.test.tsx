import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AdjustmentsPage from "./page";

const h = vi.hoisted(() => ({
  adjustments: vi.fn(),
  search: { value: "" },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  usePathname: () => "/affiliate/adjustments",
  useSearchParams: () => new URLSearchParams(h.search.value),
}));
vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateAdjustments: h.adjustments,
}));

const page = (data: unknown[]) => ({ isLoading: false, isError: false, data: { data, page: 1, per_page: 20, total: data.length } });

describe("AffiliateAdjustmentsPage", () => {
  beforeEach(() => {
    h.search.value = "";
    h.adjustments.mockReturnValue(
      page([
        { id: "a1", category: "bonus", amount: 50000, reason: "Bonus kampanye", applied: true, created_at: "2026-10-02T00:00:00Z" },
        { id: "a2", category: "correction", amount: -20000, reason: "Koreksi", applied: false, created_at: "2026-10-03T00:00:00Z" },
      ]),
    );
  });

  it("explains what an adjustment is and lists each one with whether it has taken effect", () => {
    render(<AdjustmentsPage />);

    expect(screen.getByText(/Penyesuaian adalah penambahan atau pengurangan saldo/)).toBeInTheDocument();
    expect(screen.getByText("Bonus kampanye")).toBeInTheDocument();
    expect(screen.getByText("Berlaku")).toBeInTheDocument();
    expect(screen.getByText("Menunggu persetujuan")).toBeInTheDocument();
  });

  it("passes the active filters to the adjustments query", () => {
    h.search.value = "q=bonus&type=bonus&page=2";
    render(<AdjustmentsPage />);

    expect(h.adjustments).toHaveBeenCalledWith({ page: 2, per_page: 20, q: "bonus", type: "bonus", from: "", to: "" });
  });
});
