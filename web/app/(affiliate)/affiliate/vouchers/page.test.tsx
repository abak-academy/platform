import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import AffiliateVouchersPage from "./page";

const hook = vi.hoisted(() => ({ state: {} as Record<string, unknown>, success: vi.fn() }));

vi.mock("sonner", () => ({ toast: { success: hook.success, error: vi.fn() } }));
vi.mock("@/lib/hooks/affiliate", () => ({ useAffiliatePromos: () => hook.state }));

const promo = {
  id: "p1",
  code: "SISWA10",
  discount_percent: 10,
  discount_amount: null,
  min_order_amount: 50000,
  max_discount_amount: 20000,
  max_uses: 5,
  used_count: 2,
  expires_at: null,
};

describe("AffiliateVouchersPage", () => {
  it("details each voucher a student can use, and copies the code", async () => {
    hook.state = { isLoading: false, isError: false, data: { data: [promo] } };
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    render(<AffiliateVouchersPage />);

    expect(screen.getByText("SISWA10")).toBeInTheDocument();
    expect(screen.getByText("10%")).toBeInTheDocument();
    expect(screen.getByText(/20\.000/)).toBeInTheDocument();
    expect(screen.getByText(/50\.000/)).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Salin kode" }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("SISWA10"));
  });

  it("says plainly when there is no active voucher", () => {
    hook.state = { isLoading: false, isError: false, data: { data: [] } };
    render(<AffiliateVouchersPage />);
    expect(screen.getByText("Belum ada promo aktif.")).toBeInTheDocument();
  });
});
