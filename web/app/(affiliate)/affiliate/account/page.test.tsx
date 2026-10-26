import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AffiliateAccountPage from "./page";

const h = vi.hoisted(() => ({
  me: vi.fn(),
  profile: vi.fn(),
  save: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/lib/hooks/auth", () => ({ useMe: h.me }));
vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateMe: h.profile,
  useUpdateAffiliateBank: () => ({ mutate: h.save, isPending: false }),
}));
vi.mock("@/components/profile/ChangePasswordForm", () => ({ ChangePasswordForm: () => <div>password-form</div> }));
vi.mock("sonner", () => ({ toast: h.toast }));

const profile = (over: Record<string, unknown> = {}) => ({
  data: {
    code: "GLDSEED2",
    level: "gold",
    status: "active",
    bank_name: "BCA",
    bank_account_number: "1234567890",
    bank_account_holder: "Gold Lengkap",
    terms_version: "v1",
    terms_accepted_at: "2026-09-01T00:00:00Z",
    ...over,
  },
});

describe("AffiliateAccountPage", () => {
  beforeEach(() => {
    h.save.mockReset();
    h.toast.error.mockReset();
    h.me.mockReturnValue({ data: { name: "Gold Lengkap", email: "gold@seed.local", phone: "0812", created_at: "2026-09-01T00:00:00Z" } });
    h.profile.mockReturnValue(profile());
  });

  it("shows the account details and the saved bank account", () => {
    render(<AffiliateAccountPage />);
    expect(screen.getByText("gold@seed.local")).toBeInTheDocument();
    expect(screen.getByText("GLDSEED2")).toBeInTheDocument();
    expect(screen.getByLabelText("Nomor rekening")).toHaveValue("1234567890");
    expect(screen.getByText("password-form")).toBeInTheDocument();
  });

  it("saves a trimmed bank account", () => {
    render(<AffiliateAccountPage />);
    fireEvent.change(screen.getByLabelText("Nama bank"), { target: { value: " Mandiri " } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan rekening" }));
    expect(h.save).toHaveBeenCalledWith(
      { bank_name: "Mandiri", bank_account_number: "1234567890", bank_account_holder: "Gold Lengkap" },
      expect.any(Object),
    );
  });

  it("refuses an incomplete bank account", () => {
    render(<AffiliateAccountPage />);
    fireEvent.change(screen.getByLabelText("Nomor rekening"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan rekening" }));
    expect(h.save).not.toHaveBeenCalled();
    expect(h.toast.error).toHaveBeenCalled();
  });

  it("locks the bank form while the account is not active", () => {
    h.profile.mockReturnValue(profile({ status: "suspended" }));
    render(<AffiliateAccountPage />);
    expect(screen.getByLabelText("Nama bank")).toBeDisabled();
    expect(screen.getByRole("button", { name: "Simpan rekening" })).toBeDisabled();
    expect(screen.getByText(/hanya bisa diubah saat akunmu aktif/)).toBeInTheDocument();
  });
});
