import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import WalletPage from "./page";
import { ledgerDescription } from "@/lib/affiliate-labels";
import { ApiError } from "@/lib/api";

const h = vi.hoisted(() => ({
  me: vi.fn(),
  payouts: vi.fn(),
  ledger: vi.fn(),
  request: vi.fn(),
  replace: vi.fn(),
  search: { value: "" },
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: h.replace }),
  usePathname: () => "/affiliate/wallet",
  useSearchParams: () => new URLSearchParams(h.search.value),
}));
vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateMe: h.me,
  useAffiliatePayouts: h.payouts,
  useAffiliateLedger: h.ledger,
  useRequestAffiliatePayout: () => ({ mutate: h.request, isPending: false }),
}));
vi.mock("sonner", () => ({ toast: h.toast }));

const profile = {
  status: "active",
  bank_name: "BCA",
  bank_account_number: "1234567890",
  bank_account_holder: "Budi",
  frozen_until: undefined as string | undefined,
};
const payoutPage = (extra: Record<string, unknown> = {}) => ({
  isLoading: false,
  isError: false,
  data: { data: [], saldo: 410000, min_payout: 100000, payout_day_of_month: 5, page: 1, per_page: 20, total: 0, ...extra },
});

describe("AffiliateWalletPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => typeof m === "function" && "mockReset" in m && m.mockReset());
    h.toast.success.mockReset();
    h.toast.error.mockReset();
    h.search.value = "";
    h.me.mockReturnValue({ data: profile });
    h.payouts.mockReturnValue(payoutPage());
    h.ledger.mockReturnValue({ isLoading: false, isError: false, data: { data: [], saldo: 410000, page: 1, per_page: 20, total: 0 } });
  });

  it("shows the balance, minimum, schedule and a masked account", () => {
    render(<WalletPage />);

    expect(screen.getByText("Rp410.000")).toBeInTheDocument();
    expect(screen.getByText("Rp100.000")).toBeInTheDocument();
    expect(screen.getByText("Jadwal pencairan: tanggal 5 tiap bulan")).toBeInTheDocument();
    expect(screen.getByText("BCA •••• 7890")).toBeInTheDocument();
    expect(screen.queryByText("1234567890")).toBeNull();
    expect(screen.getByRole("button", { name: "Ajukan penarikan" })).toBeEnabled();
  });

  it("blocks the request and points to Account when no bank account is saved", () => {
    h.me.mockReturnValue({ data: { ...profile, bank_name: null, bank_account_number: null, bank_account_holder: null } });
    render(<WalletPage />);

    expect(screen.getByRole("button", { name: "Ajukan penarikan" })).toBeDisabled();
    expect(screen.getByRole("link", { name: "Buka menu Akun" })).toHaveAttribute("href", "/affiliate/account");
    expect(screen.getByText("Belum diisi")).toBeInTheDocument();
  });

  it("blocks the request while the balance is frozen, below the minimum, or the account is not active", () => {
    h.me.mockReturnValue({ data: { ...profile, frozen_until: "2999-01-01T00:00:00Z" } });
    const { unmount } = render(<WalletPage />);
    expect(screen.getByRole("button", { name: "Ajukan penarikan" })).toBeDisabled();
    expect(screen.getByRole("note")).toHaveTextContent("Saldo sedang dibekukan sampai");
    unmount();

    h.me.mockReturnValue({ data: profile });
    h.payouts.mockReturnValue(payoutPage({ saldo: 50000 }));
    const second = render(<WalletPage />);
    expect(screen.getByRole("button", { name: "Ajukan penarikan" })).toBeDisabled();
    expect(screen.getByRole("note")).toHaveTextContent("Saldo belum mencapai minimum penarikan (Rp100.000).");
    second.unmount();

    h.me.mockReturnValue({ data: { ...profile, status: "suspended" } });
    h.payouts.mockReturnValue(payoutPage());
    render(<WalletPage />);
    expect(screen.getByRole("button", { name: "Ajukan penarikan" })).toBeDisabled();
    expect(screen.getByRole("note")).toHaveTextContent("Akunmu belum aktif");
  });

  it("requests the whole balance when the amount is empty and an explicit amount otherwise", () => {
    render(<WalletPage />);
    fireEvent.click(screen.getByRole("button", { name: "Ajukan penarikan" }));
    expect(screen.getByText("Kosongkan untuk menarik seluruh saldo (Rp410.000).")).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole("button", { name: "Ajukan penarikan" }).pop()!);
    expect(h.request).toHaveBeenLastCalledWith({}, expect.anything());

    fireEvent.change(screen.getByLabelText("Jumlah"), { target: { value: "150000" } });
    fireEvent.click(screen.getAllByRole("button", { name: "Ajukan penarikan" }).pop()!);
    expect(h.request).toHaveBeenLastCalledWith({ amount: 150000 }, expect.anything());
  });

  it("refuses a non-positive amount and translates a server rejection", () => {
    h.request.mockImplementation((_input, opts) => opts.onError(new ApiError("affiliate_payout_in_flight", "x", 409)));
    render(<WalletPage />);
    fireEvent.click(screen.getByRole("button", { name: "Ajukan penarikan" }));

    fireEvent.change(screen.getByLabelText("Jumlah"), { target: { value: "-5" } });
    fireEvent.click(screen.getAllByRole("button", { name: "Ajukan penarikan" }).pop()!);
    expect(h.request).not.toHaveBeenCalled();
    expect(h.toast.error).toHaveBeenCalledWith("Jumlah harus berupa angka lebih dari nol.");

    fireEvent.change(screen.getByLabelText("Jumlah"), { target: { value: "" } });
    fireEvent.click(screen.getAllByRole("button", { name: "Ajukan penarikan" }).pop()!);
    expect(h.toast.error).toHaveBeenCalledWith("Masih ada penarikan yang sedang diproses.");
  });

  it("starts on the balance history and switches to payouts through the URL", () => {
    render(<WalletPage />);
    expect(h.ledger).toHaveBeenCalled();

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Riwayat penarikan" }), { button: 0 });
    expect(h.replace).toHaveBeenCalledWith("/affiliate/wallet?tab=payouts", { scroll: false });
  });

  it("shows the payout history on the payouts tab", () => {
    h.search.value = "tab=payouts";
    h.payouts.mockReturnValue(
      payoutPage({
        data: [{ id: "p1", amount: 100000, status: "rejected", requested_at: "2026-10-01T00:00:00Z", processed_at: "2026-10-02T00:00:00Z", reject_reason: "Rekening tidak sesuai" }],
        total: 1,
      }),
    );
    render(<WalletPage />);

    expect(screen.getByText("Rekening tidak sesuai")).toBeInTheDocument();
    expect(screen.getByText("Ditolak")).toBeInTheDocument();
  });
});

