import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AssessmentTab } from "./AssessmentTab";
import type { AssessmentResponse, School } from "@/lib/types";

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
  average_score: 88.25,
  distribution: [],
  violation_attempts: 1,
  violation_events: 2,
};

const page1: AssessmentResponse = {
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
      score: 88.25,
      status: "completed",
      attempts_count: 2,
      latest_session_id: "session-latest",
      latest_attempt_number: 2,
      latest_submitted_at: "2026-08-20T00:00:00Z",
      latest_violations: 2,
    },
  ],
};

const page2: AssessmentResponse = {
  summary,
  next_cursor: "",
  data: [
    page1.data[0],
    {
      registration_id: "reg-2",
      student_id: "student-2",
      student_name: "Siti Belum Mulai",
      username: "siti",
      school_id: "school-1",
      school_name: "SMAN 1",
      rank: null,
      score: null,
      status: "not_started",
      attempts_count: 0,
      latest_session_id: null,
      latest_attempt_number: null,
      latest_submitted_at: null,
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
      <AssessmentTab examId="exam-1" />
    </QueryClientProvider>,
  );
}

describe("AssessmentTab", () => {
  beforeEach(() => {
    vi.useRealTimers();
    mockAuthFetch.mockReset();
    toastError.mockReset();
    mockAuthFetch.mockImplementation((url: string) => {
      if (url === "/schools") return Promise.resolve(schools);
      if (url.startsWith("/admin/exams/exam-1/assessment/reg-1/attempts")) {
        return Promise.resolve({
          data: [
            {
              session_id: "session-latest",
              attempt_number: 2,
              status: "in_progress",
              submitted_at: null,
              score: null,
              violations: 2,
              result_available: false,
              is_latest: true,
            },
            {
              session_id: "session-old",
              attempt_number: 1,
              status: "submitted",
              submitted_at: "2026-08-20T00:00:00Z",
              score: 88.25,
              violations: 0,
              result_available: true,
              is_latest: false,
            },
          ],
        });
      }
      if (url.startsWith("/admin/results/session-old")) {
        return Promise.resolve({
          session_id: "session-old",
          student_name: "Budi Santoso",
          username: "budi",
          score: 88.25,
          submitted_at: "2026-08-20T00:00:00Z",
          result_config: "score_only",
          correct_count: 8,
          wrong_count: 1,
          empty_count: 1,
          breakdown: [],
        });
      }
      if (url.startsWith("/admin/exams/exam-1/assessment")) return Promise.resolve(page1);
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
    expect(mockAuthFetch.mock.calls.filter(([url]) => String(url).includes("/assessment")).length).toBe(0);

    await new Promise((resolve) => setTimeout(resolve, 100));
    await waitFor(() => {
      expect(mockAuthFetch.mock.calls.some(([url]) => String(url).includes("q=budi"))).toBe(true);
    });
    expect(mockAuthFetch.mock.calls.filter(([url]) => String(url).includes("/assessment") && String(url).includes("q=")).length).toBe(1);
  });

  it("accumulates unique rows and keeps summary mounted while loading more", async () => {
    const page2Deferred = deferred<AssessmentResponse>();
    mockAuthFetch.mockImplementation((url: string) => {
      if (url === "/schools") return Promise.resolve(schools);
      if (url.includes("cursor=cursor-2")) return page2Deferred.promise;
      if (url.startsWith("/admin/exams/exam-1/assessment")) return Promise.resolve(page1);
      return Promise.reject(new Error(`Unhandled authFetch ${url}`));
    });

    renderTab();
    await screen.findByText("Budi Santoso");
    expect(screen.getAllByText("88.3").length).toBeGreaterThan(0);
    expect(screen.getByText((content) => content.replace(/\s/g, "") === "50%")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /muat lebih banyak/i }));
    expect(screen.getByText((content) => content.replace(/\s/g, "") === "50%")).toBeInTheDocument();

    await act(async () => {
      page2Deferred.resolve(page2);
    });
    await screen.findByText("Siti Belum Mulai");
    expect(screen.getAllByText("Budi Santoso")).toHaveLength(1);
  });

  it("opens the drawer, maps attempt statuses, and loads eligible attempt details", async () => {
    renderTab();
    const row = await screen.findByText("Budi Santoso");
    fireEvent.click(within(row.closest("tr") as HTMLElement).getByRole("button", { name: /lihat/i }));

    await screen.findByText(/Percobaan 2/);
    expect(screen.getByText(/Berlangsung|In Progress/)).toBeInTheDocument();
    expect(screen.getByText(/Belum tersedia|Not available/i)).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole("button", { name: /lihat/i })[0]);
    await waitFor(() => {
      expect(screen.getByText("8")).toBeInTheDocument();
      expect(mockAuthFetch).toHaveBeenCalledWith("/admin/results/session-old");
    });
  });
});
