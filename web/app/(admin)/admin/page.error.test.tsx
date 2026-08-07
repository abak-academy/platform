import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import AdminIndexPage from "./page";

// This test deliberately does NOT mock useAdminDashboard. The suite in
// page.test.tsx mocks the hook to return isError, which proves the error
// BRANCH renders but never proves isError is reachable. This drives the real
// hook through a rejecting fetch, which is the condition a real outage
// produces, so a regression in the wiring between hook and page is caught.

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace, push: vi.fn() }) }));

const authStore = { token: "t", user: { role: "super_admin", name: "Admin" } };
vi.mock("@/stores/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (s: typeof authStore) => unknown) => selector(authStore),
    { getState: () => ({ ...authStore, refreshToken: "r", clear: () => {} }) }
  ),
}));

vi.mock("@/lib/hooks/auth", () => ({
  useMe: () => ({ data: { role: "super_admin", name: "Admin" }, isError: false, isLoading: false }),
}));

vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: () => true }));

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: 0, staleTime: 0 } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("AdminIndexPage — real fetch failure (visible tab)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi
      .fn()
      .mockRejectedValue(new TypeError("Failed to fetch")) as unknown as typeof fetch;
  });

  it("shows the failure message and never renders fabricated zeros", async () => {
    render(<AdminIndexPage />, { wrapper });

    // Match the dashboard's exact string, not /gagal memuat/i — the audit
    // panel's failure copy ("Gagal memuat log aktivitas. Coba lagi nanti.")
    // also matches that regex, and once both queries have rejected getByText
    // throws on the ambiguity. Whether both had settled by this point was
    // timing-dependent, so the loose matcher passed locally and failed in CI.
    await waitFor(() => {
      expect(screen.getByText("Gagal memuat dashboard.")).toBeInTheDocument();
    });

    // The zeroed fallback must not reach the screen alongside or instead of it.
    expect(screen.queryByTestId("kpi-row")).toBeNull();
    expect(screen.queryByText("Rp0")).toBeNull();
  });
});
