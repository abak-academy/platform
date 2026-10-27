import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import {
  downloadAffiliateReportCsv,
  useAdminAffiliator,
  useAdminAffiliators,
  useAffiliateReport,
  useAffiliateTrend,
  useCreateAffiliator,
  useEnqueueAffiliatorBulkImport,
  usePresignAffiliatorBulkUpload,
  usePromotableAdmins,
  usePromoteAffiliator,
  useSetAffiliatorSchools,
  useSetAffiliatorStatus,
} from "./admin-affiliators";

const authFetch = vi.fn();
vi.mock("@/lib/api", () => ({
  authFetch: (...args: unknown[]) => authFetch(...args),
  API_BASE: "http://api.test/api/v1",
  ApiError: class extends Error {
    constructor(public code: string, message: string, public status: number) {
      super(message);
    }
  },
}));
vi.mock("@/stores/auth", () => ({ useAuthStore: { getState: () => ({ token: "tok" }) } }));

function wrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("admin affiliator hooks", () => {
  beforeEach(() => authFetch.mockReset());

  it("lists affiliators with status, search, cursor and page size", async () => {
    authFetch.mockResolvedValue({ data: [{ id: "a1" }], total: 7, next_cursor: "c2" });
    const { result } = renderHook(() => useAdminAffiliators({ status: "deactivated", q: "andi", cursor: "c1", limit: 50 }), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.data?.total).toBe(7));
    expect(authFetch).toHaveBeenCalledWith("/admin/system/affiliators?status=deactivated&q=andi&cursor=c1&limit=50");
  });

  it("asks for 20 rows and omits empty filters by default", async () => {
    authFetch.mockResolvedValue({ data: [], total: 0 });
    const { result } = renderHook(() => useAdminAffiliators({}), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetch).toHaveBeenCalledWith("/admin/system/affiliators?limit=20");
  });

  it("lists the school admins that can be promoted", async () => {
    authFetch.mockResolvedValue({ data: [{ id: "u1" }], total: 1 });
    const { result } = renderHook(() => usePromotableAdmins({ q: "sma", limit: 20 }), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.data?.data).toEqual([{ id: "u1" }]));
    expect(authFetch).toHaveBeenCalledWith("/admin/system/affiliators/promotable?q=sma&limit=20");
  });

  it("does not fetch a detail until an id is chosen, then fetches by id", async () => {
    renderHook(() => useAdminAffiliator(null), { wrapper: wrapper() });
    expect(authFetch).not.toHaveBeenCalled();

    authFetch.mockResolvedValue({ affiliator: { id: "a 1" } });
    const { result } = renderHook(() => useAdminAffiliator("a 1"), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetch).toHaveBeenCalledWith("/admin/system/affiliators/a%201");
  });

  it("sends each write to its own route", async () => {
    authFetch.mockResolvedValue({});
    const create = renderHook(() => useCreateAffiliator(), { wrapper: wrapper() });
    const promote = renderHook(() => usePromoteAffiliator(), { wrapper: wrapper() });
    const schools = renderHook(() => useSetAffiliatorSchools(), { wrapper: wrapper() });
    const status = renderHook(() => useSetAffiliatorStatus(), { wrapper: wrapper() });
    const presign = renderHook(() => usePresignAffiliatorBulkUpload(), { wrapper: wrapper() });
    const enqueue = renderHook(() => useEnqueueAffiliatorBulkImport(), { wrapper: wrapper() });
    const input = { email: "a@example.com", name: "A", school_ids: ["s1"] };

    await act(async () => {
      await create.result.current.mutateAsync(input);
      await promote.result.current.mutateAsync({ id: "u1", school_ids: ["s1"] });
      await schools.result.current.mutateAsync({ id: "u1", school_ids: ["s2"] });
      await status.result.current.mutateAsync({ id: "u1", status: "deactivated" });
      await presign.result.current.mutateAsync({ filename: "a b.csv", contentType: "text/csv" });
      await enqueue.result.current.mutateAsync({ fileKey: "affiliator-bulk/x.csv" });
    });

    expect(authFetch.mock.calls).toEqual([
      ["/admin/system/affiliators", { method: "POST", body: JSON.stringify(input) }],
      ["/admin/system/affiliators/u1/promote", { method: "POST", body: JSON.stringify({ school_ids: ["s1"] }) }],
      ["/admin/system/affiliators/u1/schools", { method: "PUT", body: JSON.stringify({ school_ids: ["s2"] }) }],
      ["/admin/system/accounts/u1/status", { method: "PATCH", body: JSON.stringify({ status: "deactivated" }) }],
      ["/admin/system/affiliators/bulk/presign?filename=a+b.csv&content_type=text%2Fcsv", { method: "POST" }],
      ["/admin/system/affiliators/bulk", { method: "POST", body: JSON.stringify({ file_key: "affiliator-bulk/x.csv" }) }],
    ]);
  });
});

describe("affiliate report hooks", () => {
  beforeEach(() => authFetch.mockReset());

  it("pages a report with the period, search, inactive flag and offset", async () => {
    authFetch.mockResolvedValue({ data: [], total: 0, limit: 50, offset: 100 });
    const { result } = renderHook(
      () => useAffiliateReport("affiliators", { from: "2026-10-01", to: "2026-10-31", q: "andi", inactive: true, limit: 50, offset: 100 }),
      { wrapper: wrapper() },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetch).toHaveBeenCalledWith("/admin/system/affiliate-reports/affiliators?from=2026-10-01&to=2026-10-31&q=andi&inactive=true&limit=50&offset=100");
  });

  it("starts at the first page of 20 and leaves an open period off the query", async () => {
    authFetch.mockResolvedValue({ data: [], total: 0 });
    const { result } = renderHook(() => useAffiliateReport("schools", {}), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetch).toHaveBeenCalledWith("/admin/system/affiliate-reports/schools?limit=20&offset=0");
  });

  it("reads the unpaged trend and unwraps it", async () => {
    authFetch.mockResolvedValue({ data: [{ month: "2026-10" }] });
    const { result } = renderHook(() => useAffiliateTrend({ from: "2026-01-01" }), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.data).toEqual([{ month: "2026-10" }]));
    expect(authFetch).toHaveBeenCalledWith("/admin/system/affiliate-reports/trend?from=2026-01-01");
  });

  it("downloads the whole report as a named CSV with the bearer token and no paging", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("a,b\n", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    let clicked: HTMLAnchorElement | undefined;
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
      clicked = this;
    });
    URL.createObjectURL = vi.fn(() => "blob:x");
    URL.revokeObjectURL = vi.fn();

    await downloadAffiliateReportCsv("promos", { from: "2026-10-01", to: "2026-10-31", q: "x", limit: 20, offset: 40 });

    expect(fetchMock).toHaveBeenCalledWith(
      "http://api.test/api/v1/admin/system/affiliate-reports/promos/export?from=2026-10-01&to=2026-10-31&q=x",
      { headers: { Authorization: "Bearer tok" } },
    );
    expect(clicked?.download).toBe("laporan-affiliator-promos-2026-10-01_2026-10-31.csv");
    vi.restoreAllMocks();
  });

  it("rejects when the server refuses the export", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("no", { status: 403 })));

    await expect(downloadAffiliateReportCsv("trend", {})).rejects.toThrow("Failed to download report: 403");
  });
});
