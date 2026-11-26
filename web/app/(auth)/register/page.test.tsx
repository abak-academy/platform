import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import RegisterPage from "./page";

const pushMock = vi.fn();
const registerMock = vi.fn();
const search = vi.hoisted(() => ({ value: "" }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => new URLSearchParams(search.value),
}));

vi.mock("@/lib/hooks/auth", () => ({
  useRegister: () => ({ mutateAsync: registerMock, isPending: false }),
}));

vi.mock("@/lib/hooks/affiliate", () => ({
  useValidateAffiliateCode: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@/components/auth/GoogleSignInButton", () => ({
  GoogleSignInButton: ({ text }: { text: string }) => (
    <div data-testid="google-sign-in" data-text={text} />
  ),
}));

describe("RegisterPage", () => {
  it("prefills the referral field from the aff_ref cookie", async () => {
    document.cookie = "aff_ref=BUDI10; path=/";
    render(<RegisterPage />);

    expect(await screen.findByLabelText("Kode referral (opsional)")).toHaveValue("BUDI10");
    document.cookie = "aff_ref=; path=/; max-age=0";
  });

  it("renders the Google sign-up button above the registration form", () => {
    const { container } = render(<RegisterPage />);
    const form = container.querySelector("form");

    expect(screen.getByTestId("google-sign-in")).toHaveAttribute("data-text", "signup_with");
    expect(screen.getByTestId("google-sign-in").compareDocumentPosition(form!))
      .toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it("keeps next on the sign-in link", () => {
    search.value = "next=%2Fcart";
    render(<RegisterPage />);
    fireEvent.click(screen.getByRole("button", { name: "Masuk" }));
    expect(pushMock).toHaveBeenCalledWith("/login?next=%2Fcart");
    search.value = "";
  });

  it("carries next to /otp after registering", async () => {
    search.value = "next=%2Fcart";
    registerMock.mockResolvedValue({ otp_required: true, pending_token: "tok" });
    render(<RegisterPage />);
    fireEvent.change(screen.getByLabelText(/nama/i, { selector: "input" }), { target: { value: "Budi" } });
    fireEvent.change(screen.getByLabelText(/^email/i, { selector: "input" }), { target: { value: "budi@example.com" } });
    fireEvent.change(screen.getByLabelText(/^kata sandi/i, { selector: "input" }), { target: { value: "Secret123!" } });
    fireEvent.change(screen.getByLabelText(/konfirmasi/i, { selector: "input" }), { target: { value: "Secret123!" } });
    fireEvent.click(screen.getByRole("button", { name: /daftar/i }));
    await waitFor(() =>
      expect(pushMock).toHaveBeenCalledWith("/otp?id=budi%40example.com&next=%2Fcart"),
    );
    search.value = "";
  });
});
