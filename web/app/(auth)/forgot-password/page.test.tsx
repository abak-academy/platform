import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import ForgotPasswordPage from "./page";
import { ApiError } from "@/lib/api";

const mutateAsyncMock = vi.fn();

vi.mock("@/lib/hooks/auth", () => ({
  useForgotPassword: () => ({ mutateAsync: mutateAsyncMock, isPending: false }),
}));

function submit(email: string) {
  fireEvent.change(screen.getByLabelText("Email"), { target: { value: email } });
  fireEvent.click(screen.getByRole("button", { name: "Kirim tautan reset" }));
}

describe("ForgotPasswordPage", () => {
  beforeEach(() => {
    mutateAsyncMock.mockReset();
  });

  it("sends the trimmed email and shows the check-your-email state", async () => {
    mutateAsyncMock.mockResolvedValue({ message: "ok" });
    render(<ForgotPasswordPage />);
    submit("  siswa@example.com ");

    expect(await screen.findByText("Cek email kamu")).toBeTruthy();
    expect(mutateAsyncMock).toHaveBeenCalledWith("siswa@example.com");
  });

  it.each([
    ["rate_limited", 429, "Terlalu banyak permintaan. Coba lagi sebentar lagi."],
    ["email_delivery_failed", 503, "Email gagal terkirim. Coba lagi beberapa saat lagi."],
    ["internal_error", 500, "Gagal mengirim. Coba lagi."],
  ])("maps %s to its message and stays on the form", async (code, status, message) => {
    mutateAsyncMock.mockImplementation(async () => {
      throw new ApiError(code, "x", status);
    });
    render(<ForgotPasswordPage />);
    submit("siswa@example.com");

    await waitFor(() => expect(screen.getByRole("alert").textContent).toBe(message));
    expect(screen.queryByText("Cek email kamu")).toBeNull();
  });
});
