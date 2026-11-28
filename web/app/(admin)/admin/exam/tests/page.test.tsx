import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from "vitest";
import userEvent from "@testing-library/user-event";
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
let topicsLoading = false;

vi.mock("@/lib/hooks/admin-topics", () => ({
  useTopics: (subject?: string) => ({
    data: { data: subject ? sampleTopics.filter((t) => t.subject === subject) : sampleTopics },
    isLoading: topicsLoading,
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

beforeAll(() => {
  globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} };
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

async function pick(field: string, name: string) {
  await userEvent.click(screen.getByRole("combobox", { name: field }));
  await userEvent.click(await screen.findByRole("option", { name }));
}

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

    expect(screen.getAllByTestId("test-row")[0].tagName).toBe("ARTICLE");
    expect(screen.getByText("Section · Matematika · Aljabar")).toBeInTheDocument();
    expect(screen.getByText(/90\s*menit/)).toBeInTheDocument();
    expect(screen.getByText("Section · Bahasa Inggris · Reading")).toBeInTheDocument();
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

  it("does not navigate when clicking the delete action button", async () => {
    render(<TestsPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout UTBK Saintek")).toBeInTheDocument();
    });

    const row = screen.getByText("Tryout UTBK Saintek").closest("[data-testid=test-row]") as HTMLElement;
    vi.stubGlobal("confirm", () => false);
    fireEvent.click(within(row).getByRole("button", { name: /hapus/i }));
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

  it("has no quick-edit button on a card, only delete", () => {
    render(<TestsPage />);

    const [row] = screen.getAllByTestId("test-row");
    expect(within(row).queryByRole("button", { name: /^edit$/i })).not.toBeInTheDocument();
    expect(within(row).getByRole("button", { name: /hapus/i })).toBeInTheDocument();
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
      used: undefined,
      cursor: undefined,
    });
  });

  it("sends q only after the debounce, trimmed", async () => {
    vi.useFakeTimers();
    render(<TestsPage />);
    mockUseAdminTests.mockClear();

    fireEvent.change(screen.getByRole("searchbox"), {
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

    await userEvent.click(screen.getByRole("button", { name: /^filter/i }));
    await pick("Mata pelajaran", "Kimia");
    await pick("Topik", "Stoikiometri");
    expect(screen.getByRole("combobox", { name: "Topik" })).toHaveTextContent("Stoikiometri");

    await pick("Mata pelajaran", "Fisika");

    const topic = screen.getByRole("combobox", { name: "Topik" });
    expect(topic).not.toHaveTextContent("Stoikiometri");
    await userEvent.click(topic);
    expect(await screen.findByRole("option", { name: "Mekanika" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Stoikiometri" })).not.toBeInTheDocument();
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

  it("puts question count and duration chips in one row container next to delete", () => {
    testsState = { ...testsState, data: { data: [{ ...sampleTests[0], question_count: 12 }] } };
    render(<TestsPage />);

    const chips = screen.getByTestId("test-chips");
    expect(within(chips).getByText("12 soal")).toBeInTheDocument();
    expect(within(chips).getByText("90 menit")).toBeInTheDocument();
    expect(chips).toHaveClass("flex-nowrap");
  });

  it("shows one section line with subject, topic and section type", () => {
    testsState = {
      ...testsState,
      data: { data: [{ ...sampleTests[0], section_type: "listening" }, sampleTests[1]] },
    };
    render(<TestsPage />);

    const [first, second] = screen.getAllByTestId("test-row");
    expect(within(first).getByText("Section · Matematika · Aljabar · Listening")).toBeInTheDocument();
    expect(within(second).getByText("Section · Bahasa Inggris · Reading")).toBeInTheDocument();
  });

  it("joins shared material and audio on one line, only what exists, never a negative", () => {
    testsState = {
      ...testsState,
      data: {
        data: [
          { ...sampleTests[0], id: "a", shared_material_html: "<p>x</p>", audio_url: "https://cdn.test/a.mp3" },
          { ...sampleTests[0], id: "b", shared_material_html: "<p>x</p>" },
          { ...sampleTests[0], id: "c", audio_url: "https://cdn.test/a.mp3" },
          { ...sampleTests[0], id: "d", shared_material_html: "  " },
        ],
      },
    };
    render(<TestsPage />);

    const [both, material, audio, none] = screen.getAllByTestId("test-row");
    expect(within(both).getByTestId("test-content")).toHaveTextContent(/^Materi bersama · Audio section$/);
    expect(within(material).getByTestId("test-content")).toHaveTextContent(/^Materi bersama$/);
    expect(within(audio).getByTestId("test-content")).toHaveTextContent(/^Audio section$/);
    expect(within(none).queryByTestId("test-content")).not.toBeInTheDocument();
    expect(screen.queryByText(/tanpa materi/i)).not.toBeInTheDocument();
  });

  it("shows how many exams use a test, or that it is unused", () => {
    testsState = {
      ...testsState,
      data: { data: [{ ...sampleTests[0], exam_count: 3 }, { ...sampleTests[1], exam_count: 0 }] },
    };
    render(<TestsPage />);

    const [used, unused] = screen.getAllByTestId("test-row");
    expect(within(used).getByText("Dipakai di 3 ujian")).toBeInTheDocument();
    expect(within(unused).getByText("Belum dipakai")).toBeInTheDocument();
  });

  it("sends used=true/false from the Usage filter, counts it, and reset clears it", async () => {
    render(<TestsPage />);

    await userEvent.click(screen.getByRole("button", { name: /^filter/i }));
    await pick("Pemakaian", "Dipakai");
    expect(mockUseAdminTests).toHaveBeenLastCalledWith(expect.objectContaining({ used: true }));
    expect(screen.getByRole("button", { name: /^filter \(1\)$/i })).toBeInTheDocument();
    expect(within(screen.getByRole("group", { name: "Filter aktif" })).getByText("Pemakaian: Dipakai")).toBeInTheDocument();

    await pick("Pemakaian", "Belum dipakai");
    expect(mockUseAdminTests).toHaveBeenLastCalledWith(expect.objectContaining({ used: false }));

    await userEvent.click(screen.getByRole("button", { name: /^reset filter$/i }));
    expect(mockUseAdminTests).toHaveBeenLastCalledWith(expect.objectContaining({ used: undefined }));
    expect(screen.queryByRole("group", { name: "Filter aktif" })).not.toBeInTheDocument();
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

  it("disables the topic dropdown while topics load", async () => {
    topicsLoading = true;
    try {
      render(<TestsPage />);
      await userEvent.click(screen.getByRole("button", { name: /^filter$/i }));
      expect(screen.getByRole("combobox", { name: "Topik" })).toBeDisabled();
    } finally {
      topicsLoading = false;
    }
  });

  it("keeps the subject and topic filters behind the Filter button and counts active ones", async () => {
    render(<TestsPage />);

    expect(screen.queryByRole("combobox", { name: "Mata pelajaran" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /^filter$/i }));
    expect(screen.getByRole("combobox", { name: "Mata pelajaran" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Topik" })).toBeInTheDocument();

    await pick("Mata pelajaran", "Fisika");
    expect(screen.getByRole("button", { name: /^filter \(1\)$/i })).toBeInTheDocument();
    expect(within(screen.getByRole("group", { name: "Filter aktif" })).getByText("Mata pelajaran: Fisika")).toBeInTheDocument();

    await pick("Mata pelajaran", "Semua");
    expect(screen.queryByRole("group", { name: "Filter aktif" })).not.toBeInTheDocument();
  });

  it("never requests the previous cursor with a changed filter", async () => {
    testsState = { ...testsState, data: { data: sampleTests, next_cursor: "cursor-2", total: 22 } };
    render(<TestsPage />);

    fireEvent.click(screen.getByRole("button", { name: /muat lebih banyak/i }));
    await waitFor(() =>
      expect(mockUseAdminTests).toHaveBeenLastCalledWith(expect.objectContaining({ cursor: "cursor-2" })),
    );
    mockUseAdminTests.mockClear();

    await userEvent.click(screen.getByRole("button", { name: /^filter/i }));
    await pick("Mata pelajaran", "Fisika");

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
