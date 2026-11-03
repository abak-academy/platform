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

let examsQuery: ((filter?: Record<string, unknown>) => typeof examsState) | undefined;

const useExamsSpy = vi.fn();

vi.mock("@/lib/hooks/admin-exams", () => ({
  useExams: (...args: unknown[]) => {
    useExamsSpy(...args);
    return examsQuery?.(args[0] as Record<string, unknown> | undefined) ?? examsState;
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
    scheduled_end_at: "2026-07-01T10:00:00Z",
    is_free: false,
    status: "draft",
    format_version: 2,
    navigation_policy: "free",
    timer_mode: "overall",
    duration_minutes: 90,
    requires_checkin: true,
    allow_leaderboard: true,
    randomize: false,
    registration_count: 0,
    section_count: 2,
    question_count: 30,
    sections_duration_minutes: 75,
  },
  {
    id: "e2",
    title: "UAS IPA",
    scheduled_at: "2026-07-15T09:00:00Z",
    is_free: true,
    status: "published",
    format_version: 2,
    navigation_policy: "sequential",
    timer_mode: "per_test",
    duration_minutes: null,
    requires_checkin: false,
    allow_leaderboard: false,
    randomize: true,
    registration_count: 7,
    section_count: 3,
    question_count: 45,
    sections_duration_minutes: 105,
  },
  {
    id: "e3",
    title: "Tryout Lama",
    scheduled_at: null,
    is_free: false,
    status: "published",
    format_version: 1,
    mode: "utbk",
    timer_mode: "overall",
    duration_minutes: 60,
    requires_checkin: false,
    allow_leaderboard: false,
    randomize: false,
    registration_count: 28,
    section_count: 0,
    question_count: 0,
    sections_duration_minutes: null,
  },
];

describe("ExamPackagesPage", () => {
  beforeEach(() => {
    push.mockReset();
    mockRole = undefined;
    useExamsSpy.mockClear();
    examsQuery = undefined;
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

  it("renders a five-column table with exam titles and the create button", async () => {
    examsState = { data: { data: sampleExams }, isLoading: false, isError: false, error: null };

    render(<ExamPackagesPage />);

    await waitFor(() => {
      expect(screen.getByText("UTS Matematika")).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: /buat ujian/i })).toBeInTheDocument();
    const headers = screen.getAllByRole("columnheader").map((h) => h.textContent);
    expect(headers).toEqual(["Ujian", "Susunan", "Cara pengerjaan", "Waktu pengerjaan", "Jadwal akses"]);
  });

  it("shows composition, delivery, working time and access window for a composable exam", async () => {
    examsState = { data: { data: sampleExams }, isLoading: false, isError: false, error: null };

    render(<ExamPackagesPage />);
    await screen.findByText("UTS Matematika");

    const rows = screen.getAllByTestId("package-row");
    expect(rows).toHaveLength(sampleExams.length);

    const overall = rows[0];
    expect(within(overall).getByText("2 section · 30 soal")).toBeInTheDocument();
    expect(within(overall).getByText("Bebas")).toBeInTheDocument();
    expect(within(overall).getByText("Tampilan soal diatur per section")).toBeInTheDocument();
    expect(within(overall).getByText("90 menit total")).toBeInTheDocument();
    expect(within(overall).getByText("Timer seluruh ujian")).toBeInTheDocument();
    expect(within(overall).getByText(/2026.*–.*2026/)).toBeInTheDocument();
    expect(within(overall).getByText("0 peserta terdaftar")).toBeInTheDocument();

    const perTest = rows[1];
    expect(within(perTest).getByText("3 section · 45 soal")).toBeInTheDocument();
    expect(within(perTest).getByText("Berurutan")).toBeInTheDocument();
    expect(within(perTest).getByText("Total 105 menit")).toBeInTheDocument();
    expect(within(perTest).getByText("Timer per section")).toBeInTheDocument();
    expect(within(perTest).getByText("7 peserta terdaftar")).toBeInTheDocument();
  });

  it("shows the previous-format label and legacy mode for a format 1 exam", async () => {
    examsState = { data: { data: sampleExams }, isLoading: false, isError: false, error: null };

    render(<ExamPackagesPage />);
    await screen.findByText("Tryout Lama");

    const legacy = screen.getAllByTestId("package-row")[2];
    expect(within(legacy).getAllByText("Format sebelumnya").length).toBeGreaterThan(0);
    expect(within(legacy).getByText("UTBK")).toBeInTheDocument();
    expect(within(legacy).getByText("60 menit total")).toBeInTheDocument();
    expect(within(legacy).queryByText(/section ·/)).not.toBeInTheDocument();
  });

  it("shows a dash when working time is unknown", async () => {
    examsState = {
      data: { data: [{ ...sampleExams[1], id: "e9", title: "Tanpa Section", sections_duration_minutes: null }] },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackagesPage />);
    const row = (await screen.findByText("Tanpa Section")).closest("[data-testid=package-row]") as HTMLElement;

    const cells = within(row).getAllByRole("cell");
    expect(cells[3]).toHaveTextContent("—");
  });

  it("renders no status badge, no status filter, and the products footnote", async () => {
    examsState = { data: { data: sampleExams }, isLoading: false, isError: false, error: null };

    render(<ExamPackagesPage />);
    await screen.findByText("UTS Matematika");

    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    for (const row of screen.getAllByTestId("package-row")) {
      expect(within(row).queryByText(/^(draft|published)$/)).not.toBeInTheDocument();
    }
    expect(screen.getByText(/dikelola melalui Produk/i)).toBeInTheDocument();
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

  it("fetches /admin/exams without ever sending a status filter", async () => {
    examsState = { data: { data: sampleExams }, isLoading: false, isError: false, error: null };

    render(<ExamPackagesPage />);
    await screen.findByText("UTS Matematika");

    expect(useExamsSpy).toHaveBeenCalledWith({ cursor: undefined, limit: 20, q: undefined });
    expect(useExamsSpy.mock.calls.every((call) => !("status" in (call[0] ?? {})))).toBe(true);
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

    expect(useExamsSpy).toHaveBeenLastCalledWith({
      cursor: undefined,
      limit: 20,
      q: "matematika",
    });

    vi.useRealTimers();
  });

  it("loads next_cursor pages and deduplicates exams by id", async () => {
    const firstPage = {
      data: { data: sampleExams, next_cursor: "cursor-2" },
      isLoading: false,
      isError: false,
      error: null,
    };
    const secondPage = {
      data: { data: [sampleExams[1], { ...sampleExams[0], id: "e4", title: "Tryout TKA" }] },
      isLoading: false,
      isError: false,
      error: null,
    };
    examsQuery = (filter) => filter?.cursor ? secondPage : firstPage;

    render(<ExamPackagesPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Muat lebih" }));

    await waitFor(() => expect(screen.getByText("Tryout TKA")).toBeInTheDocument());
    expect(screen.getAllByTestId("package-row")).toHaveLength(4);
    expect(useExamsSpy).toHaveBeenLastCalledWith({
      cursor: "cursor-2",
      limit: 20,
      q: undefined,
    });
  });

  it("resets accumulated pages when the debounced search changes and when it is cleared", async () => {
    const firstPage = {
      data: { data: sampleExams, next_cursor: "cursor-2" },
      isLoading: false,
      isError: false,
      error: null,
    };
    const secondPage = {
      data: { data: [{ ...sampleExams[0], id: "x3", title: "Tryout TKA" }] },
      isLoading: false,
      isError: false,
      error: null,
    };
    const searchPage = {
      data: { data: [{ ...sampleExams[0], id: "q1", title: "Tryout Fisika" }] },
      isLoading: false,
      isError: false,
      error: null,
    };
    examsQuery = (filter) => {
      if (filter?.q === "fisika") return searchPage;
      return filter?.cursor ? secondPage : firstPage;
    };

    render(<ExamPackagesPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Muat lebih" }));
    await waitFor(() => expect(screen.getByText("Tryout TKA")).toBeInTheDocument());

    vi.useFakeTimers();
    fireEvent.change(screen.getByPlaceholderText("Cari…"), { target: { value: "fisika" } });
    await act(async () => vi.advanceTimersByTime(300));
    expect(screen.getByText("Tryout Fisika")).toBeInTheDocument();
    expect(screen.queryByText("Tryout TKA")).not.toBeInTheDocument();
    expect(useExamsSpy).toHaveBeenLastCalledWith({ cursor: undefined, limit: 20, q: "fisika" });

    fireEvent.change(screen.getByPlaceholderText("Cari…"), { target: { value: "" } });
    await act(async () => vi.advanceTimersByTime(300));
    expect(useExamsSpy).toHaveBeenLastCalledWith({ cursor: undefined, limit: 20, q: undefined });
    expect(screen.queryByText("Tryout Fisika")).not.toBeInTheDocument();
    expect(screen.getByText("UTS Matematika")).toBeInTheDocument();

    vi.useRealTimers();
  });
});
