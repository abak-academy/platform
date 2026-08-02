import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useSaveAnswers } from "./exam";

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
