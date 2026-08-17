import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGoogleLogin, useLogout } from "./auth";

const apiFetch = vi.fn();
const setSession = vi.fn();
const clear = vi.fn();

vi.mock("@/lib/api", () => ({
  apiFetch: (...args: unknown[]) => apiFetch(...args),
  authFetch: vi.fn(),
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (
    selector: (state: { setSession: typeof setSession; clear: typeof clear }) => unknown,
  ) => selector({ setSession, clear }),
}));

describe("useGoogleLogin", () => {
  beforeEach(() => {
    apiFetch.mockReset();
    setSession.mockReset();
  });

  it("posts the id token and stores the returned session", async () => {
    const user = { id: "user-1", role: "student", auth_provider: "google" };
    apiFetch.mockResolvedValue({
      access_token: "access",
      refresh_token: "refresh",
      user,
    });
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useGoogleLogin(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id_token: "google-token" });
    });

    expect(apiFetch).toHaveBeenCalledWith("/auth/google", {
      method: "POST",
      body: JSON.stringify({ id_token: "google-token" }),
    });
    expect(setSession).toHaveBeenCalledWith("access", "refresh", user);
  });
});

describe("useLogout", () => {
  beforeEach(() => {
    clear.mockReset();
  });

  it("clears the react-query cache so a prior session's data can't leak to the next user", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });
    // Seed a cached profile from the previous session.
    queryClient.setQueryData(["students", "profile"], { id: "prev-user" });
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useLogout(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync();
    });

    expect(clear).toHaveBeenCalled();
    expect(queryClient.getQueryData(["students", "profile"])).toBeUndefined();
  });
});

describe("useLogin retry on 429 rate_limited", () => {
  const originalFetch = global.fetch;

  function jsonResponse(status: number, body: unknown) {
    return {
      ok: status >= 200 && status < 300,
      status,
      text: async () => JSON.stringify(body),
      json: async () => body,
    } as Response;
  }

  async function freshUseLogin() {
    vi.doUnmock("@/lib/api");
    vi.resetModules();
    const mod = await import("./auth");
    const api = await import("@/lib/api");
    return { useLogin: mod.useLogin, ApiError: api.ApiError };
  }

  function wrapperFor(queryClient: QueryClient) {
    return ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }

  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    global.fetch = originalFetch;
    vi.doUnmock("@/lib/api");
    vi.resetModules();
  });

  it("retries a rate_limited 429 twice, waiting up to the jittered base delay, then resolves", async () => {
    const { useLogin } = await freshUseLogin();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(429, { code: "rate_limited", message: "too many login attempts" }))
      .mockResolvedValueOnce(jsonResponse(429, { code: "rate_limited", message: "too many login attempts" }))
      .mockResolvedValueOnce(jsonResponse(200, { access_token: "a", refresh_token: "r", user: { id: "1" } }));
    global.fetch = fetchMock as unknown as typeof fetch;
    // Stub the jitter source to its maximum so the drawn delay equals the base (1000ms, then 2000ms).
    const randomSpy = vi.spyOn(Math, "random").mockReturnValue(1);

    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const { result } = renderHook(() => useLogin(), { wrapper: wrapperFor(queryClient) });

    let resolved = false;
    const promise = result.current
      .mutateAsync({ identifier: "budi", password: "secret" })
      .then(() => {
        resolved = true;
      });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(999);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(fetchMock).toHaveBeenCalledTimes(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1999);
    });
    expect(fetchMock).toHaveBeenCalledTimes(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
      await promise;
    });

    expect(resolved).toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    randomSpy.mockRestore();
  });

  it("never waits longer than the base delay for a retry attempt", async () => {
    const { useLogin } = await freshUseLogin();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(429, { code: "rate_limited", message: "too many login attempts" }))
      .mockResolvedValueOnce(jsonResponse(429, { code: "rate_limited", message: "too many login attempts" }))
      .mockResolvedValueOnce(jsonResponse(200, { access_token: "a", refresh_token: "r", user: { id: "1" } }));
    global.fetch = fetchMock as unknown as typeof fetch;
    // Mid-range jitter draw: delay is a fraction of the base, never the base itself.
    const randomSpy = vi.spyOn(Math, "random").mockReturnValue(0.5);

    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const { result } = renderHook(() => useLogin(), { wrapper: wrapperFor(queryClient) });

    const promise = result.current.mutateAsync({ identifier: "budi", password: "secret" });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(499);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(fetchMock).toHaveBeenCalledTimes(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(999);
    });
    expect(fetchMock).toHaveBeenCalledTimes(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
      await promise;
    });
    expect(fetchMock).toHaveBeenCalledTimes(3);
    randomSpy.mockRestore();
  });

  it("fires retries immediately when the jitter source is stubbed to its minimum", async () => {
    const { useLogin } = await freshUseLogin();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(429, { code: "rate_limited", message: "too many login attempts" }))
      .mockResolvedValueOnce(jsonResponse(429, { code: "rate_limited", message: "too many login attempts" }))
      .mockResolvedValueOnce(jsonResponse(200, { access_token: "a", refresh_token: "r", user: { id: "1" } }));
    global.fetch = fetchMock as unknown as typeof fetch;
    const randomSpy = vi.spyOn(Math, "random").mockReturnValue(0);

    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const { result } = renderHook(() => useLogin(), { wrapper: wrapperFor(queryClient) });

    let resolved = false;
    const promise = result.current
      .mutateAsync({ identifier: "budi", password: "secret" })
      .then(() => {
        resolved = true;
      });

    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(resolved).toBe(true);

    await promise;
    randomSpy.mockRestore();
  });

  it("does not retry a 401 and rejects immediately with a single fetch call", async () => {
    const { useLogin, ApiError } = await freshUseLogin();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(401, { code: "invalid_credentials", message: "bad credentials" }));
    global.fetch = fetchMock as unknown as typeof fetch;

    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const { result } = renderHook(() => useLogin(), { wrapper: wrapperFor(queryClient) });

    await expect(
      result.current.mutateAsync({ identifier: "budi", password: "wrong" }),
    ).rejects.toBeInstanceOf(ApiError);

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("rejects with an ApiError after exactly 3 calls when 429s never stop", async () => {
    const { useLogin, ApiError } = await freshUseLogin();
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(429, { code: "rate_limited", message: "too many login attempts" }));
    global.fetch = fetchMock as unknown as typeof fetch;
    const randomSpy = vi.spyOn(Math, "random").mockReturnValue(1);

    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const { result } = renderHook(() => useLogin(), { wrapper: wrapperFor(queryClient) });

    let error: unknown;
    const promise = result.current.mutateAsync({ identifier: "budi", password: "secret" }).catch((err) => {
      error = err;
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(999);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(fetchMock).toHaveBeenCalledTimes(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1999);
    });
    expect(fetchMock).toHaveBeenCalledTimes(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
      await promise;
    });

    expect(error).toBeInstanceOf(ApiError);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    randomSpy.mockRestore();
  });
});
