import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AffiliateRegisterPage from "./page";
import { ApiError } from "@/lib/api";
import { AFFILIATE_TERMS_VERSION } from "@/lib/affiliate-terms";

const push = vi.fn();
const registerMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

vi.mock("@/lib/hooks/auth", () => ({
  useRegisterAffiliator: () => ({ mutateAsync: registerMock, isPending: false }),
}));

const toastError = vi.hoisted(() => vi.fn());

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: toastError, warning: vi.fn() },
}));

function fill(label: string, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
}

function fillValid() {
  fill("Nama Lengkap", "Budi Santoso");
  fill("Email", "budi@example.com");
  fill("Nomor HP", "081200000000");
  fill("Kata Sandi", "password123");
  fill("Konfirmasi Kata Sandi", "password123");
}

describe("AffiliateRegisterPage", () => {
  beforeEach(() => {
    push.mockClear();
    registerMock.mockReset();
    sessionStorage.clear();
  });

  it("blocks submit until the terms are accepted", async () => {
    render(<AffiliateRegisterPage />);
    fillValid();

    fireEvent.click(screen.getByRole("button", { name: /daftar & verifikasi otp/i }));

    expect(await screen.findByText("Kamu harus menyetujui Ketentuan Program")).toBeInTheDocument();
    expect(registerMock).not.toHaveBeenCalled();
  });

  it("registers with the terms version and hands the challenge to the OTP page", async () => {
    registerMock.mockResolvedValue({ otp_required: true, pending_token: "tok", resend_after: 60 });
    render(<AffiliateRegisterPage />);
    fillValid();
    fireEvent.click(screen.getByRole("checkbox"));

    fireEvent.click(screen.getByRole("button", { name: /daftar & verifikasi otp/i }));

    await waitFor(() => expect(push).toHaveBeenCalledWith("/otp?id=budi%40example.com"));
    expect(registerMock).toHaveBeenCalledWith({
      name: "Budi Santoso",
      email: "budi@example.com",
      phone: "081200000000",
      password: "password123",
      terms_version: AFFILIATE_TERMS_VERSION,
    });
    expect(sessionStorage.getItem("abak-pending-token")).toBe("tok");
  });

  it("stays on the form when registration is closed", async () => {
    registerMock.mockRejectedValue(new ApiError("affiliate_registration_closed", "closed", 403));
    render(<AffiliateRegisterPage />);
    fillValid();
    fireEvent.click(screen.getByRole("checkbox"));

    fireEvent.click(screen.getByRole("button", { name: /daftar & verifikasi otp/i }));

    await waitFor(() => expect(registerMock).toHaveBeenCalled());
    expect(push).not.toHaveBeenCalled();
  });

  it("explains a full daily quota", async () => {
    registerMock.mockRejectedValue(new ApiError("affiliate_registration_cap", "limit", 429));
    render(<AffiliateRegisterPage />);
    fillValid();
    fireEvent.click(screen.getByRole("checkbox"));

    fireEvent.click(screen.getByRole("button", { name: /daftar & verifikasi otp/i }));

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith("Kuota pendaftaran affiliator hari ini sudah penuh. Coba lagi besok."),
    );
    expect(push).not.toHaveBeenCalled();
  });
});
