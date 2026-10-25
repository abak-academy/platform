import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import FlagsPage from "./page";

const h = vi.hoisted(() => ({ list: vi.fn(), review: vi.fn(), capability: vi.fn() }));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useAdminAffiliateFlags: h.list,
  useReviewAffiliateFlag: () => ({ mutate: h.review, isPending: false }),
}));
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: h.capability }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const flag = (id: string, status: string) => ({
  id,
  rule: "self_purchase",
  payload: { order_id: "o1" },
  status,
  created_at: "2026-10-02T00:00:00Z",
  affiliator_name: "Budi Santoso",
  affiliator_code: "BUDI1234",
});

describe("AdminAffiliateFlagsPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => m.mockReset());
    h.capability.mockReturnValue(true);
    h.list.mockReturnValue({ isLoading: false, isError: false, data: { data: [flag("f1", "open"), flag("f2", "valid")] } });
  });

  it("defaults to open flags and shows the rule in words", () => {
    render(<FlagsPage />);

    expect(h.list).toHaveBeenCalledWith("open");
    expect(screen.getAllByText("Pembelian diri sendiri")).toHaveLength(2);
  });

  it("reviews only open flags", () => {
    render(<FlagsPage />);

    expect(screen.getAllByRole("button", { name: "Tandai valid" })).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: "Abaikan" }));
    expect(h.review).toHaveBeenCalledWith({ id: "f1", status: "ignored" }, expect.anything());
  });

  it("is blocked without the capability", () => {
    h.capability.mockReturnValue(false);
    render(<FlagsPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });
});
