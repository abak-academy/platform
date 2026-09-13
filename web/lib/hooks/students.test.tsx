import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useSchoolById, useSchoolSearch, useUpdatePhoto, usePresignUpload, studentsKeys } from "./students";
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
    mockAuthFetch.mockResolvedValueOnce({ data: schools, next_cursor: "next" });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(
      () => useSchoolSearch({ province_id: "p1", q: "sman", category: "SMA", limit: 20 }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/schools?q=sman&province_id=p1&category=SMA&limit=20");
    expect(result.current.data).toEqual({ data: schools, next_cursor: "next" });
  });

  it("useSchoolSearch issues no request when disabled", async () => {
    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useSchoolSearch({ q: "sman" }, false), { wrapper });

    expect(result.current.isPending).toBe(true);
    expect(result.current.fetchStatus).toBe("idle");
    expect(mockAuthFetch).not.toHaveBeenCalled();
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
