import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useGradingSessions,
  useSessionEssays,
  useGradeEssay,
  useExamRoster,
  adminExamsKeys,
} from "./admin-exams";
import { examKeys } from "./exam";
import type { GradingSessionItem, GradingEssayItem, ExamRosterEntry } from "@/lib/types";

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

describe("admin-exams grading hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("useGradingSessions fetches GET /admin/exams/:id/grading", async () => {
    const items: GradingSessionItem[] = [
      {
        session_id: "s1",
        student_id: "u1",
        student_name: "Budi",
        submitted_at: "2026-06-30T10:00:00Z",
        ungraded_essay_count: 2,
      },
    ];
    mockAuthFetch.mockResolvedValueOnce({ data: items });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useGradingSessions("e1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/e1/grading");
    expect(result.current.data?.data).toEqual(items);
  });

  it("useSessionEssays fetches GET /admin/sessions/:id/essays", async () => {
    const items: GradingEssayItem[] = [
      {
        question_id: "q1",
        body: "Explain X",
        answer: "My answer",
        point_correct: 5,
        score: null,
        grader_comment: null,
        graded_at: null,
      },
    ];
    mockAuthFetch.mockResolvedValueOnce({ data: items });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useSessionEssays("s1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/sessions/s1/essays");
    expect(result.current.data?.data).toEqual(items);
  });

  it("useGradeEssay posts to /admin/sessions/:id/grade and invalidates essays, grading list, and session result", async () => {
    mockAuthFetch.mockResolvedValueOnce({ status: "ok", score: 7 });

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useGradeEssay("s1"), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ question_id: "q1", score: 4, comment: "Good" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/sessions/s1/grade", {
      method: "POST",
      body: JSON.stringify({ question_id: "q1", score: 4, comment: "Good" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminExamsKeys.sessionEssays("s1") });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminExamsKeys.gradingLists() });
    expect(spy).toHaveBeenCalledWith({ queryKey: examKeys.result("s1") });
  });

  it("useExamRoster fetches GET /admin/exams/:id/registrations", async () => {
    const rows: ExamRosterEntry[] = [
      {
        registration_id: "r1",
        student_id: "s1",
        student_name: "Andi Saputra",
        student_username: "andi123",
        participant_number: 1,
        participant_no: "250620-0042-000001",
        status: "registered",
        checked_in_at: null,
      },
    ];
    mockAuthFetch.mockResolvedValueOnce({ data: rows });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useExamRoster("e1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/e1/registrations");
    expect(result.current.data?.data).toEqual(rows);
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
