import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import AdminLoginPage from "./page";

const pushMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

const mutateAsyncMock = vi.fn();

vi.mock("@/lib/hooks/auth", () => ({
  useLogin: () => ({ mutateAsync: mutateAsyncMock, isPending: false }),
}));

describe("AdminLoginPage", () => {
  beforeEach(() => {
    pushMock.mockClear();
    mutateAsyncMock.mockClear();
  });

  it("renders identifier and password fields", () => {
    render(<AdminLoginPage />);

    expect(
      screen.getByLabelText(/email atau username|email or username/i, { selector: "input" }),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText(/kata sandi|password/i, { selector: "input" }),
    ).toBeInTheDocument();
  });

  it("renders no Google sign-in element", () => {
    render(<AdminLoginPage />);
    expect(screen.queryByTestId("google-sign-in")).toBeNull();
  });

  it("renders no link or button to /register", () => {
    render(<AdminLoginPage />);
    expect(screen.queryByRole("link", { name: /register|daftar/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /register|daftar|sign up/i })).toBeNull();
  });

  it("routes via redirectForRole with the returned role on successful submit", async () => {
    mutateAsyncMock.mockResolvedValue({ user: { role: "super_admin" } });

    render(<AdminLoginPage />);

    fireEvent.change(
      screen.getByLabelText(/email atau username|email or username/i, { selector: "input" }),
      { target: { value: "admin@example.com" } },
    );
    fireEvent.change(screen.getByLabelText(/kata sandi|password/i, { selector: "input" }), {
      target: { value: "secret123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /masuk|sign in|login/i }));

    await waitFor(() => {
      expect(pushMock).toHaveBeenCalledWith("/admin");
    });
  });
});
