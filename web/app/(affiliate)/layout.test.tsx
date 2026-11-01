import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import AffiliateLayout from "./layout";

const replace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

vi.mock("@/components/shell/AppShell", () => ({
  AppShell: ({ role, children }: { role: string; children: React.ReactNode }) => (
    <div data-testid="shell" data-role={role}>
      {children}
    </div>
  ),
}));

const meState = vi.hoisted(() => ({ value: { program_enabled: true } as { program_enabled: boolean } | undefined }));
let token: string | null = null;
let roleState = { role: undefined as string | undefined, hydrated: true, meIsError: false };

vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateMe: () => ({ data: meState.value }),
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: { token: string | null }) => unknown) => selector({ token }),
}));

vi.mock("@/lib/hooks/use-capability", () => ({
  useResolvedRole: () => roleState,
}));

describe("AffiliateLayout", () => {
  beforeEach(() => {
    replace.mockClear();
    token = null;
    roleState = { role: undefined, hydrated: true, meIsError: false };
  });

  it("redirects to /login without a token", async () => {
    render(<AffiliateLayout>portal</AffiliateLayout>);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
  });

  it("renders the shell for an affiliator", async () => {
    token = "t";
    roleState = { role: "affiliator", hydrated: true, meIsError: false };
    const { getByTestId } = render(<AffiliateLayout>portal</AffiliateLayout>);
    await waitFor(() => expect(getByTestId("shell")).toHaveAttribute("data-role", "affiliator"));
  });

  it("tells the affiliator on every menu that the program is off, and only then", async () => {
    token = "t";
    roleState = { role: "affiliator", hydrated: true, meIsError: false };
    meState.value = { program_enabled: false };
    const off = render(<AffiliateLayout>portal</AffiliateLayout>);
    await waitFor(() => expect(off.getByRole("status")).toHaveTextContent(/sedang dinonaktifkan/i));
    off.unmount();

    meState.value = { program_enabled: true };
    const on = render(<AffiliateLayout>portal</AffiliateLayout>);
    await waitFor(() => expect(on.getByTestId("shell")).toBeInTheDocument());
    expect(on.queryByRole("status")).toBeNull();
  });

  it("sends a student back to their own home and renders nothing of the portal", async () => {
    token = "t";
    roleState = { role: "student", hydrated: true, meIsError: false };
    const { queryByTestId } = render(<AffiliateLayout>portal</AffiliateLayout>);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));
    expect(queryByTestId("shell")).not.toBeInTheDocument();
  });

  it("sends a super_admin to the admin home", async () => {
    token = "t";
    roleState = { role: "super_admin", hydrated: true, meIsError: false };
    render(<AffiliateLayout>portal</AffiliateLayout>);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/admin"));
  });
});
