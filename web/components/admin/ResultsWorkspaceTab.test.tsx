import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ResultsWorkspaceTab } from "./ResultsWorkspaceTab";
import type { ResultsWorkspaceResponse, School } from "@/lib/types";

const mockAuthFetch = vi.fn();
const toastError = vi.fn();

vi.mock("@/lib/api", () => ({
  authFetch: (...args: Parameters<typeof mockAuthFetch>) => mockAuthFetch(...args),
  API_BASE: "http://api.test/api/v1",
}));

vi.mock("sonner", () => ({
  toast: { error: (...args: Parameters<typeof toastError>) => toastError(...args) },
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: {
    getState: () => ({ token: "token-1" }),
  },
}));

const schools: School[] = [{ id: "school-1", name: "SMAN 1" }];

const summary = {
  total_registered: 2,
  completed_participants: 1,
  completion_rate: 0.5,
  average_score: 69,
  max_possible_score: 73,
  distribution: [
    { label: "0-20", count: 0 },
    { label: "21-40", count: 0 },
    { label: "41-60", count: 0 },
    { label: "61-80", count: 0 },
    { label: "81-100", count: 1 },
  ],
  violation_attempts: 1,
  violation_events: 2,
};

const page1: ResultsWorkspaceResponse = {
  summary,
  next_cursor: "cursor-2",
  data: [
    {
      registration_id: "reg-1",
      student_id: "student-1",
      student_name: "Budi Santoso",
      username: "budi",
      school_id: "school-1",
      school_name: "SMAN 1",
      rank: 2,
      score: 69,
      attempts_count: 2,
      latest_session_id: "session-latest",
      latest_attempt_number: 2,
      latest_submitted_at: "2026-08-20T00:00:00Z",
      latest_violations: 2,
    },
  ],
};

const page2: ResultsWorkspaceResponse = {
  summary,
  next_cursor: "",
  data: [
    page1.data[0],
    {
      registration_id: "reg-2",
      student_id: "student-2",
      student_name: "Siti Juara",
      username: "siti",
      school_id: "school-1",
      school_name: "SMAN 1",
      rank: 3,
      score: 75,
      attempts_count: 1,
      latest_session_id: "session-siti",
      latest_attempt_number: 1,
      latest_submitted_at: "2026-08-20T00:00:00Z",
      latest_violations: 0,
    },
  ],
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function renderTab() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ResultsWorkspaceTab examId="exam-1" />
    </QueryClientProvider>,
  );
}

