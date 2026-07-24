import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useSchools, useUpdatePhoto, usePresignUpload, studentsKeys } from "./students";
import type { School, User } from "@/lib/types";

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

describe("useSchools", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("fetches GET /schools and returns bare School array", async () => {
    const schools: School[] = [
      { id: "s1", name: "SMAN 1 Jakarta" },
      { id: "s2", name: "SMAN 2 Jakarta" },
    ];
    mockAuthFetch.mockResolvedValueOnce(schools);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useSchools(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/schools");
    // Assert data is the bare array, not wrapped in { data: ... }
    expect(Array.isArray(result.current.data)).toBe(true);
    expect(result.current.data).toEqual(schools);
  });

  it("returns empty array when empty response", async () => {
    mockAuthFetch.mockResolvedValueOnce([]);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useSchools(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual([]);
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
