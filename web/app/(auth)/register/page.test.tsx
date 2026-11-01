import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import RegisterPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("@/lib/hooks/auth", () => ({
  useRegister: () => ({ mutateAsync: vi.fn(), isPending: false }),
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
});