describe("ResultsWorkspaceTab", () => {
  beforeEach(() => {
    vi.useRealTimers();
    mockAuthFetch.mockReset();
    toastError.mockReset();
    mockAuthFetch.mockImplementation((url: string) => {
      if (url === "/schools") return Promise.resolve(schools);
      if (url.startsWith("/admin/exams/exam-1/results-workspace/reg-1/attempts")) {
        return Promise.resolve({
          data: [
            {
              session_id: "session-latest",
              attempt_number: 2,
              status: "submitted",
              submitted_at: "2026-08-20T00:00:00Z",
              score: 69,
              violations: 2,
              result_available: true,
              is_latest: true,
            },
            {
              session_id: "session-old",
              attempt_number: 1,
              status: "in_progress",
              submitted_at: null,
              score: null,
              violations: 0,
              result_available: false,
              is_latest: false,
            },
          ],
        });
      }
      if (url.startsWith("/admin/exams/exam-1/results-workspace/sessions/session-latest")) {
        return Promise.resolve({
          session_id: "session-latest",
          student_name: "Budi Santoso",
          username: "budi",
          score: 69,
          submitted_at: "2026-08-20T00:00:00Z",
          result_config: "score_only",
          correct_count: 8,
          wrong_count: 1,
          empty_count: 1,
          breakdown: [{ test_id: "test-1", title: "Matematika", earned: 8, max: 10 }],
          pembahasan: [
            {
              question_id: "question-1",
              body: "2 + 2 = ?",
              format: "mcq",
              your_answer: "A",
              correct_answer: "B",
              is_correct: false,
              explanation: "Empat adalah jawaban yang benar.",
            },
          ],
        });
      }
      if (url.startsWith("/admin/exams/exam-1/results-workspace")) return Promise.resolve(page1);
      return Promise.reject(new Error(`Unhandled authFetch ${url}`));
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("debounces search before changing the backend query", async () => {
    renderTab();
    await screen.findByText("Budi Santoso");
    mockAuthFetch.mockClear();

    fireEvent.change(screen.getByPlaceholderText(/nama atau username/i), { target: { value: "b" } });
    fireEvent.change(screen.getByPlaceholderText(/nama atau username/i), { target: { value: "bu" } });
    fireEvent.change(screen.getByPlaceholderText(/nama atau username/i), { target: { value: "bud" } });
    fireEvent.change(screen.getByPlaceholderText(/nama atau username/i), { target: { value: "budi" } });

    await new Promise((resolve) => setTimeout(resolve, 250));
    expect(mockAuthFetch.mock.calls.filter(([url]) => String(url).includes("/results-workspace")).length).toBe(0);

    await new Promise((resolve) => setTimeout(resolve, 100));
    await waitFor(() => {
      expect(mockAuthFetch.mock.calls.some(([url]) => String(url).includes("q=budi"))).toBe(true);
    });
    expect(mockAuthFetch.mock.calls.filter(([url]) => String(url).includes("/results-workspace") && String(url).includes("q=")).length).toBe(1);
  });

  it("accumulates unique rows and keeps summary mounted while loading more", async () => {
    const page2Deferred = deferred<ResultsWorkspaceResponse>();
    mockAuthFetch.mockImplementation((url: string) => {
      if (url === "/schools") return Promise.resolve(schools);
      if (url.includes("cursor=cursor-2")) return page2Deferred.promise;
      if (url.startsWith("/admin/exams/exam-1/results-workspace")) return Promise.resolve(page1);
      return Promise.reject(new Error(`Unhandled authFetch ${url}`));
    });

    renderTab();
    await screen.findByText("Budi Santoso");
    expect(screen.getAllByText("94.5%").length).toBeGreaterThan(0);
    expect(screen.getAllByText("69.0 / 73").length).toBeGreaterThan(0);
    expect(screen.getByText(/Distribusi Skor|Score Distribution/i)).toBeInTheDocument();
    expect(screen.getByText("81-100%")).toBeInTheDocument();
    expect(screen.getByText((content) => content.replace(/\s/g, "") === "50%")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /muat lebih banyak/i }));
    expect(screen.getByText((content) => content.replace(/\s/g, "") === "50%")).toBeInTheDocument();

    await act(async () => {
      page2Deferred.resolve(page2);
    });
    await screen.findByText("Siti Juara");
    expect(screen.getAllByText("Budi Santoso")).toHaveLength(1);
  });

  it("expands a ranked result row and shows per-question answers", async () => {
    renderTab();
    const row = await screen.findByText("Budi Santoso");
    fireEvent.click(within(row.closest("tr") as HTMLElement).getByRole("button", { name: /lihat/i }));

    await waitFor(() => {
      expect(screen.getByText("8")).toBeInTheDocument();
      expect(screen.getAllByText("94.5%").length).toBeGreaterThan(1);
      expect(screen.getAllByText("69.0 / 73").length).toBeGreaterThan(1);
      expect(screen.getByText(/Percobaan 2/)).toBeInTheDocument();
      expect(screen.getByText(/Selesai|Completed/i)).toBeInTheDocument();
      expect(screen.queryByText(/Belum Mulai|Not started/i)).not.toBeInTheDocument();
      expect(screen.queryByText("2 + 2 = ?")).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /Matematika/i }));

    await waitFor(() => {
      expect(screen.getByText("2 + 2 = ?")).toBeInTheDocument();
      expect(screen.queryByText("Tiga")).not.toBeInTheDocument();
      expect(screen.queryByText("Empat")).not.toBeInTheDocument();
      expect(screen.getByText(/Jawaban Anda|Your answer/i)).toBeInTheDocument();
      expect(screen.getByText(/Jawaban Benar|Correct answer/i)).toBeInTheDocument();
      expect(screen.getByText("Empat adalah jawaban yang benar.")).toBeInTheDocument();
      expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/exam-1/results-workspace/sessions/session-latest");
    });
  });
});
