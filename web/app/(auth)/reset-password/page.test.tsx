import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import ResetPasswordPage from "./page";
import { ApiError } from "@/lib/api";

const mutateAsyncMock = vi.fn();
const search = vi.hoisted(() => ({ value: "token=u1-123" }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(search.value),
}));

vi.mock("@/lib/hooks/auth", () => ({
  useResetPassword: () => ({ mutateAsync: mutateAsyncMock, isPending: false }),
}));

function fill(code: string, password: string) {
  const digits = screen.getAllByRole("textbox").filter((el) => el.getAttribute("id") !== "new-password");
  code.split("").forEach((d, i) => fireEvent.change(digits[i], { target: { value: d } }));
  fireEvent.change(screen.getByLabelText("Kata sandi baru"), { target: { value: password } });
  fireEvent.click(screen.getByRole("button", { name: "Simpan kata sandi" }));
}

describe("ResetPasswordPage", () => {
  beforeEach(() => {
    mutateAsyncMock.mockReset();
    search.value = "token=u1-123";
  });

  it("sends the link token, the typed code and the new password, then shows done", async () => {
    mutateAsyncMock.mockResolvedValue({ message: "ok" });
    render(<ResetPasswordPage />);
    fill("482913", "newpassword123");

    expect(await screen.findByText("Kata sandi diperbarui")).toBeTruthy();
    expect(mutateAsyncMock).toHaveBeenCalledWith({ token: "u1-123", otp: "482913", new_password: "newpassword123" });
    expect(screen.getByRole("link", { name: "Masuk" }).getAttribute("href")).toBe("/login");
  });

  it("blocks a password shorter than 8 characters without calling the API", async () => {
    render(<ResetPasswordPage />);
    fill("482913", "short");

    await waitFor(() => expect(screen.getByRole("alert").textContent).toBe("Kata sandi minimal 8 karakter."));
    expect(mutateAsyncMock).not.toHaveBeenCalled();
  });

  it("shows the wrong-code message on reset_invalid", async () => {
    mutateAsyncMock.mockImplementation(async () => {
      throw new ApiError("reset_invalid", "invalid reset token", 422);
    });
    render(<ResetPasswordPage />);
    fill("000000", "newpassword123");

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toBe(
        "Kode salah atau tautan sudah kedaluwarsa. Coba lagi, atau minta tautan baru.",
      ),
    );
    expect(screen.queryByText("Kata sandi diperbarui")).toBeNull();
  });

  it("asks for a new link when the URL has no token", () => {
    search.value = "";
    render(<ResetPasswordPage />);

    expect(screen.getByRole("alert").textContent).toBe("Tautan reset tidak lengkap. Minta tautan baru.");
    expect(screen.getByRole("link", { name: "Minta tautan baru" }).getAttribute("href")).toBe("/forgot-password");
    expect(screen.queryByRole("button", { name: "Simpan kata sandi" })).toBeNull();
  });
});
