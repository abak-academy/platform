import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import OtpPage from "./page";
import { ApiError } from "@/lib/api";

const pushMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => ({ get: (key: string) => (key === "id" ? "budi@example.com" : null) }),
}));

const verifyMock = vi.fn();
const sendMock = vi.fn();

vi.mock("@/lib/hooks/auth", () => ({
  useVerifyOtp: () => ({ mutateAsync: verifyMock, isPending: false }),
  useSendOtp: () => ({ mutateAsync: sendMock, isPending: false }),
}));

function seedSession(flags: Record<string, string>) {
  for (const [key, value] of Object.entries(flags)) {
    sessionStorage.setItem(key, value);
  }
}

function fillCode(code: string) {
  const inputs = screen.getAllByRole("textbox");
  code.split("").forEach((digit, i) => {
    fireEvent.change(inputs[i], { target: { value: digit } });
  });
}

describe("OtpPage", () => {
  beforeEach(() => {
    pushMock.mockClear();
    verifyMock.mockClear();
    sendMock.mockClear();
    sessionStorage.clear();
  });

  it("shows the email-failed banner and the server's resend budget instead of the resend button", () => {
    seedSession({ "abak-email-failed": "1", "abak-resend-after": "45" });
    render(<OtpPage />);

    expect(screen.getByRole("status")).toHaveTextContent(/gagal dikirim ke email/i);
    expect(screen.getByText(/Kirim ulang tersedia dalam 45 detik/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /kirim ulang kode/i })).toBeNull();
  });

  it("shows the live-code banner and offers an immediate resend when no send was made", () => {
    seedSession({ "abak-otp-active": "1", "abak-resend-after": "0" });
    render(<OtpPage />);

    expect(screen.getByRole("status")).toHaveTextContent(/masih berlaku/i);
    const resend = screen.getByRole("button", { name: /kirim ulang kode/i });
    fireEvent.click(resend);
    // No pending token in storage — the endpoint accepts an email-only resend.
    expect(sendMock).toHaveBeenCalledWith({ email: "budi@example.com" });
  });

  it("maps a failed resend to friendly copy and adopts the server's Retry-After", async () => {
    seedSession({ "abak-resend-after": "0" });
    sendMock.mockRejectedValue(
      new ApiError("email_delivery_failed", "email delivery failed: 451 relay said no", 503, undefined, 300),
    );
    render(<OtpPage />);

    fireEvent.click(screen.getByRole("button", { name: /kirim ulang kode/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/Coba lagi beberapa saat/i);
    });
    expect(screen.queryByText("email delivery failed: 451 relay said no")).not.toBeInTheDocument();
    expect(screen.getByText(/Kirim ulang tersedia dalam 300 detik/)).toBeInTheDocument();
  });

  it("clears the stored challenge state and navigates after a successful verify", async () => {
    seedSession({ "abak-pending-token": "tok-abc", "abak-email-failed": "1" });
    verifyMock.mockResolvedValue({ user: { role: "student" } });
    render(<OtpPage />);

    fillCode("123456");
    fireEvent.click(screen.getByRole("button", { name: /verifikasi/i }));

    await waitFor(() => {
      expect(pushMock).toHaveBeenCalledWith("/");
    });
    expect(verifyMock).toHaveBeenCalledWith(
      expect.objectContaining({ identifier: "budi@example.com", code: "123456", pending_token: "tok-abc" }),
    );
    expect(sessionStorage.getItem("abak-pending-token")).toBeNull();
    expect(sessionStorage.getItem("abak-email-failed")).toBeNull();
  });
});
