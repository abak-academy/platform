// @vitest-environment node
import { renderToString } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, it, vi } from "vitest";
import { useSchoolAutocomplete } from "./students";

const mockAuthFetch = vi.fn();
vi.mock("@/lib/api", () => ({ authFetch: (...args: unknown[]) => mockAuthFetch(...args) }));
vi.mock("@/stores/auth", () => ({ useAuthStore: { getState: () => ({}) } }));

it("server rendering neither fetches schools nor retains query caches with eviction timers", () => {
  vi.useFakeTimers();
  const client = new QueryClient();
  function PickerSearch() {
    useSchoolAutocomplete({ q: "bina", limit: 20 });
    return null;
  }
  try {
    renderToString(<QueryClientProvider client={client}><PickerSearch /></QueryClientProvider>);
    expect(mockAuthFetch).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  } finally {
    client.clear();
    vi.useRealTimers();
  }
});
