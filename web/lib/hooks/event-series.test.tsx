import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useEventSeries } from "./event-series";
import * as api from "@/lib/api";

function wrapper({ children }: { children: React.ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

afterEach(() => vi.restoreAllMocks());

describe("useEventSeries", () => {
  it("requests the public endpoint with the language and unwraps data", async () => {
    const spy = vi.spyOn(api, "apiFetch").mockResolvedValue({ data: [{ slug: "ikmc", name: "IKMC" }] } as any);
    const { result } = renderHook(() => useEventSeries("en"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(spy).toHaveBeenCalledWith("/event-series?lang=en");
    expect(result.current.data?.map((e) => e.slug)).toEqual(["ikmc"]);
  });

  it("returns an empty list when data is null", async () => {
    vi.spyOn(api, "apiFetch").mockResolvedValue({ data: null } as any);
    const { result } = renderHook(() => useEventSeries("id"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });
});
