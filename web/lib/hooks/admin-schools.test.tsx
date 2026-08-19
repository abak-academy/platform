import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAdminSchools,
  useSchoolOptions,
  useCreateSchool,
  useUpdateSchool,
  useChangeSchoolStatus,
  adminSchoolsKeys,
} from "./admin-schools";
import type { School, SchoolOption } from "@/lib/types";

const mockAuthFetch = vi.fn();

vi.mock("@/lib/api", () => ({
  authFetch: (...args: Parameters<typeof mockAuthFetch>) =>
    mockAuthFetch(...args),
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

describe("admin-schools hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("useAdminSchools fetches GET /admin/schools and returns paginated response", async () => {
    const schools: School[] = [{ id: "s1", name: "SMAN 1 Jakarta" }];
    const response = { data: schools, next_cursor: "cursor-abc", total: 1, active: 1, students: 0 };
    mockAuthFetch.mockResolvedValueOnce(response);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminSchools(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/schools");
    expect(result.current.data).toEqual(response);
  });

  it("useAdminSchools passes cursor and limit as query params", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined, total: 0, active: 0, students: 0 });

    const { wrapper } = wrapperFactory();
    renderHook(() => useAdminSchools({ cursor: "cursor-xyz", limit: 10 }), { wrapper });

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/admin/schools?cursor=cursor-xyz&limit=10",
      ),
    );
  });

  it("useAdminSchools passes q and status as query params", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined, total: 0, active: 0, students: 0 });

    const { wrapper } = wrapperFactory();
    renderHook(() => useAdminSchools({ q: "sman", status: "active" }), { wrapper });

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/admin/schools?q=sman&status=active",
      ),
    );
  });

  it("useSchoolOptions fetches GET /admin/schools/options", async () => {
    const options: SchoolOption[] = [{ id: "s1", name: "SMAN 1 Jakarta", code: "SMAN1JKT" }];
    mockAuthFetch.mockResolvedValueOnce({ data: options });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useSchoolOptions(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/schools/options");
    expect(result.current.data).toEqual({ data: options });
  });

  it("useCreateSchool posts to /admin/schools and invalidates list", async () => {
    const school: School = {
      id: "s2",
      name: "SMAN 2 Jakarta",
      code: "SMAN2JKT",
    };
    mockAuthFetch.mockResolvedValueOnce(school);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useCreateSchool(), { wrapper });

    const input = {
      name: "SMAN 2 Jakarta",
      code: "SMAN2JKT",
      npsn: "12345678",
    };

    await act(async () => {
      await result.current.mutateAsync(input);
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/schools", {
      method: "POST",
      body: JSON.stringify(input),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminSchoolsKeys.all });
  });

  it("useUpdateSchool puts to /admin/schools/:id and invalidates list", async () => {
    const updated: School = {
      id: "s1",
      name: "SMAN 1 Jakarta Updated",
    };
    mockAuthFetch.mockResolvedValueOnce(updated);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateSchool(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "s1", name: "SMAN 1 Jakarta Updated" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/schools/s1", {
      method: "PUT",
      body: JSON.stringify({ name: "SMAN 1 Jakarta Updated" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminSchoolsKeys.all });
  });

  it("useChangeSchoolStatus patches /admin/schools/:id and invalidates list", async () => {
    const updated: School = {
      id: "s1",
      name: "SMAN 1 Jakarta",
      status: "deactivated",
    };
    mockAuthFetch.mockResolvedValueOnce(updated);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useChangeSchoolStatus(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "s1", status: "deactivated" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/schools/s1", {
      method: "PATCH",
      body: JSON.stringify({ status: "deactivated" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminSchoolsKeys.all });
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
