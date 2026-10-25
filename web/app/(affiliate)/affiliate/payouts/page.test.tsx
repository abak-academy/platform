import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import PayoutsPage from "./page";
import { ApiError } from "@/lib/api";

const h = vi.hoisted(() => ({
  me: vi.fn(),
  payouts: vi.fn(),
  saveBank: vi.fn(),
  request: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateMe: h.me,
  useAffiliatePayouts: h.payouts,
  useUpdateAffiliateBank: () => ({ mutate: h.saveBank, isPending: false }),
  useRequestAffiliatePayout: () => ({ mutate: h.request, isPending: false }),
}));

vi.mock("sonner", () => ({ toast: h.toast }));

describe("AffiliatePayoutsPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => "mockReset" in m && m.mockReset());
    h.toast.success.mockReset();
    h.toast.error.mockReset();
    h.me.mockReturnValue({
      data: { bank_name: "BCA", bank_account_number: "123", bank_account_holder: "Budi" },
    });
    h.payouts.mockReturnValue({
      isLoading: false,
      isError: false,
      data: {
        data: [],
        saldo: 250000,
        min_payout: 100000,
        payout_day_of_month: 5,
        page: 1,
        per_page: 20,
        total: 0,
      },
    });
  });

  it("prefills the bank form and shows the balance, minimum and schedule", async () => {
    render(<PayoutsPage />);

    expect(await screen.findByDisplayValue("BCA")).toBeInTheDocument();
    expect(screen.getByText(/Saldo Rp250\.000 · Minimum penarikan Rp100\.000/)).toBeInTheDocument();
    expect(screen.getByText(/tanggal 5 setiap bulan/)).toBeInTheDocument();
  });

  it("refuses to save an incomplete bank form", async () => {
    render(<PayoutsPage />);
    fireEvent.change(await screen.findByLabelText("Nomor rekening"), { target: { value: "" } });

    fireEvent.click(screen.getByRole("button", { name: "Simpan rekening" }));

    expect(h.saveBank).not.toHaveBeenCalled();
    expect(h.toast.error).toHaveBeenCalledWith("Semua data rekening wajib diisi");
  });

  it("requests the full balance when the amount is empty", () => {
    render(<PayoutsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Ajukan penarikan" }));

    expect(h.request).toHaveBeenCalledWith({}, expect.anything());
  });

  it("sends an explicit amount", () => {
    render(<PayoutsPage />);
    fireEvent.change(screen.getByLabelText(/Jumlah/), { target: { value: "150000" } });

    fireEvent.click(screen.getByRole("button", { name: "Ajukan penarikan" }));

    expect(h.request).toHaveBeenCalledWith({ amount: 150000 }, expect.anything());
  });

  it("does not send a non-positive amount", () => {
    render(<PayoutsPage />);
    fireEvent.change(screen.getByLabelText(/Jumlah/), { target: { value: "-5" } });

    fireEvent.click(screen.getByRole("button", { name: "Ajukan penarikan" }));

    expect(h.request).not.toHaveBeenCalled();
  });

  it("translates the frozen-balance rejection", async () => {
    h.request.mockImplementation((_input, opts) =>
      opts.onError(new ApiError("affiliate_frozen", "frozen", 409)),
    );
    render(<PayoutsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Ajukan penarikan" }));

    await waitFor(() => expect(h.toast.error).toHaveBeenCalledWith("Saldo sedang dibekukan."));
  });
});
