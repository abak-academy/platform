import { describe, it, expect, vi, beforeEach } from "vitest";
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
  data: null as { data: Test[]; next_cursor?: string } | null,
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
    expect(screen.getByText(/90\s*min/)).toBeInTheDocument();
    expect(screen.getByText("Reading")).toBeInTheDocument();
    expect(screen.getByText(/60\s*min/)).toBeInTheDocument();
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
    fireEvent.input(screen.getByLabelText(/mata pelajaran/i), { target: { value: "Matematika" } });
    fireEvent.input(screen.getByLabelText(/topik/i), { target: { value: "Aljabar" } });
    fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "60" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Tryout Baru",
          subject: "Matematika",
          topic: "Aljabar",
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

    vi.unstubAllGlobals();
  });

  it("fetches with no filters when every control is at its default (FR-16)", async () => {
    render(<TestsPage />);

    await waitFor(() => expect(screen.getByText("Tryout UTBK Saintek")).toBeInTheDocument());

    expect(mockUseAdminTests).toHaveBeenLastCalledWith({
      subject: undefined,
      topic: undefined,
      q: undefined,
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

    vi.useRealTimers();
  });

  it("re-scopes the topic dropdown to the selected subject and clears a stale topic (FR-13)", async () => {
    render(<TestsPage />);
    await waitFor(() => expect(screen.getByText("Tryout UTBK Saintek")).toBeInTheDocument());

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
});