import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import SchoolReportsPage from "./page";
import type { Product, AdminResultRow, AdminResultDetail, School } from "@/lib/types";

const mockExport = vi.fn();

// Mock the exam products hook
const mockProducts = vi.fn();

// Mock admin results hooks
let resultsState = {
  data: null as { data: AdminResultRow[]; next_cursor?: string } | null,
  isLoading: false,
  isFetching: false,
  isError: false,
  error: null as Error | null,
};

let detailState = {
  data: null as AdminResultDetail | null,
  isLoading: false,
  isFetching: false,
  isError: false,
  error: null as Error | null,
};

// Auth store mock
let authStore: {
  token: string | null;
  user: { role?: string; name?: string } | null;
} = {
  token: "t",
  user: { role: "admin_school" },
};

// Schools mock
let schoolsState = {
  data: null as { data: School[]; next_cursor?: string } | null,
  isLoading: false,
  isError: false,
};

vi.mock("@/lib/hooks/products", () => ({
  useProducts: () => mockProducts(),
}));

vi.mock("@/lib/hooks/admin-results", () => ({
  useAdminResults: () => resultsState,
  useAdminResultDetail: (sessionId: string) => detailState,
  exportAdminResults: (...args: Parameters<typeof mockExport>) => mockExport(...args),
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof authStore) => unknown) => selector(authStore),
}));

vi.mock("@/lib/hooks/admin-schools", () => ({
  useSchoolOptions: () => schoolsState,
}));

const sampleExamProducts = [
  {
    id: "exam-1",
    type: "exam",
    name: "Tryout Matematika",
    price: 50000,
    status: "published",
  },
  {
    id: "exam-2",
    type: "exam",
    name: "Tryout Fisika",
    price: 0,
    status: "published",
  },
] as unknown as Product[];

const sampleResultRows: AdminResultRow[] = [
  {
    session_id: "s1",
    student_name: "Budi Santoso",
    username: "12345",
    score: 85,
    submitted_at: "2026-01-15T00:00:00Z",
  },
  {
    session_id: "s2",
    student_name: "Siti Aisyah",
    username: "67890",
    score: 92,
    submitted_at: "2026-02-20T00:00:00Z",
  },
];

const paginatedResponse = (rows: AdminResultRow[]) => ({
  data: rows,
  next_cursor: undefined,
});

const scoreOnlyDetail: AdminResultDetail = {
  session_id: "s1",
  student_name: "Budi Santoso",
  username: "12345",
  score: 85,
  submitted_at: "2026-01-15T00:00:00Z",
  result_config: "score_only",
  correct_count: 10,
  wrong_count: 2,
  empty_count: 1,
};

const scorePembahasanDetail: AdminResultDetail = {
  session_id: "s2",
  student_name: "Siti Aisyah",
  username: "67890",
  score: 92,
  submitted_at: "2026-02-20T00:00:00Z",
  result_config: "score_pembahasan",
  correct_count: 12,
  wrong_count: 1,
  empty_count: 0,
  breakdown: [
    { test_id: "t1", title: "Aljabar", subject: "Matematika", topic: "Aljabar", earned: 8, max: 10 },
  ],
  pembahasan: [
    {
      question_id: "q1",
      body: "Berapa 2+2?",
      format: "mcq",
      your_answer: "4",
      correct_answer: "4",
      is_correct: true,
      explanation: "2+2=4",
    },
  ],
};

