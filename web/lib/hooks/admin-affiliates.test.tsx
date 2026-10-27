import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  adminAffiliateKeys,
  useAdminAffiliates,
  useDecideAffiliateApproval,
  useDecideAffiliatePayout,
  useAdminAffiliateCommissions,
  useAdminAffiliateFlags,
  useAffiliateReportSummary,
  useRunAffiliateJob,
} from "./admin-affiliates";

const mockAuthFetch = vi.fn();

vi.mock("@/lib/api", () => ({
  API_BASE: "http://localhost:8080/api/v1",
  ApiError: class extends Error {},
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

function wrapperFactory() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
    queryClient,
  };
}

describe("admin affiliate hooks", () => {
  beforeEach(() => mockAuthFetch.mockReset());

  it("lists affiliates with only the filters that are set", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], page: 1, per_page: 20, total: 0 });
    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminAffiliates({ status: "pending_approval", q: "budi", page: 2 }), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/affiliates?status=pending_approval&q=budi&page=2");
  });

  it("posts an approval and refreshes every affiliate query", async () => {
    mockAuthFetch.mockResolvedValueOnce({});
    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useDecideAffiliateApproval(), { wrapper });

    await act(() => result.current.mutateAsync({ id: "a1", decision: "reject", reason: "data palsu" }));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/affiliates/a1/approval", {
      method: "POST",
      body: JSON.stringify({ decision: "reject", reason: "data palsu" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminAffiliateKeys.all });
  });

  it("sends the proof key for a paid payout and the reason for a rejection", async () => {
    mockAuthFetch.mockResolvedValue({});
    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useDecideAffiliatePayout(), { wrapper });

    await act(() => result.current.mutateAsync({ id: "p1", decision: "paid", proof_url: "payment_proof/x.png" }));
    await act(() => result.current.mutateAsync({ id: "p2", decision: "reject", reason: "rekening salah" }));

    expect(mockAuthFetch).toHaveBeenNthCalledWith(1, "/admin/affiliate-payouts/p1/decide", {
      method: "POST",
      body: JSON.stringify({ decision: "paid", proof_url: "payment_proof/x.png", reason: "" }),
    });
    expect(mockAuthFetch).toHaveBeenNthCalledWith(2, "/admin/affiliate-payouts/p2/decide", {
      method: "POST",
      body: JSON.stringify({ decision: "reject", proof_url: "", reason: "rekening salah" }),
    });
  });

  it("returns the flag list body as sent", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [{ id: "f1" }] });
    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminAffiliateFlags({ q: "budi", status: "", page: 2, per_page: 20 }), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/affiliate-flags?q=budi&status=all&page=2&per_page=20");
    expect(result.current.data).toEqual({ data: [{ id: "f1" }] });
  });

  it("asks for one affiliator's commissions with the filters", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], total: 0, page: 1, per_page: 10 });
    const { wrapper } = wrapperFactory();
    const { result } = renderHook(
      () => useAdminAffiliateCommissions("a/1", { q: "sari", status: "available", type: "course", from: "2026-10-01", to: "2026-10-31", page: 1, per_page: 10 }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/affiliates/a%2F1/commissions?q=sari&status=available&type=course&from=2026-10-01&to=2026-10-31&page=1&per_page=10",
    );
  });

  it("asks for the report with search, level and paging", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], totals: {}, total: 0 });
    const { wrapper } = wrapperFactory();
    const { result } = renderHook(
      () => useAffiliateReportSummary({ from: "2026-10-01", to: "2026-10-31", q: "budi", level: "gold", page: 2, per_page: 20 }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/affiliate-reports/summary?from=2026-10-01&to=2026-10-31&q=budi&level=gold&page=2&per_page=20",
    );
  });

  it("reruns a job with the scope and quarter", async () => {
    mockAuthFetch.mockResolvedValueOnce({ scope: "levels" });
    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useRunAffiliateJob(), { wrapper });

    await act(() => result.current.mutateAsync({ scope: "levels", quarter: "2026-Q3" }));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/affiliate-jobs/run", {
      method: "POST",
      body: JSON.stringify({ scope: "levels", quarter: "2026-Q3" }),
    });
  });
});
