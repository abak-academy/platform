import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import AdminLayout from "./layout";

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

let authStore = {
  token: null as string | null,
  user: null as { role?: string } | null,
};

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof authStore) => unknown) => selector(authStore),
}));

let meState = {
  data: null as { role?: string } | null,
  isError: false,
  isLoading: false,
};

vi.mock("@/lib/hooks/auth", async () => {
  const actual = await vi.importActual<typeof import("@/lib/hooks/auth")>("@/lib/hooks/auth");
  return {
    ...actual,
    useMe: ({ enabled }: { enabled?: boolean }) => (enabled ? meState : { data: null, isError: false, isLoading: false }),
  };
});

describe("AdminLayout", () => {
  beforeEach(() => {
    replace.mockClear();
    authStore = { token: null, user: null };
    meState = { data: null, isError: false, isLoading: false };
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("redirects to the admin login when there is no token", async () => {
    render(<AdminLayout>protected</AdminLayout>);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/admin/login"));
    expect(replace).not.toHaveBeenCalledWith("/login");
  });

  it("renders the shell for admin_store role from the store", async () => {
    authStore = { token: "t", user: { role: "admin_store" } };
    const { getByTestId } = render(<AdminLayout>protected</AdminLayout>);
    await waitFor(() => expect(getByTestId("shell")).toBeInTheDocument());
    expect(getByTestId("shell")).toHaveAttribute("data-role", "admin_store");
  });

  it("redirects to / when the role is student", async () => {
    authStore = { token: "t", user: { role: "student" } };
    render(<AdminLayout>protected</AdminLayout>);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));
  });

  it("falls back to /auth/me when the store role is missing", async () => {
    authStore = { token: "t", user: {} };
    meState = { data: { role: "admin_exam" }, isError: false, isLoading: false };
    const { getByTestId } = render(<AdminLayout>protected</AdminLayout>);
    await waitFor(() => expect(getByTestId("shell")).toBeInTheDocument());
    expect(getByTestId("shell")).toHaveAttribute("data-role", "admin_exam");
  });

  it("redirects to the admin login when /auth/me fails", async () => {
    authStore = { token: "t", user: {} };
    meState = { data: null, isError: true, isLoading: false };
    render(<AdminLayout>protected</AdminLayout>);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/admin/login"));
    expect(replace).not.toHaveBeenCalledWith("/login");
  });
});
