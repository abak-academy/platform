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

const programStatus = vi.hoisted(() => ({ value: { enabled: true } as { enabled: boolean } | undefined }));

vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateProgramStatus: () => ({ data: programStatus.value }),
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

  it("keeps the sign-up button off until the terms are ticked, and says why", () => {
    render(<AffiliateRegisterPage />);
    fillValid();
    const button = screen.getByRole("button", { name: /daftar & verifikasi otp/i });

    expect(button).toBeDisabled();
    expect(screen.getByText("Centang persetujuan di atas untuk mengaktifkan tombol daftar.")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox"));
    expect(button).toBeEnabled();
    expect(screen.queryByText("Centang persetujuan di atas untuk mengaktifkan tombol daftar.")).toBeNull();

    fireEvent.click(screen.getByRole("checkbox"));
    expect(button).toBeDisabled();
    expect(registerMock).not.toHaveBeenCalled();
  });

  it("matches the student sign-up: password meter, no Google button", () => {
    render(<AffiliateRegisterPage />);
    fill("Kata Sandi", "abcdefghij");

    expect(screen.getByTestId("password-strength")).toBeInTheDocument();
    expect(screen.queryByText(/google/i)).toBeNull();
  });

  it("still validates the fields once the terms are ticked", async () => {
    render(<AffiliateRegisterPage />);
    fireEvent.click(screen.getByRole("checkbox"));

    fireEvent.click(screen.getByRole("button", { name: /daftar & verifikasi otp/i }));

    expect(await screen.findByText("Nomor HP wajib diisi")).toBeInTheDocument();
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

  it("shows a closed notice instead of the form while the program is off", () => {
    programStatus.value = { enabled: false };
    render(<AffiliateRegisterPage />);

    expect(screen.getByText(/pendaftaran affiliator sedang ditutup/i)).toBeInTheDocument();
    expect(screen.queryByLabelText("Nama Lengkap")).toBeNull();
    expect(screen.getByRole("link", { name: "Kembali ke halaman masuk" })).toHaveAttribute("href", "/login");
    programStatus.value = { enabled: true };
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
