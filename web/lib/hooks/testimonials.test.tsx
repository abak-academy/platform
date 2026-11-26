import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useTestimonials } from "./testimonials";
import * as api from "@/lib/api";

function wrapper({ children }: { children: React.ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

afterEach(() => vi.restoreAllMocks());

describe("useTestimonials", () => {
  it("requests the public endpoint with the language and unwraps data", async () => {
    const spy = vi.spyOn(api, "apiFetch").mockResolvedValue({
      data: [{ id: "t1", name: "Peserta A", role_label: "Siswa", photo_url: "testimonials/a.jpg", quote: "Seru", achievements: [] }],
    } as any);
    const { result } = renderHook(() => useTestimonials("en"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(spy).toHaveBeenCalledWith("/testimonials?lang=en");
    expect(result.current.data?.map((t) => t.id)).toEqual(["t1"]);
  });

  it("returns an empty list when data is null", async () => {
    vi.spyOn(api, "apiFetch").mockResolvedValue({ data: null } as any);
    const { result } = renderHook(() => useTestimonials("id"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });
});
