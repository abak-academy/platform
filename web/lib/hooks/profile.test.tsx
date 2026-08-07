import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useUpdateOwnPhoto } from "./profile";
import type { User } from "@/lib/types";

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
      refreshToken: "refresh-abc",
      setSession: mockSetSession,
    }),
  },
}));

describe("useUpdateOwnPhoto", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
    mockSetSession.mockClear();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("PATCHes /auth/photo, writes the returned user back to the auth store, and invalidates auth/me", async () => {
    const updatedUser: User = {
      id: "u1",
      name: "Budi Santoso",
      email: "budi@test.com",
      photo_url: "avatars/u1/new-photo.jpg",
    };
    mockAuthFetch.mockResolvedValueOnce(updatedUser);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateOwnPhoto(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("avatars/u1/new-photo.jpg");
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/auth/photo", {
      method: "PATCH",
      body: JSON.stringify({ photo_url: "avatars/u1/new-photo.jpg" }),
    });

    expect(mockSetSession).toHaveBeenCalledWith("test-token-123", "refresh-abc", updatedUser);
    expect(spy).toHaveBeenCalledWith({ queryKey: ["auth", "me"] });
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
