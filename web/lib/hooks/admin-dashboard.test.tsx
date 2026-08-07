import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { useAdminDashboard } from "./admin-dashboard";

const authFetch = vi.fn();
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return { ...actual, authFetch: (...args: unknown[]) => authFetch(...args) };
});

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

beforeEach(() => authFetch.mockReset());

describe("useAdminDashboard", () => {
  it("requests the bare endpoint when the range is empty", async () => {
    authFetch.mockResolvedValue({ series: [] });
    renderHook(() => useAdminDashboard({}), { wrapper });

    await waitFor(() => expect(authFetch).toHaveBeenCalled());
    expect(authFetch).toHaveBeenCalledWith("/admin/dashboard");
  });

  it("appends from and to when present", async () => {
    authFetch.mockResolvedValue({ series: [] });
    renderHook(() => useAdminDashboard({ from: "2026-07-01", to: "2026-07-31" }), { wrapper });

    await waitFor(() => expect(authFetch).toHaveBeenCalled());
    expect(authFetch).toHaveBeenCalledWith("/admin/dashboard?from=2026-07-01&to=2026-07-31");
  });

  it("refetches when the range changes", async () => {
    authFetch.mockResolvedValue({ series: [] });
    const { rerender } = renderHook(({ r }) => useAdminDashboard(r), {
      wrapper,
      initialProps: { r: {} as { from?: string; to?: string } },
    });

    await waitFor(() => expect(authFetch).toHaveBeenCalledTimes(1));
    rerender({ r: { from: "2026-07-01" } });
    await waitFor(() => expect(authFetch).toHaveBeenCalledTimes(2));
  });
});