describe("SchoolReportsPage", () => {
  beforeEach(() => {
    authStore = {
      token: "t",
      user: { role: "admin_school" },
    };
    schoolsState = {
      data: { data: [{ id: "s1", name: "SMAN 1 Jakarta" }, { id: "s2", name: "SMAN 2 Bandung" }], next_cursor: undefined },
      isLoading: false,
      isError: false,
    };
    mockProducts.mockReturnValue({ data: sampleExamProducts, isLoading: false });
    mockExport.mockReset();

    resultsState = {
      data: paginatedResponse(sampleResultRows),
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };

    detailState = {
      data: null,
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };

    URL.createObjectURL = vi.fn().mockReturnValue("blob:test-url");
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Test 1: Picker renders exam options from useProducts("exam") — mocked in test ──
  it("renders exam picker with options from useProducts", async () => {
    render(<SchoolReportsPage />);

    // Open the select dropdown to show options
    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);

    await waitFor(() => {
      expect(screen.getByText("Tryout Matematika")).toBeInTheDocument();
      expect(screen.getByText("Tryout Fisika")).toBeInTheDocument();
    });
  });

  // ── Test 2: Table renders rows, search updates query, "load more" appends without duplicating ──
  it("renders results table when an exam is selected", async () => {
    render(<SchoolReportsPage />);

    // Select an exam
    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);

    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
      expect(screen.getByText("Siti Aisyah")).toBeInTheDocument();
    });

    // Check Username and score columns
    expect(screen.getByText("12345")).toBeInTheDocument();
    expect(screen.getByText("67890")).toBeInTheDocument();
    expect(screen.getByText("85")).toBeInTheDocument();
    expect(screen.getByText("92")).toBeInTheDocument();
  });

  it("shows load more button when next_cursor exists", async () => {
    resultsState = {
      data: { data: sampleResultRows, next_cursor: "cursor-next" },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };

    render(<SchoolReportsPage />);

    // Select an exam
    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);

    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    await waitFor(() => {
      expect(screen.getByText("Muat lebih")).toBeInTheDocument();
    });
  });

  it("clicking load more does not crash", async () => {
    resultsState = {
      data: { data: sampleResultRows, next_cursor: "cursor-2" },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };

    render(<SchoolReportsPage />);

    // Select exam
    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);
    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });

    // Click "load more" — should not crash
    const loadMoreBtn = screen.getByText("Muat lebih");
    fireEvent.click(loadMoreBtn);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
      expect(screen.getByText("Siti Aisyah")).toBeInTheDocument();
    });
  });

  it("search input updates query", async () => {
    render(<SchoolReportsPage />);

    // Select an exam
    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);
    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    // Find search input
    const searchInput = screen.getByPlaceholderText(/cari/i);
    expect(searchInput).toBeInTheDocument();

    fireEvent.input(searchInput, { target: { value: "Budi" } });

    await waitFor(() => {
      expect(searchInput).toHaveValue("Budi");
    });
  });

  // ── Test 3: Mocked empty list renders neutral empty state, NOT an error banner ──
  it("renders neutral empty state when results list is empty", async () => {
    resultsState = {
      data: { data: [], next_cursor: undefined },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };

    render(<SchoolReportsPage />);

    // Select an exam
    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);
    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    await waitFor(() => {
      expect(screen.getByText(/belum ada hasil/i)).toBeInTheDocument();
    });

    // Verify it's NOT an error banner
    expect(screen.queryByText(/gagal/i)).not.toBeInTheDocument();
  });

  // ── Test 4: Drill-down dialog shows breakdown/pembahasan only for score_pembahasan ──
  it("shows score_only detail in drill-down dialog (no breakdown/pembahasan)", async () => {
    detailState = {
      data: scoreOnlyDetail,
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };

    render(<SchoolReportsPage />);

    // Select exam
    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);
    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });

    // Click the row's view control to drill down
    const row = screen.getByText("Budi Santoso").closest("tr") as HTMLElement;
    fireEvent.click(within(row).getByRole("button", { name: /lihat/i }));

    await waitFor(() => {
      expect(screen.getByText("Detail Hasil")).toBeInTheDocument();
    });

    // For score_only: no breakdown section, no pembahasan section
    expect(screen.queryByText("Berdasarkan Topik")).not.toBeInTheDocument();
    expect(screen.queryByText("Pembahasan")).not.toBeInTheDocument();

    // Close dialog
    const closeBtn = screen.getByRole("button", { name: /batal/i });
    fireEvent.click(closeBtn);
  });

  it("shows breakdown and pembahasan for score_pembahasan drill-down", async () => {
    detailState = {
      data: scorePembahasanDetail,
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };

    render(<SchoolReportsPage />);

    // Select exam
    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);
    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    await waitFor(() => {
      expect(screen.getByText("Siti Aisyah")).toBeInTheDocument();
    });

    // Click the row's view control
    const row = screen.getByText("Siti Aisyah").closest("tr") as HTMLElement;
    fireEvent.click(within(row).getByRole("button", { name: /lihat/i }));

    await waitFor(() => {
      expect(screen.getByText("Detail Hasil")).toBeInTheDocument();
    });

    // score_pembahasan: breakdown and pembahasan should be visible
    expect(screen.getByText("Berdasarkan Topik")).toBeInTheDocument();
    expect(screen.getByText("Pembahasan")).toBeInTheDocument();
  });

  it("uppercases mcq option keys in pembahasan drill-down", async () => {
    detailState = {
      data: {
        ...scorePembahasanDetail,
        pembahasan: [
          {
            question_id: "q1",
            body: "Ibu kota?",
            format: "mcq",
            your_answer: "b",
            correct_answer: "a",
            is_correct: false,
          },
        ],
      },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };

    render(<SchoolReportsPage />);

    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);
    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    await waitFor(() => {
      expect(screen.getByText("Siti Aisyah")).toBeInTheDocument();
    });
    const row = screen.getByText("Siti Aisyah").closest("tr") as HTMLElement;
    fireEvent.click(within(row).getByRole("button", { name: /lihat/i }));

    await waitFor(() => {
      expect(screen.getByText("Detail Hasil")).toBeInTheDocument();
    });
    expect(screen.getByText(/Jawaban Anda:\s*B/)).toBeInTheDocument();
    expect(screen.getByText(/Jawaban Benar:\s*A/)).toBeInTheDocument();
  });

  // ── Drill-down moved to a labelled view control, not the <tr> (6c2b592) ──
  it("opens the drill-down via the row's view control, and the <tr> carries no role/tabindex/click handler", async () => {
    render(<SchoolReportsPage />);

    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);
    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });

    const row = screen.getByText("Budi Santoso").closest("tr") as HTMLElement;
    expect(row.getAttribute("role")).toBeNull();
    expect(row.getAttribute("tabindex")).toBeNull();

    // Clicking the row itself (not the view control) must not open the dialog
    fireEvent.click(row);
    expect(screen.queryByText("Detail Hasil")).not.toBeInTheDocument();

    fireEvent.click(within(row).getByRole("button", { name: /lihat/i }));

    await waitFor(() => {
      expect(screen.getByText("Detail Hasil")).toBeInTheDocument();
    });
  });

  it("renders rich body in pembahasan drill-down via RichContent (Task 8 audit)", async () => {
    detailState = {
      data: {
        ...scorePembahasanDetail,
        pembahasan: [
          {
            question_id: "q-rich-school",
            body: "Hitung \\(x^2\\) dan buat <b>tebal</b>",
            format: "mcq",
            your_answer: "4",
            correct_answer: "4",
            is_correct: true,
            explanation: "Penjelasan polos \\(x^2\\). <b>tidak</b> dirich.",
          },
        ],
      },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };

    render(<SchoolReportsPage />);

    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);
    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    await waitFor(() => {
      expect(screen.getByText("Siti Aisyah")).toBeInTheDocument();
    });
    const row = screen.getByText("Siti Aisyah").closest("tr") as HTMLElement;
    fireEvent.click(within(row).getByRole("button", { name: /lihat/i }));

    const richNode = await waitFor(() => {
      const el = document.querySelector("[data-rich-content] .katex");
      if (!el) throw new Error("not yet");
      return el.closest("[data-rich-content]") as HTMLElement;
    });
    expect(richNode).not.toBeNull();
    const b = richNode.querySelector("b");
    expect(b).not.toBeNull();
    expect(b?.textContent).toBe("tebal");
    expect(richNode.textContent).not.toContain("\\(");
  });

  // ── Test 5: No rendered DOM contains substring "rank" — grep test ──
  it("does not render the word 'rank' anywhere in the DOM", async () => {
    render(<SchoolReportsPage />);

    // Select exam
    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);
    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    const html = document.body.innerHTML;
    expect(html.toLowerCase()).not.toContain("rank");
  });

  // ── Test 6: Export button calls export function with current examId, doesn't crash ──
  it("renders export button disabled when no exam selected", async () => {
    render(<SchoolReportsPage />);

    const exportBtn = screen.getByRole("button", { name: /ekspor/i });
    expect(exportBtn).toBeDisabled();
  });

  it("calls exportAdminResults with selected examId on export click", async () => {
    mockExport.mockResolvedValueOnce(undefined);

    render(<SchoolReportsPage />);

    // Select exam
    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);
    const examOption = await screen.findByText("Tryout Matematika");
    fireEvent.click(examOption);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });

    const exportBtn = screen.getByRole("button", { name: /ekspor/i });
    expect(exportBtn).not.toBeDisabled();

    fireEvent.click(exportBtn);

    expect(mockExport).toHaveBeenCalledWith("exam-1", undefined);
  });

  // ── School dropdown (Bug B) ──

  it("shows school dropdown for super_admin role", async () => {
    authStore = { token: "t", user: { role: "super_admin" } };

    render(<SchoolReportsPage />);

    // There should be at least 2 comboboxes: exam picker + school picker
    const comboboxes = screen.getAllByRole("combobox");
    expect(comboboxes.length).toBeGreaterThanOrEqual(2);
  });

  it("does not show school dropdown for admin_school role", async () => {
    authStore = { token: "t", user: { role: "admin_school" } };

    render(<SchoolReportsPage />);

    // Only the exam combobox should exist, no school combobox
    const comboboxes = screen.getAllByRole("combobox");
    expect(comboboxes.length).toBe(1);
  });
});
