import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAdminUnitSales, downloadUnitSalesCsv } from "./admin-unit-sales";
import { authFetch } from "@/lib/api";

vi.mock("@/lib/api", () => ({ authFetch: vi.fn(), API_BASE: "/api/v1", ApiError: class extends Error {} }));
vi.mock("@/stores/auth", () => ({ useAuthStore: { getState: () => ({ token: "test-token" }) } }));

beforeEach(() => { vi.clearAllMocks(); });

describe("unit sales hook", () => {
  it("refetches when the range changes and all-time omits dates", async () => {
    vi.mocked(authFetch).mockResolvedValue({ from: null, to: null, types: [] });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: React.ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
    const { result, rerender } = renderHook(({ range }) => useAdminUnitSales(range), { wrapper, initialProps: { range: {} as { from?: string; to?: string } } });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetch).toHaveBeenLastCalledWith("/admin/orders/unit-sales");
    rerender({ range: { from: "2026-01-01", to: "2026-01-31" } });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetch).toHaveBeenLastCalledWith("/admin/orders/unit-sales?from=2026-01-01&to=2026-01-31");
  });

  it.each([{}, { from: "2026-01-01", to: "2026-01-31" }])("downloads just one type for range %j", async (range) => {
    const blob = new Blob(["csv"]);
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue({ ok: true, blob: async () => blob } as Response);
    URL.createObjectURL = vi.fn().mockReturnValue("blob:test");
    URL.revokeObjectURL = vi.fn();
    let filename = "";
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) { filename = this.download; });
    await downloadUnitSalesCsv("book", range);
    expect(fetchMock).toHaveBeenCalledWith(`/api/v1/admin/orders/unit-sales/export?type=book${"from" in range ? "&from=2026-01-01&to=2026-01-31" : ""}`, { headers: { Authorization: "Bearer test-token" } });
    expect(filename).toBe(`unit-terjual-book-${range.from ?? "semua-waktu"}_${range.to ?? "semua-waktu"}.csv`);
    expect(URL.createObjectURL).toHaveBeenCalledWith(blob);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:test");
    click.mockRestore(); fetchMock.mockRestore();
  });
});
