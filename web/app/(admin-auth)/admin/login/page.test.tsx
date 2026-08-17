import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import AdminLoginPage from "./page";
import { ApiError } from "@/lib/api";

const pushMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

const mutateAsyncMock = vi.fn();

vi.mock("@/lib/hooks/auth", () => ({
  useLogin: () => ({ mutateAsync: mutateAsyncMock, isPending: false }),
}));

// Mocked so the absence assertion below can actually fail. Without this the
// query matches nothing whether or not the real button renders.
vi.mock("@/components/auth/GoogleSignInButton", () => ({
  GoogleSignInButton: () => <div data-testid="google-sign-in" />,
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
    expect(screen.queryByText(/google/i)).toBeNull();
  });

  it("renders no link or button to /register", () => {
    render(<AdminLoginPage />);
    expect(screen.queryByRole("link", { name: /register|daftar sekarang|sign up/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /register|daftar sekarang|sign up/i })).toBeNull();
  });

  it("uses no placeholder text — labels carry the field names", () => {
    render(<AdminLoginPage />);
    for (const input of Array.from(document.querySelectorAll("input"))) {
      expect(input.getAttribute("placeholder")).toBeNull();
    }
  });

  it("offers a way back to the student door", () => {
    render(<AdminLoginPage />);
    const link = screen.getByRole("link", { name: /masuk sebagai siswa|sign in as a student/i });
    expect(link).toHaveAttribute("href", "/login");
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
    fireEvent.click(screen.getByRole("button", { name: /^masuk$|sign in|login/i }));

    await waitFor(() => {
      expect(pushMock).toHaveBeenCalledWith("/admin");
    });
  });

  it("renders the localized rate-limited message instead of the raw backend message", async () => {
    mutateAsyncMock.mockRejectedValue(
      new ApiError("rate_limited", "too many login attempts", 429),
    );

    render(<AdminLoginPage />);

    fireEvent.change(
      screen.getByLabelText(/email atau username|email or username/i, { selector: "input" }),
      { target: { value: "admin@example.com" } },
    );
    fireEvent.change(screen.getByLabelText(/kata sandi|password/i, { selector: "input" }), {
      target: { value: "secret123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^masuk$|sign in|login/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Terlalu banyak percobaan masuk. Coba lagi sebentar lagi.",
    );
    expect(screen.queryByText("too many login attempts")).not.toBeInTheDocument();
  });
});
