import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useSchoolById, useSchoolAutocomplete, useSchoolSearch, useUpdatePhoto, usePresignUpload, studentsKeys } from "./students";
import type { SchoolOption, User } from "@/lib/types";

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

const mockSetSession = vi.fn();

vi.mock("@/stores/auth", () => ({
  useAuthStore: {
    getState: () => ({
      token: "test-token-123",
      setSession: mockSetSession,
    }),
  },
}));

describe("school search hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("useSchoolSearch sends bounded search params", async () => {
    const schools: SchoolOption[] = [{ id: "s1", name: "SMAN 1 Jakarta", code: "SMAN1JKT" }];
    mockAuthFetch.mockResolvedValueOnce({ data: schools });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(
      () => useSchoolSearch({ province_id: "p1", city_id: "c1", school_type: "SMA", limit: 20 }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/schools?province_id=p1&city_id=c1&school_type=SMA&limit=20");
    expect(result.current.data).toEqual({ data: schools });
  });

  it("useSchoolSearch issues no request when disabled", async () => {
    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useSchoolSearch({ province_id: "p1" }, false), { wrapper });

    expect(result.current.isPending).toBe(true);
    expect(result.current.fetchStatus).toBe("idle");
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("paginates autocomplete with the same name and partial filters, then resets on a new query", async () => {
    const school = { id: "s1", name: "Needle, Academy", code: "N1" };
    mockAuthFetch.mockResolvedValueOnce({ data: [school], next_cursor: "Needle, Academy,s1" })
      .mockResolvedValueOnce({ data: [{ ...school, id: "s2" }] })
      .mockResolvedValueOnce({ data: [] });
    const { wrapper } = wrapperFactory();
    const { result, rerender } = renderHook(({ q }) => useSchoolAutocomplete({ q, province_id: "35", city_id: "3578", limit: 20 }), { wrapper, initialProps: { q: "needle" } });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.pages).toHaveLength(1);
    await act(async () => { await result.current.fetchNextPage(); });
    expect(mockAuthFetch).toHaveBeenNthCalledWith(2, "/schools?q=needle&cursor=Needle%2C+Academy%2Cs1&province_id=35&city_id=3578&limit=20", { signal: expect.any(AbortSignal) });
    await waitFor(() => expect(result.current.data?.pages.flatMap(page => page.data)).toHaveLength(2));
    expect(result.current.hasNextPage).toBe(false);
    rerender({ q: "other" });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenLastCalledWith("/schools?q=other&province_id=35&city_id=3578&limit=20", { signal: expect.any(AbortSignal) });
    expect(result.current.data?.pages).toHaveLength(1);
  });

  it("reuses a recently cached search when returning to it", async () => {
    mockAuthFetch.mockImplementation(async (path: string) => ({ data: [{ id: path, name: path, code: "cached" }] }));
    const { wrapper, queryClient } = wrapperFactory();
    const params = { q: "bina", province_id: "35", city_id: "3578", limit: 20 };
    const key = [...studentsKeys.schoolSearch(params), "autocomplete"];
    queryClient.setQueryData(key, { pages: [{ data: [{ id: "cached", name: "Bina", code: "cached" }] }], pageParams: [""] }, { updatedAt: Date.now() - 60_000 });
    const { result, rerender } = renderHook(({ q }) => useSchoolAutocomplete({ ...params, q }), { wrapper, initialProps: { q: "bina" } });
    expect(result.current.data?.pages[0].data[0].id).toBe("cached");
    expect(mockAuthFetch).not.toHaveBeenCalled();
    rerender({ q: "negeri" });
    await waitFor(() => expect(result.current.isFetching).toBe(false));
    expect(mockAuthFetch).toHaveBeenCalledTimes(1);
    rerender({ q: "bina" });
    expect(result.current.data?.pages[0].data[0].id).toBe("cached");
    expect(result.current.isFetching).toBe(false);
    expect(mockAuthFetch).toHaveBeenCalledTimes(1);
  });

  it("refreshes a cached search older than five minutes and isolates filters", async () => {
    mockAuthFetch.mockResolvedValue({ data: [] });
    const { wrapper, queryClient } = wrapperFactory();
    const params = { q: "bina", province_id: "35", city_id: "3578", limit: 20 };
    queryClient.setQueryData([...studentsKeys.schoolSearch(params), "autocomplete"], { pages: [{ data: [] }], pageParams: [""] }, { updatedAt: Date.now() - 5 * 60_000 - 1 });
    const { result, rerender } = renderHook(({ city_id }) => useSchoolAutocomplete({ ...params, city_id }), { wrapper, initialProps: { city_id: "3578" } });
    await waitFor(() => expect(result.current.isFetching).toBe(false));
    expect(mockAuthFetch).toHaveBeenCalledTimes(1);
    rerender({ city_id: "3573" });
    await waitFor(() => expect(result.current.isFetching).toBe(false));
    expect(mockAuthFetch).toHaveBeenCalledTimes(2);
    expect(mockAuthFetch.mock.calls[1][0]).toContain("city_id=3573");
  });

  it("aborts superseded and unmounted searches without caching their late results", async () => {
    const requests: Array<{ signal: AbortSignal; resolve: (data: unknown) => void }> = [];
    mockAuthFetch.mockImplementation((_path: string, options?: RequestInit) => new Promise(resolve => {
      requests.push({ signal: options?.signal as AbortSignal, resolve });
    }));
    const { wrapper, queryClient } = wrapperFactory();
    const { result, rerender, unmount } = renderHook(({ q }) => useSchoolAutocomplete({ q, limit: 20 }), { wrapper, initialProps: { q: "old" } });
    await waitFor(() => expect(requests).toHaveLength(1));
    expect(requests[0].signal).toBeInstanceOf(AbortSignal);
    expect(requests[0].signal.aborted).toBe(false);
    rerender({ q: "new" });
    await waitFor(() => expect(requests).toHaveLength(2));
    expect(requests[0].signal.aborted).toBe(true);
    await act(async () => { requests[0].resolve({ data: [{ id: "old" }] }); });
    expect(result.current.data).toBeUndefined();
    expect(queryClient.getQueryData([...studentsKeys.schoolSearch({ q: "old", limit: 20 }), "autocomplete"])).toBeUndefined();
    unmount();
    expect(requests[1].signal.aborted).toBe(true);
  });

  it("useSchoolById hydrates one school", async () => {
    const school: SchoolOption = { id: "s1", name: "SMAN 1 Jakarta", code: "SMAN1JKT" };
    mockAuthFetch.mockResolvedValueOnce(school);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useSchoolById("s1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/schools/s1");
    expect(result.current.data).toEqual(school);
  });
});

