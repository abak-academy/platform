import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import {
  useAdminAffiliator,
  useAdminAffiliators,
  useBulkCreateAffiliators,
  useCreateAffiliator,
  usePromoteAffiliator,
  useSetAffiliatorSchools,
  useSetAffiliatorStatus,
} from "./admin-affiliators";

const authFetch = vi.fn();
vi.mock("@/lib/api", () => ({ authFetch: (...args: unknown[]) => authFetch(...args) }));

function wrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("admin affiliator hooks", () => {
  beforeEach(() => authFetch.mockReset());

  it("lists affiliators with the status filter", async () => {
    authFetch.mockResolvedValue({ data: [{ id: "a1" }] });
    const { result } = renderHook(() => useAdminAffiliators("deactivated"), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.data).toEqual([{ id: "a1" }]));
    expect(authFetch).toHaveBeenCalledWith("/admin/system/affiliators?limit=100&status=deactivated");
  });

  it("does not fetch a detail until an id is chosen", () => {
    renderHook(() => useAdminAffiliator(null), { wrapper: wrapper() });

    expect(authFetch).not.toHaveBeenCalled();
  });

  it("fetches one affiliator by id", async () => {
    authFetch.mockResolvedValue({ affiliator: { id: "a 1" } });
    const { result } = renderHook(() => useAdminAffiliator("a 1"), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetch).toHaveBeenCalledWith("/admin/system/affiliators/a%201");
  });

  it("sends each write to its own route", async () => {
    authFetch.mockResolvedValue({});
    const create = renderHook(() => useCreateAffiliator(), { wrapper: wrapper() });
    const bulk = renderHook(() => useBulkCreateAffiliators(), { wrapper: wrapper() });
    const promote = renderHook(() => usePromoteAffiliator(), { wrapper: wrapper() });
    const schools = renderHook(() => useSetAffiliatorSchools(), { wrapper: wrapper() });
    const status = renderHook(() => useSetAffiliatorStatus(), { wrapper: wrapper() });
    const input = { email: "a@example.com", name: "A", password: "password1", school_ids: ["s1"] };

    await act(async () => {
      await create.result.current.mutateAsync(input);
      await bulk.result.current.mutateAsync([input]);
      await promote.result.current.mutateAsync({ id: "u1", school_ids: ["s1"] });
      await schools.result.current.mutateAsync({ id: "u1", school_ids: ["s2"] });
      await status.result.current.mutateAsync({ id: "u1", status: "deactivated" });
    });

    expect(authFetch.mock.calls).toEqual([
      ["/admin/system/affiliators", { method: "POST", body: JSON.stringify(input) }],
      ["/admin/system/affiliators/bulk", { method: "POST", body: JSON.stringify({ rows: [input] }) }],
      ["/admin/system/affiliators/u1/promote", { method: "POST", body: JSON.stringify({ school_ids: ["s1"] }) }],
      ["/admin/system/affiliators/u1/schools", { method: "PUT", body: JSON.stringify({ school_ids: ["s2"] }) }],
      ["/admin/system/accounts/u1/status", { method: "PATCH", body: JSON.stringify({ status: "deactivated" }) }],
    ]);
  });
});
