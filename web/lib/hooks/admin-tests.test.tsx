import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAdminTests,
  useTestDetail,
  useCreateTest,
  useUpdateTest,
  useDeleteTest,
  useTestQuestions,
  useSaveQuestion,
  useDeleteQuestion,
  useAttachQuestions,
  useDetachQuestion,
  useReorderTestQuestions,
  adminTestsKeys,
} from "./admin-tests";
import { adminBankQuestionsKeys } from "./admin-bank-questions";

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

describe("admin-tests hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("useAdminTests_returns_data_when_API_responds", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      data: [
        {
          id: "t1",
          title: "Tryout 1",
          subject: "math",
          topic: "algebra",
          duration_minutes: 60,
        },
      ],
      next_cursor: "",
    });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminTests(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests");
    expect(result.current.data?.data).toEqual([
      {
        id: "t1",
        title: "Tryout 1",
        subject: "math",
        topic: "algebra",
        duration_minutes: 60,
      },
    ]);
  });

  it("useAdminTests_uses_query_params_when_filters_provided", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: "" });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(
      () => useAdminTests({ subject: "math", topic: "algebra", limit: 5 }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith(
      expect.stringContaining("/admin/tests?"),
    );
    const called = mockAuthFetch.mock.calls[0][0] as string;
    expect(called).toContain("subject=math");
    expect(called).toContain("topic=algebra");
    expect(called).toContain("limit=5");
  });

  it("useAdminTests_includes_q_in_query_params", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: "" });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminTests({ q: "tryout" }), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests?q=tryout");
  });

  it("useTestDetail_fetches_with_given_id", async () => {
    const detail = {
      test: {
        id: "t1",
        title: "Tryout 1",
        subject: "math",
        topic: "algebra",
        duration_minutes: 60,
      },
      questions: [],
    };
    mockAuthFetch.mockResolvedValueOnce(detail);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useTestDetail("t1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests/t1");
    expect(result.current.data).toEqual(detail);
  });

  it("useCreateTest_calls_POST_and_invalidates_query", async () => {
    const test = {
      id: "t2",
      title: "Tryout 2",
      subject: "math",
      topic: "algebra",
      duration_minutes: 90,
    };
    mockAuthFetch.mockResolvedValueOnce(test);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useCreateTest(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        title: "Tryout 2",
        subject: "math",
        topic: "algebra",
        duration_minutes: 90,
      });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests", {
      method: "POST",
      body: JSON.stringify({
        title: "Tryout 2",
        subject: "math",
        topic: "algebra",
        duration_minutes: 90,
      }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminTestsKeys.list() });
  });

  it("useUpdateTest_calls_PATCH_with_id", async () => {
    const test = {
      id: "t1",
      title: "Tryout 1 v2",
      subject: "math",
      topic: "algebra",
      duration_minutes: 75,
    };
    mockAuthFetch.mockResolvedValueOnce(test);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateTest("t1"), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ title: "Tryout 1 v2", duration_minutes: 75 });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests/t1", {
      method: "PATCH",
      body: JSON.stringify({ title: "Tryout 1 v2", duration_minutes: 75 }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminTestsKeys.list() });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminTestsKeys.detail("t1") });
  });

  it("useDeleteTest_calls_DELETE_with_id", async () => {
    mockAuthFetch.mockResolvedValueOnce(undefined);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useDeleteTest("t1"), { wrapper });

    await act(async () => {
      await result.current.mutateAsync();
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests/t1", {
      method: "DELETE",
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminTestsKeys.list() });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminTestsKeys.detail("t1") });
  });

  it("useTestQuestions_fetches_with_test_id", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      data: [
        {
          question: {
            id: "q1",
            topic_id: null,
            topic: null,
            format: "mcq",
            body: "What is 1+1?",
            sort_order: 1,
          },
          options: [
            {
              question_id: "q1",
              key: "a",
              text: "1",
              is_correct: false,
              sort_order: 1,
            },
          ],
        },
      ],
      next_cursor: "",
    });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useTestQuestions("t1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests/t1/questions");
    expect(result.current.data?.data).toHaveLength(1);
  });

  it("useSaveQuestion_routes_to_POST_when_no_id", async () => {
    const created = {
      question: {
        id: "q2",
        test_id: "t1",
        format: "mcq",
        body: "New Q",
        sort_order: 1,
      },
      options: [],
    };
    mockAuthFetch.mockResolvedValueOnce(created);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useSaveQuestion("t1"), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        input: { format: "mcq", body: "New Q" },
      });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests/t1/questions", {
      method: "POST",
      body: JSON.stringify({ format: "mcq", body: "New Q" }),
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminTestsKeys.questions("t1"),
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminBankQuestionsKeys.lists(),
    });
  });

  it("useSaveQuestion_routes_to_PATCH_when_id_present", async () => {
    const updated = {
      question: {
        id: "q1",
        test_id: "t1",
        format: "mcq",
        body: "Edited Q",
        sort_order: 1,
      },
      options: [],
    };
    mockAuthFetch.mockResolvedValueOnce(updated);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useSaveQuestion("t1"), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        question: "q1",
        input: { format: "mcq", body: "Edited Q" },
      });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/questions/q1", {
      method: "PATCH",
      body: JSON.stringify({ format: "mcq", body: "Edited Q" }),
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminTestsKeys.questions("t1"),
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminBankQuestionsKeys.lists(),
    });
  });

  it("useDeleteQuestion_calls_DELETE", async () => {
    mockAuthFetch.mockResolvedValueOnce(undefined);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useDeleteQuestion("t1"), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("q1");
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/questions/q1", {
      method: "DELETE",
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminTestsKeys.questions("t1"),
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminBankQuestionsKeys.lists(),
    });
  });

  it("useAttachQuestions_calls_POST_attach", async () => {
    mockAuthFetch.mockResolvedValueOnce(undefined);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useAttachQuestions("t1"), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ question_ids: ["q1", "q2"] });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests/t1/questions/attach", {
      method: "POST",
      body: JSON.stringify({ question_ids: ["q1", "q2"] }),
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminTestsKeys.questions("t1"),
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminBankQuestionsKeys.lists(),
    });
  });

  it("useDetachQuestion_calls_DELETE_join", async () => {
    mockAuthFetch.mockResolvedValueOnce(undefined);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useDetachQuestion("t1"), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("q1");
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests/t1/questions/q1", {
      method: "DELETE",
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminTestsKeys.questions("t1"),
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminBankQuestionsKeys.lists(),
    });
  });

  it("useReorderTestQuestions_calls_PUT_order", async () => {
    mockAuthFetch.mockResolvedValueOnce(undefined);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useReorderTestQuestions("t1"), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ question_ids: ["q2", "q1"] });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/tests/t1/questions/order", {
      method: "PUT",
      body: JSON.stringify({ question_ids: ["q2", "q1"] }),
    });
    expect(spy).toHaveBeenCalledWith({
      queryKey: adminTestsKeys.questions("t1"),
    });
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