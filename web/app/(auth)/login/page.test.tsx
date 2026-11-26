import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import LoginPage from "./page";
import { ApiError } from "@/lib/api";

const pushMock = vi.fn();
const search = vi.hoisted(() => ({ value: "" }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => new URLSearchParams(search.value),
}));

const mutateAsyncMock = vi.fn();

vi.mock("@/lib/hooks/auth", () => ({
  useLogin: () => ({ mutateAsync: mutateAsyncMock, isPending: false }),
  useGoogleLogin: () => ({ mutate: vi.fn(), isPending: false }),
}));

const programStatus = vi.hoisted(() => ({ value: { enabled: true } as { enabled: boolean } | undefined }));

vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateProgramStatus: () => ({ data: programStatus.value }),
}));

vi.mock("@/components/auth/GoogleSignInButton", () => ({
  GoogleSignInButton: ({ text, next }: { text: string; next?: string | null }) => (
    <div data-testid="google-sign-in" data-text={text} data-next={next ?? ""} />
  ),
  googleSignInAvailable: true,
}));

describe("LoginPage", () => {
  beforeEach(() => {
    pushMock.mockClear();
    mutateAsyncMock.mockClear();
    sessionStorage.clear();
    search.value = "";
  });

  it("links to the affiliate sign-up under the student sign-up link", () => {
    render(<LoginPage />);

    const student = screen.getByRole("button", { name: "Daftar sekarang" });
    const affiliate = screen.getByRole("button", { name: "Daftar sebagai affiliator" });
    expect(student.compareDocumentPosition(affiliate)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);

    fireEvent.click(affiliate);
    expect(pushMock).toHaveBeenCalledWith("/affiliate-register");
  });

  it("hides the affiliate sign-up link while the program is off or unknown", () => {
    programStatus.value = { enabled: false };
    const { rerender } = render(<LoginPage />);
    expect(screen.queryByRole("button", { name: "Daftar sebagai affiliator" })).toBeNull();

    programStatus.value = undefined;
    rerender(<LoginPage />);
    expect(screen.queryByRole("button", { name: "Daftar sebagai affiliator" })).toBeNull();
    programStatus.value = { enabled: true };
  });

  // Google moved below the credential form (client, 2026-08-03): most sign-ins
  // are email/password, and leading with Google pushed the form down.
  it("renders the Google sign-in button below the password form", () => {
    const { container } = render(<LoginPage />);
    const form = container.querySelector("form");

    expect(screen.getByTestId("google-sign-in")).toHaveAttribute("data-text", "signin_with");
    expect(screen.getByTestId("google-sign-in").compareDocumentPosition(form!))
      .toBe(Node.DOCUMENT_POSITION_PRECEDING);
  });

  it("hides the or-divider along with the button when Google sign-in is unconfigured", async () => {
    vi.resetModules();
    vi.doMock("@/components/auth/GoogleSignInButton", () => ({
      GoogleSignInButton: () => null,
      googleSignInAvailable: false,
    }));
    const { default: PageWithoutGoogle } = await import("./page");

    render(<PageWithoutGoogle />);

    expect(screen.queryByTestId("google-sign-in")).toBeNull();
    expect(screen.queryByText("atau")).toBeNull();
    vi.doUnmock("@/components/auth/GoogleSignInButton");
  });

  it("stores the pending token and navigates to /otp on verification_pending", async () => {
    mutateAsyncMock.mockRejectedValue(
      new ApiError("verification_pending", "verify your email", 403, {
        pending_token: "tok-abc",
      }),
    );

    render(<LoginPage />);

    fireEvent.change(
      screen.getByLabelText(/email atau username|email or username/i, { selector: "input" }),
      { target: { value: "budi@example.com" } },
    );
    fireEvent.change(screen.getByLabelText(/kata sandi|password/i, { selector: "input" }), {
      target: { value: "secret123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /masuk|sign in|login/i }));

    await waitFor(() => {
      expect(pushMock).toHaveBeenCalledWith("/otp?id=budi%40example.com");
    });
    expect(sessionStorage.getItem("abak-pending-token")).toBe("tok-abc");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders the localized rate-limited message instead of the raw backend message", async () => {
    mutateAsyncMock.mockRejectedValue(
      new ApiError("rate_limited", "too many login attempts", 429),
    );

    render(<LoginPage />);

    fireEvent.change(
      screen.getByLabelText(/email atau username|email or username/i, { selector: "input" }),
      { target: { value: "budi@example.com" } },
    );
    fireEvent.change(screen.getByLabelText(/kata sandi|password/i, { selector: "input" }), {
      target: { value: "secret123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /masuk|sign in|login/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Terlalu banyak percobaan masuk. Coba lagi sebentar lagi.",
    );
    expect(screen.queryByText("too many login attempts")).not.toBeInTheDocument();
  });

  async function signIn(role: string) {
    mutateAsyncMock.mockResolvedValue({ user: { role } });
    render(<LoginPage />);
    fireEvent.change(
      screen.getByLabelText(/email atau username|email or username/i, { selector: "input" }),
      { target: { value: "budi@example.com" } },
    );
    fireEvent.change(screen.getByLabelText(/kata sandi|password/i, { selector: "input" }), {
      target: { value: "secret123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /masuk|sign in|login/i }));
    await waitFor(() => expect(pushMock).toHaveBeenCalled());
  }

  it.each([
    ["/cart", "student", "/cart"],
    ["https://evil.example", "student", "/"],
    ["//evil.example", "student", "/"],
    ["/cart", "super_admin", "/admin"],
  ])("next=%s as %s lands on %s", async (next, role, landing) => {
    search.value = `next=${encodeURIComponent(next)}`;
    await signIn(role);
    expect(pushMock).toHaveBeenCalledWith(landing);
  });

  it("keeps next on the register link and the OTP hop", async () => {
    search.value = "next=%2Fcart";
    render(<LoginPage />);
    fireEvent.click(screen.getByRole("button", { name: "Daftar sekarang" }));
    expect(pushMock).toHaveBeenCalledWith("/register?next=%2Fcart");
  });

  it("carries next to /otp on verification_pending", async () => {
    search.value = "next=%2Fcart";
    mutateAsyncMock.mockRejectedValue(
      new ApiError("verification_pending", "verify", 403, { pending_token: "tok" }),
    );
    render(<LoginPage />);
    fireEvent.change(
      screen.getByLabelText(/email atau username|email or username/i, { selector: "input" }),
      { target: { value: "budi@example.com" } },
    );
    fireEvent.change(screen.getByLabelText(/kata sandi|password/i, { selector: "input" }), {
      target: { value: "secret123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /masuk|sign in|login/i }));
    await waitFor(() =>
      expect(pushMock).toHaveBeenCalledWith("/otp?id=budi%40example.com&next=%2Fcart"),
    );
  });

  it("hands next to the Google button", () => {
    search.value = "next=%2Fcart";
    render(<LoginPage />);
    expect(screen.getByTestId("google-sign-in")).toHaveAttribute("data-next", "/cart");
  });
});
