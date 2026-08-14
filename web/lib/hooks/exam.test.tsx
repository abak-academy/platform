import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useSaveAnswers,
  useReconnectSession,
  useSubmitSession,
  useAdvanceSection,
} from "./exam";

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

function wrapperFactory() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
  };
}

function isGetCall(call: unknown[]) {
  const init = call[1] as { method?: string } | undefined;
  return !init?.method;
}

describe("useSaveAnswers", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  // Blocker 3 root cause: with no serialization, two saves issued close
  // together race as independent in-flight requests, so an older, slower
  // one can settle after a newer one and stomp its result (position/queue
  // bookkeeping in page.tsx relies on saves resolving in the order they
  // were issued). The hook must serialize the underlying PATCH calls.
  it("does not dispatch a second save's PATCH until the first has settled (Blocker 3, FR-32, NFR-R5)", async () => {
    let resolveFirst: (() => void) | undefined;
    mockAuthFetch.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveFirst = resolve;
        }),
    );
    mockAuthFetch.mockImplementationOnce(() => Promise.resolve());

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useSaveAnswers("session-1"), {
      wrapper,
    });

    let firstDone = false;
    let secondDone = false;

    act(() => {
      result.current.mutate(
        {
          answers: [{ question_id: "q1", answer: "a", flagged_for_review: false }],
          current_position: 0,
        },
        { onSuccess: () => { firstDone = true; } },
      );
    });
    act(() => {
      result.current.mutate(
        {
          answers: [{ question_id: "q1", answer: "b", flagged_for_review: false }],
          current_position: 0,
        },
        { onSuccess: () => { secondDone = true; } },
      );
    });

    // Only the first request's PATCH may have been dispatched — the second
    // must still be waiting on the chain.
    await waitFor(() => {
      expect(mockAuthFetch).toHaveBeenCalledTimes(1);
    });
    expect(firstDone).toBe(false);
    expect(secondDone).toBe(false);

    // Resolving the first unblocks the second.
    await act(async () => {
      resolveFirst?.();
    });
    await waitFor(() => {
      expect(mockAuthFetch).toHaveBeenCalledTimes(2);
    });
    await waitFor(() => {
      expect(secondDone).toBe(true);
    });
  });

  // A rejected first save must not wedge the chain — the second save (the
  // freshest state, since buildSavePayload always sends a full snapshot in
  // page.tsx) still goes out and still resolves normally. Note: React
  // Query's useMutation only guarantees per-call onSuccess/onError for the
  // MOST RECENTLY issued mutate() call on a given hook instance — calling
  // mutate() again detaches the previous call's observer, so an older
  // call's callbacks are not asserted here. page.tsx's own sequence
  // guard (attemptSave/saveSeqRef) is what makes the app correct regardless
  // of that detail; this test only pins down the network-level contract.
  it("a rejected first save does not block the second from being sent and settling (Blocker 3, FR-32)", async () => {
    mockAuthFetch.mockImplementationOnce(() => Promise.reject(new Error("network error")));
    mockAuthFetch.mockImplementationOnce(() => Promise.resolve());

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useSaveAnswers("session-1"), {
      wrapper,
    });

    let secondDone = false;

    act(() => {
      result.current.mutate({
        answers: [{ question_id: "q1", answer: "a", flagged_for_review: false }],
        current_position: 0,
      });
    });
    act(() => {
      result.current.mutate(
        {
          answers: [{ question_id: "q1", answer: "b", flagged_for_review: false }],
          current_position: 0,
        },
        { onSuccess: () => { secondDone = true; } },
      );
    });

    await waitFor(() => {
      expect(secondDone).toBe(true);
    });
    expect(mockAuthFetch).toHaveBeenCalledTimes(2);
  });
});

// Issue #94: a successful autosave used to invalidate examKeys.session(id),
// which refetches the whole paper (4 + 6T queries via ReconnectSession) on
// every debounced keystroke. These tests pin the fix at the network level —
// a mounted useReconnectSession observer sharing the QueryClient is the
// thing that proves whether a refetch actually fired.
describe("useSaveAnswers onSuccess and the mounted session query (issue #94)", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  it("a successful save issues exactly one network call (the PATCH) and does not refetch the mounted session query (FR-1)", async () => {
    mockAuthFetch.mockImplementation(() => Promise.resolve({}));

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(
      () => ({
        reconnect: useReconnectSession("session-1"),
        save: useSaveAnswers("session-1"),
      }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.reconnect.isSuccess).toBe(true);
    });
    mockAuthFetch.mockClear();

    await act(async () => {
      result.current.save.mutate({
        answers: [{ question_id: "q1", answer: "a", flagged_for_review: false }],
        current_position: 0,
      });
    });
    await waitFor(() => {
      expect(result.current.save.isSuccess).toBe(true);
    });
    // Flush any microtask a re-added invalidateQueries would have kicked
    // off alongside the onSuccess callback.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(mockAuthFetch).toHaveBeenCalledTimes(1);
    const [url, init] = mockAuthFetch.mock.calls[0];
    expect(init).toMatchObject({ method: "PATCH" });
    expect(url).toContain("/exam/sessions/session-1/answers");
  });

  it("submit still refetches the mounted session query (FR-3 control)", async () => {
    mockAuthFetch.mockImplementation(() => Promise.resolve({}));

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(
      () => ({
        reconnect: useReconnectSession("session-1"),
        submit: useSubmitSession("session-1"),
      }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.reconnect.isSuccess).toBe(true);
    });
    mockAuthFetch.mockClear();

    await act(async () => {
      result.current.submit.mutate();
    });
    await waitFor(() => {
      expect(result.current.submit.isSuccess).toBe(true);
    });

    await waitFor(() => {
      expect(mockAuthFetch.mock.calls.some(isGetCall)).toBe(true);
    });
  });

  it("advance-section still refetches the mounted session query (FR-4 control)", async () => {
    mockAuthFetch.mockImplementation(() => Promise.resolve({}));

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(
      () => ({
        reconnect: useReconnectSession("session-1"),
        advance: useAdvanceSection("session-1"),
      }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.reconnect.isSuccess).toBe(true);
    });
    mockAuthFetch.mockClear();

    await act(async () => {
      result.current.advance.mutate("test-1");
    });
    await waitFor(() => {
      expect(result.current.advance.isSuccess).toBe(true);
    });

    await waitFor(() => {
      expect(mockAuthFetch.mock.calls.some(isGetCall)).toBe(true);
    });
  });
});