describe("useUpdatePhoto", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
    mockSetSession.mockClear();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("calls authFetch PATCH and updates auth store on success", async () => {
    const updatedUser: User = {
      id: "u1",
      name: "Budi Santoso",
      email: "budi@test.com",
      photo_url: "https://example.com/new-photo.jpg",
    };
    mockAuthFetch.mockResolvedValueOnce(updatedUser);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdatePhoto(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("https://example.com/new-photo.jpg");
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/students/photo", {
      method: "PATCH",
      body: JSON.stringify({ photo_url: "https://example.com/new-photo.jpg" }),
    });

    // Auth store should be updated with token, refreshToken, and returned user
    expect(mockSetSession).toHaveBeenCalledWith("test-token-123", "", updatedUser);

    // Profile query should still be invalidated
    expect(spy).toHaveBeenCalledWith({ queryKey: studentsKeys.profile() });
  });
});

describe("usePresignUpload", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("sends no kind param when kind is omitted (student profile call)", async () => {
    mockAuthFetch.mockResolvedValueOnce({ url: "https://upload.example", method: "PUT", key: "avatars/u/photo.png" });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => usePresignUpload(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ filename: "photo.png", content_type: "image/png" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/uploads/presign?filename=photo.png&content_type=image%2Fpng",
      { method: "POST" }
    );
  });

  it("appends &kind=product when kind is provided", async () => {
    mockAuthFetch.mockResolvedValueOnce({ url: "https://upload.example", method: "PUT", key: "product/img.png" });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => usePresignUpload(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ filename: "img.png", content_type: "image/png", kind: "product" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/uploads/presign?filename=img.png&content_type=image%2Fpng&kind=product",
      { method: "POST" }
    );
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
