import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, within, fireEvent, act } from "@testing-library/react";
import { toast } from "sonner";
import TestsPage from "./page";
import type { Test, ExamTopic } from "@/lib/types";
import type { AdminTestsFilters } from "@/lib/hooks/admin-tests";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

const mockMutateAsync = vi.fn();

let testsState = {
  data: null as { data: Test[]; next_cursor?: string; total?: number } | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

let createState = { mutateAsync: mockMutateAsync, isPending: false };
let updateState = { mutateAsync: mockMutateAsync, isPending: false };
let deleteState = { mutateAsync: mockMutateAsync, isPending: false };

const mockUseAdminTests = vi.fn((_filters?: AdminTestsFilters) => testsState);

vi.mock("@/lib/hooks/admin-tests", () => ({
  useAdminTests: (filters?: AdminTestsFilters) => mockUseAdminTests(filters),
  useCreateTest: () => createState,
  useUpdateTest: () => updateState,
  useDeleteTest: () => deleteState,
}));

// Deliberately distinct from sampleTests' subject/topic strings below — the
// toolbar's own <option> text must not collide with row text in getByText queries.
const sampleTopics: ExamTopic[] = [
  { id: "topic-1", name: "Optik", subject: "Fisika" },
  { id: "topic-2", name: "Mekanika", subject: "Fisika" },
  { id: "topic-3", name: "Stoikiometri", subject: "Kimia" },
];

// useTopics() (no subject) drives the subject dropdown; useTopics(subject) re-scopes the topic one.
vi.mock("@/lib/hooks/admin-topics", () => ({
  useTopics: (subject?: string) => ({
    data: { data: subject ? sampleTopics.filter((t) => t.subject === subject) : sampleTopics },
    isLoading: false,
    isError: false,
    error: null,
  }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock("@/lib/hooks/admin-uploads", () => ({
  usePresignAdminAudioUpload: () => ({ mutateAsync: vi.fn(), isPending: false }),
  usePresignAdminImageUpload: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

const sampleTests: Test[] = [
  {
    id: "t1",
    title: "Tryout UTBK Saintek",
    subject: "Matematika",
    topic: "Aljabar",
    duration_minutes: 90,
  },
  {
    id: "t2",
    title: "Tryout Bahasa Inggris",
    subject: "Bahasa Inggris",
    topic: "Reading",
    duration_minutes: 60,
  },
];

describe("TestsPage", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    testsState = {
      data: { data: sampleTests },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    createState = { mutateAsync: mockMutateAsync, isPending: false };
    updateState = { mutateAsync: mockMutateAsync, isPending: false };
    deleteState = { mutateAsync: mockMutateAsync, isPending: false };
    mockMutateAsync.mockReset();
    mockUseAdminTests.mockClear();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
    push.mockReset();
  });

  it("renders the tests table with subject, topic, duration", async () => {
    render(<TestsPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout UTBK Saintek")).toBeInTheDocument();
      expect(screen.getByText("Tryout Bahasa Inggris")).toBeInTheDocument();
    });

    expect(screen.getByText("Matematika")).toBeInTheDocument();
    expect(screen.getByText("Aljabar")).toBeInTheDocument();
    expect(screen.getByText(/90\s*menit/)).toBeInTheDocument();
    expect(screen.getByText("Reading")).toBeInTheDocument();
    expect(screen.getByText(/60\s*menit/)).toBeInTheDocument();
  });

  it("navigates to the test detail page when a row is clicked", async () => {
    render(<TestsPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout UTBK Saintek")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Tryout UTBK Saintek"));
    expect(push).toHaveBeenCalledWith("/admin/exam/tests/t1");
  });

  it("does not navigate when clicking the edit or delete action buttons", async () => {
    render(<TestsPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout UTBK Saintek")).toBeInTheDocument();
    });

    const row = screen.getByText("Tryout UTBK Saintek").closest("[data-testid=test-row]") as HTMLElement;
    fireEvent.click(within(row).getByRole("button", { name: /^edit$/i }));
    expect(push).not.toHaveBeenCalled();
  });

  it("shows skeleton rows while loading", () => {
    testsState = {
      data: null,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<TestsPage />);

    expect(document.querySelectorAll("[data-slot=skeleton]").length).toBeGreaterThan(0);
  });

  it("surfaces an API error as inline error text", async () => {
    testsState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat tes"),
      refetch: vi.fn(),
    };

    render(<TestsPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat tes/i)).toBeInTheDocument();
    });
  });

  it("shows empty state when no tests exist", async () => {
    testsState = {
      data: { data: [] },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<TestsPage />);

    await waitFor(() => {
      expect(screen.getByText(/belum ada tes/i)).toBeInTheDocument();
    });
  });

  it("opens the create modal on New Test click and calls create mutation on save", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "t3", title: "Tryout Baru" });

    render(<TestsPage />);

    await waitFor(() => expect(screen.getByText("Tryout UTBK Saintek")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /tes baru/i }));

    expect(screen.getByRole("dialog", { name: /tes baru/i })).toBeInTheDocument();

    fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Tryout Baru" } });
    fireEvent.change(screen.getByLabelText(/mata pelajaran/i), { target: { value: "Fisika" } });
    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "Optik" } });
    fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "60" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Tryout Baru",
          subject: "Fisika",
          topic: "Optik",
          duration_minutes: 60,
        })
      );
      expect(toast.success).toHaveBeenCalledWith("Tes dibuat");
    });
  });

  it("opens the edit modal prefilled and calls update mutation on save", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "t1", title: "Tryout UTBK Saintek Revisi" });

    render(<TestsPage />);

    await waitFor(() => expect(screen.getByText("Tryout UTBK Saintek")).toBeInTheDocument());

    const row = screen.getByText("Tryout UTBK Saintek").closest("[data-testid=test-row]") as HTMLElement;
    expect(row).toBeTruthy();
    const editButton = within(row).getByRole("button", { name: /^edit$/i });
    fireEvent.click(editButton);

    expect(screen.getByRole("dialog", { name: /sunting tes/i })).toBeInTheDocument();
    expect(screen.getByDisplayValue("Tryout UTBK Saintek")).toBeInTheDocument();

    fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Tryout UTBK Saintek Revisi" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ title: "Tryout UTBK Saintek Revisi" })
      );
      expect(toast.success).toHaveBeenCalledWith("Tes diperbarui");
    });
  });

  it("deletes a test after confirm and shows a success toast", async () => {
    mockMutateAsync.mockResolvedValueOnce(undefined);
    vi.stubGlobal("confirm", () => true);

    render(<TestsPage />);

    await waitFor(() => expect(screen.getByText("Tryout Bahasa Inggris")).toBeInTheDocument());

    const row = screen.getByText("Tryout Bahasa Inggris").closest("[data-testid=test-row]") as HTMLElement;
    expect(row).toBeTruthy();
    const deleteButton = within(row).getByRole("button", { name: /hapus/i });
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith();
      expect(toast.success).toHaveBeenCalledWith("Tes dihapus");
    });

  });

  it("fetches with no filters when every control is at its default (FR-16)", async () => {
    render(<TestsPage />);

    await waitFor(() => expect(screen.getByText("Tryout UTBK Saintek")).toBeInTheDocument());

    expect(mockUseAdminTests).toHaveBeenLastCalledWith({
      subject: undefined,
      topic: undefined,
      q: undefined,
      cursor: undefined,
    });
  });

  it("sends q only after the debounce, trimmed", async () => {
    vi.useFakeTimers();
    render(<TestsPage />);
    mockUseAdminTests.mockClear();

    fireEvent.change(screen.getByTestId("tests-search-input"), {
      target: { value: "  aljabar  " },
    });
    expect(mockUseAdminTests).not.toHaveBeenCalledWith(
      expect.objectContaining({ q: "aljabar" })
    );

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockUseAdminTests).toHaveBeenLastCalledWith(
      expect.objectContaining({ q: "aljabar" })
    );

  });

  it("re-scopes the topic dropdown to the selected subject and clears a stale topic (FR-13)", async () => {
    render(<TestsPage />);
    await waitFor(() => expect(screen.getByText("Tryout UTBK Saintek")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /^filter/i }));
    const subjectSelect = screen.getByTestId("tests-subject-filter") as HTMLSelectElement;
    const topicSelect = screen.getByTestId("tests-topic-filter") as HTMLSelectElement;

    fireEvent.change(subjectSelect, { target: { value: "Kimia" } });
    fireEvent.change(topicSelect, { target: { value: "Stoikiometri" } });
    expect(topicSelect.value).toBe("Stoikiometri");

    fireEvent.change(subjectSelect, { target: { value: "Fisika" } });

    expect(topicSelect.value).toBe("");
    expect(within(topicSelect).queryByText("Stoikiometri")).not.toBeInTheDocument();
    expect(within(topicSelect).getByText("Mekanika")).toBeInTheDocument();
  });

  it("shows shown/total summary and load-more when a next_cursor exists", async () => {
    testsState = {
      data: { data: sampleTests, next_cursor: "cursor-2", total: 22 },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<TestsPage />);

    await waitFor(() => {
      expect(screen.getByText("Menampilkan 2 dari 22 tes")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /muat lebih banyak/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /muat lebih banyak/i }));

    await waitFor(() => {
      expect(mockUseAdminTests).toHaveBeenLastCalledWith(
        expect.objectContaining({ cursor: "cursor-2" }),
      );
    });
  });

  it("omits the summary and load-more when the list is empty", async () => {
    testsState = {
      data: { data: [], total: 0 },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<TestsPage />);

    await waitFor(() => {
      expect(screen.getByText(/belum ada tes/i)).toBeInTheDocument();
    });
    expect(screen.queryByText(/menampilkan/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /muat lebih banyak/i })).not.toBeInTheDocument();
  });

  it("opens the new empty test's detail page after create", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "t3", title: "Tryout Baru" });

    render(<TestsPage />);
    fireEvent.click(screen.getByRole("button", { name: /tes baru/i }));
    fireEvent.change(screen.getByLabelText(/judul/i), { target: { value: "Tryout Baru" } });
    fireEvent.change(screen.getByLabelText(/mata pelajaran/i), { target: { value: "Fisika" } });
    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "Optik" } });
    fireEvent.change(screen.getByLabelText(/durasi/i), { target: { value: "60" } });
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => expect(push).toHaveBeenCalledWith("/admin/exam/tests/t3"));
  });

  it("offers retry when the list fails to load", async () => {
    const refetch = vi.fn();
    testsState = { data: null, isLoading: false, isError: true, error: new Error("gagal memuat tes"), refetch };

    render(<TestsPage />);
    fireEvent.click(screen.getByRole("button", { name: /coba lagi/i }));

    expect(refetch).toHaveBeenCalled();
  });

  it("shows question count, duration and material/audio text lines per row", () => {
    testsState = {
      ...testsState,
      data: {
        data: [
          { ...sampleTests[0], question_count: 12, shared_material_html: "<p>Bacaan</p>", audio_url: "https://cdn.test/a.mp3" },
          { ...sampleTests[1], question_count: 0 },
        ],
      },
    };

    render(<TestsPage />);

    const [first, second] = screen.getAllByTestId("test-row");
    expect(within(first).getByText("12 soal")).toBeInTheDocument();
    expect(within(first).getByText("90 menit")).toBeInTheDocument();
    expect(within(first).getByText("Materi bersama")).toBeInTheDocument();
    expect(within(first).getByText("Audio section")).toBeInTheDocument();
    expect(within(second).getByText("0 soal")).toBeInTheDocument();
    expect(within(second).getByText("Tanpa materi bersama")).toBeInTheDocument();
    expect(within(second).queryByText("Audio section")).not.toBeInTheDocument();
  });

  it("shows the result count beside the search and the source-of-truth footnote", () => {
    testsState = { ...testsState, data: { data: sampleTests, total: 22 } };
    render(<TestsPage />);

    expect(screen.getByTestId("tests-count")).toHaveTextContent("22 tes");
    expect(screen.getByText(/tes adalah sumber soal, materi, dan durasi/i)).toBeInTheDocument();
  });

  it("falls back to the loaded row count when the server sends no total", () => {
    render(<TestsPage />);

    expect(screen.getByTestId("tests-count")).toHaveTextContent("2 tes");
  });

  it("keeps the subject and topic filters behind the Filter button and counts active ones", () => {
    render(<TestsPage />);

    expect(screen.queryByTestId("tests-subject-filter")).not.toBeInTheDocument();
    const filterButton = screen.getByRole("button", { name: /^filter$/i });
    expect(filterButton).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(filterButton);
    expect(screen.getByTestId("tests-subject-filter")).toBeInTheDocument();
    expect(screen.getByTestId("tests-topic-filter")).toBeInTheDocument();

    fireEvent.change(screen.getByTestId("tests-subject-filter"), { target: { value: "Fisika" } });
    expect(screen.getByRole("button", { name: /^filter \(1\)$/i })).toHaveAttribute("aria-expanded", "true");
  });

  it("never requests the previous cursor with a changed filter", async () => {
    testsState = { ...testsState, data: { data: sampleTests, next_cursor: "cursor-2", total: 22 } };
    render(<TestsPage />);

    fireEvent.click(screen.getByRole("button", { name: /muat lebih banyak/i }));
    await waitFor(() =>
      expect(mockUseAdminTests).toHaveBeenLastCalledWith(expect.objectContaining({ cursor: "cursor-2" })),
    );
    mockUseAdminTests.mockClear();

    fireEvent.click(screen.getByRole("button", { name: /^filter/i }));
    fireEvent.change(screen.getByTestId("tests-subject-filter"), { target: { value: "Fisika" } });

    const stale = mockUseAdminTests.mock.calls.filter(
      ([filters]) => filters?.subject === "Fisika" && filters?.cursor === "cursor-2",
    );
    expect(stale).toHaveLength(0);
    expect(mockUseAdminTests).toHaveBeenLastCalledWith(
      expect.objectContaining({ subject: "Fisika", cursor: undefined }),
    );
  });

  it("exposes each row as a real link to the detail page, named by the test title", () => {
    render(<TestsPage />);

    const link = screen.getByRole("link", { name: "Tryout UTBK Saintek" });
    expect(link).toHaveAttribute("href", "/admin/exam/tests/t1");

    fireEvent.click(link);
    expect(push).toHaveBeenCalledTimes(1);
    expect(push).toHaveBeenCalledWith("/admin/exam/tests/t1");
  });
});