describe("ledgerDescription", () => {
  const t = (key: string) =>
    ({
      aff_ledger_desc_commission: "Komisi dari {student} — {product}",
      aff_ledger_desc_commission_plain: "Komisi pembelian siswa",
      aff_ledger_desc_refund: "Refund {student} — {product}",
      aff_ledger_desc_refund_plain: "Refund",
      aff_ledger_desc_adjustment: "Penyesuaian dari admin",
      aff_ledger_desc_payout: "Penarikan diajukan",
      aff_ledger_desc_reversal: "Penarikan ditolak",
    })[key] ?? key;
  const entry = (extra: Record<string, unknown>) => ({ id: "l", entry_type: "commission", amount: 1, created_at: "", ...extra }) as never;

  it("reads in words and never leaks the internal note", () => {
    expect(ledgerDescription(t as never, entry({ student_name: "Sari W.", product_name: "Kursus", note: "commission order 8f3a" }))).toBe(
      "Komisi dari Sari W. — Kursus",
    );
    expect(ledgerDescription(t as never, entry({ note: "commission order 8f3a" }))).toBe("Komisi pembelian siswa");
    expect(ledgerDescription(t as never, entry({ entry_type: "refund_deduction", student_name: "Agus S.", product_name: "Ujian" }))).toBe("Refund Agus S. — Ujian");
    expect(ledgerDescription(t as never, entry({ entry_type: "payout" }))).toBe("Penarikan diajukan");
    expect(ledgerDescription(t as never, entry({ entry_type: "payout_reversal" }))).toBe("Penarikan ditolak");
  });

  it("uses the admin's reason for an adjustment, with a fallback", () => {
    expect(ledgerDescription(t as never, entry({ entry_type: "adjustment", note: "Bonus akhir tahun" }))).toBe("Bonus akhir tahun");
    expect(ledgerDescription(t as never, entry({ entry_type: "adjustment", note: null }))).toBe("Penyesuaian dari admin");
  });
});
