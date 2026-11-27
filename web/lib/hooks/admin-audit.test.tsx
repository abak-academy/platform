import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAdminAuditLog, adminAuditKeys } from "./admin-audit";
import type { AuditLogEntry } from "@/lib/types";

const mockAuthFetch = vi.fn();

vi.mock("@/lib/api", () => ({
  authFetch: (...args: Parameters<typeof mockAuthFetch>) => mockAuthFetch(...args),
  ApiError: class extends Error {
    code: string;
    status: number;
    constructor(code: string, message: string, status: number) {
      super(message);
      this.code = code;
      this.status = status;
    }
  },
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: {
    getState: () => ({ token: "test-token" }),
  },
}));

describe("admin-audit hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("useAdminAuditLog fetches GET /admin/system/audit and returns data", async () => {
    const entries: AuditLogEntry[] = [
      { id: 1, actor_id: "u1", actor_name: "Admin", target_type: "user", target_id: "t1", action: "account.create", created_at: "2026-01-01T00:00:00Z" },
    ];
    mockAuthFetch.mockResolvedValueOnce({ data: entries, total: 1, page: 1, per_page: 20 });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminAuditLog(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/system/audit");
    expect(result.current.data).toEqual({ data: entries, total: 1, page: 1, per_page: 20 });
  });

  it("useAdminAuditLog passes query params when filters are provided", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [] });

    const { wrapper } = wrapperFactory();
    renderHook(() => useAdminAuditLog({ actor_id: "u1", target_type: "user", q: "create" }), { wrapper });

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith("/admin/system/audit?actor_id=u1&target_type=user&q=create")
    );
  });

  it("sends page and limit and keys the cache per page", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], total: 45, page: 3, per_page: 20 });

    const { wrapper } = wrapperFactory();
    renderHook(() => useAdminAuditLog({ q: "x", page: 3, limit: 20 }), { wrapper });

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith("/admin/system/audit?q=x&page=3&limit=20")
    );
    expect(adminAuditKeys.list({ page: 2 })).not.toEqual(adminAuditKeys.list({ page: 3 }));
  });

  it("uses stable query key", () => {
    expect(adminAuditKeys.list()).toEqual(["admin", "audit", "list"]);
  });
});

function wrapperFactory() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
    queryClient,
  };
}
