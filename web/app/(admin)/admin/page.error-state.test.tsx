import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import AdminIndexPage from "./page";

// The page renders the real chart components, and their usePrefersReducedMotion
// hook calls window.matchMedia, which jsdom does not implement. Mirrors the
// stub in page.test.tsx / components/shell/AppShell.test.tsx.
beforeAll(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  });
});

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: { token: string; user: { role: string; name: string } }) => unknown) =>
    selector({ token: "t", user: { role: "super_admin", name: "Super Admin" } }),
}));

vi.mock("@/lib/hooks/use-capability", () => ({
  useHasCapability: () => true,
}));

// TanStack Query's retryer only checks focusManager.isFocused() (which reads
// document.visibilityState) before a *scheduled retry* — the very first
// attempt always fires. A hidden tab at the moment the retry is due makes the
// retryer call pause() and wait for a visibilitychange event that never
// comes, so the query sits forever at fetchStatus "paused": isLoading and
// isError both stay false and data stays undefined. This is not contrived —
// it's the exact state a backgrounded tab (or an automated/headless browser
// driving the app, or a user who alt-tabs during the ~1s retry delay) ends up
// in, and it reproduces every time the retry window lands while hidden.
function forceHiddenTab() {
  Object.defineProperty(document, "visibilityState", { value: "hidden", configurable: true });
  Object.defineProperty(document, "hidden", { value: true, configurable: true });
}

describe("AdminIndexPage — real hooks, rejecting fetch, backgrounded tab", () => {
  beforeEach(() => {
    forceHiddenTab();
    global.fetch = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"));
  });

  it("shows dash_load_failed with a reload button instead of a confident zeroed dashboard", async () => {
    // Mirrors providers.tsx's defaultOptions — retry: 1 is exactly what
    // creates the pause window between the first failure and the retry.
    const qc = new QueryClient({
      defaultOptions: { queries: { staleTime: 30 * 1000, refetchOnWindowFocus: false, retry: 1 } },
    });

    render(
      <QueryClientProvider client={qc}>
        <AdminIndexPage />
      </QueryClientProvider>
    );

    await waitFor(() => expect(screen.getByText("Gagal memuat dashboard.")).toBeInTheDocument(), {
      timeout: 5000,
    });
    expect(screen.queryByTestId("kpi-row")).toBeNull();
    expect(screen.queryByText("Rp0")).toBeNull();

    // The audit panel is stuck in the same paused state, so its own reload
    // button renders too — assert at least one exists rather than exactly one.
    expect(screen.getAllByRole("button", { name: /muat ulang/i }).length).toBeGreaterThan(0);
  });

  it("shows admin_home_audit_failed instead of a false empty-activity state", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { staleTime: 30 * 1000, refetchOnWindowFocus: false, retry: 1 } },
    });

    render(
      <QueryClientProvider client={qc}>
        <AdminIndexPage />
      </QueryClientProvider>
    );

    await waitFor(
      () => expect(screen.getByText("Gagal memuat log aktivitas. Coba lagi nanti.")).toBeInTheDocument(),
      { timeout: 5000 }
    );
    expect(screen.queryByText("Belum ada aktivitas.")).toBeNull();
  });
});
