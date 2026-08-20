import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent, act } from "@testing-library/react";
import ExamPackagesPage from "./page";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

let uiStore = { lang: "id" as "id" | "en" };

let examsState = {
  data: undefined as { data: unknown[]; next_cursor?: string } | undefined,
  isLoading: true,
  isError: false,
  error: null as Error | null,
};

const useExamsSpy = vi.fn();

vi.mock("@/lib/hooks/admin-exams", () => ({
  useExams: (...args: unknown[]) => {
    useExamsSpy(...args);
    return examsState;
  },
  useCreateExam: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateExam: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/stores/ui", () => ({
  useUIStore: (selector: (s: typeof uiStore) => unknown) => selector(uiStore),
}));

let mockRole: string | undefined = undefined;

vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: (s: { user: { role?: string } | null }) => unknown) =>
    sel({ user: mockRole ? { role: mockRole } : null }),
}));

const sampleExams = [
  {
    id: "e1",
    title: "UTS Matematika",
    scheduled_at: "2026-07-01T08:00:00Z",
    is_free: false,
    status: "draft",
    product_status: "draft",
    product_price: 50000,
    timer_mode: "overall",
    duration_minutes: 90,
    requires_checkin: true,
    allow_leaderboard: true,
    randomize: false,
    registration_count: 0,
  },
  {
    id: "e2",
    title: "UAS IPA",
    scheduled_at: "2026-07-15T09:00:00Z",
    is_free: true,
    status: "published",
    product_status: "published",
    product_price: 0,
    timer_mode: "per_test",
    duration_minutes: null,
    requires_checkin: false,
    allow_leaderboard: false,
    randomize: true,
    registration_count: 7,
  },
];

describe("ExamPackagesPage", () => {
  beforeEach(() => {
    push.mockReset();
    mockRole = undefined;
    useExamsSpy.mockClear();
  });

  it("navigates to the exam detail page when a row is clicked", async () => {
    examsState = {
      data: { data: sampleExams },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText("UTS Matematika")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("UTS Matematika"));
    expect(push).toHaveBeenCalledWith("/admin/exam/packages/e1");
  });

  it("renders the package card list with exam titles and the create button", async () => {
    examsState = {
      data: { data: sampleExams },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText("UTS Matematika")).toBeInTheDocument();
      expect(screen.getByText("UAS IPA")).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: /buat ujian/i })).toBeInTheDocument();
    expect(document.querySelector("table")).not.toBeInTheDocument();
  });

  it("renders one card row per package with scheduled date, timer mode, and status badge", async () => {
    examsState = {
      data: { data: sampleExams },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText("UTS Matematika")).toBeInTheDocument();
    });

    const rows = screen.getAllByTestId("package-row");
    expect(rows).toHaveLength(sampleExams.length);

    const row1 = screen.getByText("UTS Matematika").closest("[data-testid=package-row]") as HTMLElement;
    expect(within(row1).getByText("overall")).toBeInTheDocument();
    expect(within(row1).getByText("draft")).toBeInTheDocument();
    expect(within(row1).getByText(/2026/)).toBeInTheDocument();

    const row2 = screen.getByText("UAS IPA").closest("[data-testid=package-row]") as HTMLElement;
    expect(within(row2).getByText("published")).toBeInTheDocument();
  });

  it("navigates to the exam detail page on Enter key press", async () => {
    examsState = {
      data: { data: sampleExams },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText("UTS Matematika")).toBeInTheDocument();
    });

    const row = screen.getByText("UTS Matematika").closest("[data-testid=package-row]") as HTMLElement;
    fireEvent.keyDown(row, { key: "Enter" });
    expect(push).toHaveBeenCalledWith("/admin/exam/packages/e1");
  });

  it("hides the create button for admin_school (browse-only access to place bulk orders)", async () => {
    mockRole = "admin_school";
    examsState = {
      data: { data: sampleExams },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText("UTS Matematika")).toBeInTheDocument();
    });

    expect(screen.queryByRole("button", { name: /buat ujian/i })).not.toBeInTheDocument();
  });

  it("shows skeleton rows while loading", () => {
    examsState = {
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    expect(document.querySelectorAll("[data-slot=skeleton]").length).toBeGreaterThan(0);
  });

  it("surfaces an API error as inline error text", async () => {
    examsState = {
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat paket"),
      refetch: vi.fn(),
    } as any;

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat paket/i)).toBeInTheDocument();
    });
  });

  it("shows empty state when no packages exist", async () => {
    examsState = {
      data: { data: [] },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText(/belum ada ujian/i)).toBeInTheDocument();
    });
  });

  it("renders registration_count for every card, including a zero count", async () => {
    examsState = {
      data: { data: sampleExams },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText("UTS Matematika")).toBeInTheDocument();
    });

    const row1 = screen.getByText("UTS Matematika").closest("[data-testid=package-row]") as HTMLElement;
    expect(within(row1).getByText("0 peserta terdaftar")).toBeInTheDocument();

    const row2 = screen.getByText("UAS IPA").closest("[data-testid=package-row]") as HTMLElement;
    expect(within(row2).getByText("7 peserta terdaftar")).toBeInTheDocument();
  });

  it("fetches /admin/exams with no query string when every control is at its default", async () => {
    examsState = {
      data: { data: sampleExams },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText("UTS Matematika")).toBeInTheDocument();
    });

    expect(useExamsSpy).toHaveBeenCalledWith({ q: undefined, status: undefined });
  });

  it("selecting draft requests status=draft, and selecting all back never sends the literal string 'all'", async () => {
    examsState = {
      data: { data: sampleExams },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText("UTS Matematika")).toBeInTheDocument();
    });

    const select = screen.getByRole("combobox") as HTMLSelectElement;
    fireEvent.change(select, { target: { value: "draft" } });
    expect(useExamsSpy).toHaveBeenLastCalledWith({ q: undefined, status: "draft" });

    fireEvent.change(select, { target: { value: "" } });
    expect(useExamsSpy).toHaveBeenLastCalledWith({ q: undefined, status: undefined });
    expect(useExamsSpy.mock.calls.some((call) => call[0]?.status === "all")).toBe(false);
  });

  it("debounces and trims the search box before it reaches the request filters", async () => {
    vi.useFakeTimers();
    examsState = {
      data: { data: sampleExams },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);

    await act(async () => {
      await Promise.resolve();
    });
    useExamsSpy.mockClear();

    const search = screen.getByPlaceholderText("Cari…");
    fireEvent.change(search, { target: { value: "  matematika  " } });

    expect(useExamsSpy).not.toHaveBeenCalledWith(
      expect.objectContaining({ q: "matematika" })
    );

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(useExamsSpy).toHaveBeenLastCalledWith({ q: "matematika", status: undefined });

    vi.useRealTimers();
  });
});