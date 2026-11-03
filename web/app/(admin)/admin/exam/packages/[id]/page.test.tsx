import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import { useParams } from "next/navigation";
import { toast } from "sonner";
import { ApiError } from "@/lib/api";
import { useUIStore } from "@/stores/ui";
import ExamPackageDetailPage from "./page";
import type {
  ExamDetail,
  GradingSessionItem,
  GradingEssayItem,
  Test,
  ExamAnalytics,
  ExamLeaderboardEntry,
} from "@/lib/types";

vi.mock("next/navigation", () => ({
  useParams: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

// Default: unauthenticated/no-role, matching the real store's default state —
// existing tests below rely on this to see all seven tabs (unscoped behavior).
let mockRole: string | undefined = undefined;

vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: (s: { user: { role?: string } | null }) => unknown) =>
    sel({ user: mockRole ? { role: mockRole } : null }),
}));

vi.mock("@/components/admin/ExamRegistrationsTab", () => ({
  ExamRegistrationsTab: ({ examId, examName }: { examId: string; examName: string }) => (
    <div data-testid="exam-registrations-tab">
      {examId}:{examName}
    </div>
  ),
}));

vi.mock("@/components/admin/ExamResultsTab", () => ({
  ExamResultsTab: ({ examId }: { examId: string }) => (
    <div data-testid="exam-results-tab">{examId}</div>
  ),
}));

vi.mock("@/components/admin/ResultsWorkspaceTab", () => ({
  ResultsWorkspaceTab: ({ examId }: { examId: string }) => (
    <div data-testid="results-workspace-tab">{examId}</div>
  ),
}));

vi.mock("@/components/admin/CertificateDesignTab", () => ({
  CertificateDesignTab: ({ examId }: { examId: string }) => (
    <div data-testid="certificate-design-tab">{examId}</div>
  ),
}));

// ExamModal's end-screen section uses ImageUploadInput, which calls
// usePresignAdminImageUpload (a real useMutation) — mock it so rendering the
// modal doesn't need a QueryClient.
vi.mock("@/lib/hooks/admin-uploads", () => ({
  usePresignAdminImageUpload: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

beforeEach(() => {
  mockRole = undefined;
  mockSetCertificateEnabled.mockClear();
  mockSetCardEnabled.mockClear();
  adminTestsState = {
    data: { data: [] as Test[] },
    isLoading: false,
    isFetching: false,
  };
});

const mockReplaceTests = vi.fn();
const mockUpdateExam = vi.fn();
const mockGradeEssay = vi.fn();
const mockSetCertificateEnabled = vi.fn().mockResolvedValue({});
const mockSetCardEnabled = vi.fn().mockResolvedValue({});

// PR review P2: these 5 hooks back the tabs school-scoped admins never see
// (tests/grading/leaderboard/analytics). Spy on them so we can assert they're
// called with enabled=false — i.e. never actually fetch — for admin_school.
const useAdminTestsSpy = vi.fn();
const useGradingSessionsSpy = vi.fn();
const useSessionEssaysSpy = vi.fn();
const useExamAnalyticsSpy = vi.fn();
const useExamLeaderboardSpy = vi.fn();

let examState: {
  data: ExamDetail | undefined;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
  refetch: ReturnType<typeof vi.fn>;
} = { data: undefined, isLoading: true, isError: false, error: null, refetch: vi.fn() };

let gradingSessionsState: {
  data: { data: GradingSessionItem[] } | undefined;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
} = { data: undefined, isLoading: true, isError: false, error: null };

let sessionEssaysState: {
  data: { data: GradingEssayItem[] } | undefined;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
} = { data: undefined, isLoading: false, isError: false, error: null };

let gradeEssayState: {
  mutateAsync: ReturnType<typeof vi.fn>;
  isPending: boolean;
  variables?: { question_id: string };
} = { mutateAsync: mockGradeEssay, isPending: false, variables: undefined };

const sampleAnalytics: ExamAnalytics = {
  average_score: 75.5,
  completion_rate: 0.85,
  distribution: [
    { label: "0-20", count: 1 },
    { label: "21-40", count: 2 },
    { label: "41-60", count: 3 },
    { label: "61-80", count: 5 },
    { label: "81-100", count: 4 },
  ],
};

const sampleLeaderboardEntries: ExamLeaderboardEntry[] = [
  { rank: 1, session_id: "sess1", student_id: "s1", student_name: "Budi Santoso", score: 95 },
  { rank: 2, session_id: "sess2", student_id: "s2", student_name: "Siti Aminah", score: 88 },
  { rank: 3, session_id: "sess3", student_id: "s3", student_name: "Agus Wijaya", score: 82 },
];

let analyticsState: { data: ExamAnalytics | undefined; isLoading: boolean } = {
  data: sampleAnalytics,
  isLoading: false,
};

let leaderboardState: {
  data: { data: ExamLeaderboardEntry[]; next_cursor?: string } | undefined;
  isLoading: boolean;
  isFetching: boolean;
} = {
  data: { data: sampleLeaderboardEntries, next_cursor: undefined },
  isLoading: false,
  isFetching: false,
};

vi.mock("@/lib/hooks/admin-exams", () => ({
  useExam: () => examState,
  useReplaceExamTests: () => ({ mutateAsync: mockReplaceTests, isPending: false }),
  useCreateExam: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateExam: () => ({ mutateAsync: mockUpdateExam, isPending: false }),
  useSetCertificateEnabled: () => ({ mutateAsync: mockSetCertificateEnabled, isPending: false }),
  useSetCardEnabled: () => ({ mutateAsync: mockSetCardEnabled, isPending: false }),
  useGradingSessions: (...args: unknown[]) => {
    useGradingSessionsSpy(...args);
    return gradingSessionsState;
  },
  useSessionEssays: (...args: unknown[]) => {
    useSessionEssaysSpy(...args);
    return sessionEssaysState;
  },
  useGradeEssay: () => gradeEssayState,
  useExamAnalytics: (...args: unknown[]) => {
    useExamAnalyticsSpy(...args);
    return analyticsState;
  },
  useExamLeaderboard: (...args: unknown[]) => {
    useExamLeaderboardSpy(...args);
    return leaderboardState;
  },
}));

let adminTestsState: {
  data: { data: Test[]; next_cursor?: string; total?: number } | undefined;
  isLoading: boolean;
  isFetching: boolean;
} = { data: { data: [] as Test[] }, isLoading: false, isFetching: false };

vi.mock("@/lib/hooks/admin-tests", () => ({
  useAdminTests: (...args: unknown[]) => {
    useAdminTestsSpy(...args);
    return adminTestsState;
  },
}));

const sampleAvailableTests: Test[] = [
  {
    id: "avail-1",
    title: "Tes Aljabar",
    subject: "Matematika",
    topic: "Aljabar",
    duration_minutes: 60,
  },
  {
    id: "avail-2",
    title: "Tes Optik",
    subject: "Fisika",
    topic: "Optik",
    duration_minutes: 45,
  },
];

const sampleExam: ExamDetail = {
  id: "exam-1",
  title: "UTS Matematika",
  scheduled_at: "2026-07-01T08:00:00Z",
  timer_mode: "overall",
  duration_minutes: 90,
  is_free: false,
  requires_checkin: true,
  allow_leaderboard: false,
  randomize: false,
  status: "published",
  certificate_enabled: true,
  tests: [],
};

const sampleSessions: GradingSessionItem[] = [
  {
    session_id: "session-1",
    student_id: "student-1",
    student_name: "Budi Santoso",
    submitted_at: "2026-06-30T10:00:00Z",
    ungraded_essay_count: 2,
  },
  {
    session_id: "session-2",
    student_id: "student-2",
    student_name: "Siti Aminah",
    submitted_at: "2026-06-30T11:00:00Z",
    ungraded_essay_count: 1,
  },
];

const sampleEssays: GradingEssayItem[] = [
  {
    question_id: "q-1",
    body: "Jelaskan penyebab perang Diponegoro",
    answer: "Karena penjajahan Belanda",
    point_correct: 10,
    score: null,
    grader_comment: null,
    graded_at: null,
  },
];

function openGradingTab() {
  fireEvent.click(screen.getByRole("button", { name: /^penilaian$/i }));
}

const sampleExamWithExtendedFields: ExamDetail = {
  ...sampleExam,
  mode: "utbk",
  result_config: "score_pembahasan",
  result_release_at: "2026-07-02T10:00:00Z",
  check_in_window_minutes: 15,
  grace_window_minutes: 5,
  max_attempts: 2,
  certificate_template: "modern",
};

describe("ExamPackageDetailPage — overview tab", () => {
  beforeEach(() => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "exam-1" });
    examState = {
      data: sampleExamWithExtendedFields,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
  });

  it("renders extended package fields in the overview", async () => {
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("score_pembahasan")).toBeInTheDocument();
    });

    expect(screen.getByText("15 menit")).toBeInTheDocument();
    expect(screen.getByText("5 menit")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("modern")).toBeInTheDocument();
  });

  it("shows unlimited when max_attempts is null", async () => {
    examState.data = { ...sampleExamWithExtendedFields, max_attempts: null };

    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByText(/tidak terbatas/i)).toBeInTheDocument();
    });
  });

  it("shows cannot-retake when max_attempts is 0", async () => {
    examState.data = { ...sampleExamWithExtendedFields, max_attempts: 0 };

    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByText(/tidak bisa ulangi/i)).toBeInTheDocument();
    });
  });

  it("shows no draft notice and no status badge or status row", async () => {
    examState.data = { ...sampleExamWithExtendedFields, status: "draft" };

    render(<ExamPackageDetailPage />);

    await screen.findByText("score_pembahasan");
    expect(screen.queryByText(/tidak terlihat oleh siswa dan dapat diubah bebas/i)).not.toBeInTheDocument();
    expect(screen.queryByText("draft")).not.toBeInTheDocument();
    expect(screen.queryByText("published")).not.toBeInTheDocument();
  });

  it("offers one Pengaturan header action that opens the modal, with no separate Edit entry", async () => {
    render(<ExamPackageDetailPage />);

    const settings = await screen.findAllByRole("button", { name: /^pengaturan$/i });
    expect(settings).toHaveLength(1);
    expect(screen.queryByRole("button", { name: /^edit$/i })).not.toBeInTheDocument();

    fireEvent.click(settings[0]);

    expect(await screen.findByText("Edit Ujian")).toBeInTheDocument();
  });

  it("hides Pengaturan for admin_school", async () => {
    mockRole = "admin_school";
    render(<ExamPackageDetailPage />);

    await screen.findByText("score_pembahasan");
    expect(screen.queryByRole("button", { name: /^pengaturan$/i })).not.toBeInTheDocument();
  });

  it("does not repeat the title or policy rows in the summary", async () => {
    examState.data = { ...sampleExamWithExtendedFields, randomize: true };

    render(<ExamPackageDetailPage />);

    await screen.findByText("score_pembahasan");
    expect(screen.getAllByText("UTS Matematika")).toHaveLength(1);
    for (const label of ["Title", "Timer mode", "Duration", "Randomize", "Status", "Mode"]) {
      expect(screen.queryByText(label)).not.toBeInTheDocument();
    }
  });

  it("shows check-in window and grace only when check-in is required", async () => {
    const { unmount } = render(<ExamPackageDetailPage />);
    await screen.findByText("score_pembahasan");
    expect(screen.queryByText("Jendela Check-in")).toBeInTheDocument();
    expect(screen.queryByText("Jendela Toleransi")).toBeInTheDocument();
    unmount();

    examState.data = { ...sampleExamWithExtendedFields, requires_checkin: false };
    render(<ExamPackageDetailPage />);
    await screen.findByText("score_pembahasan");
    expect(screen.queryByText("Jendela Check-in")).not.toBeInTheDocument();
    expect(screen.queryByText("Jendela Toleransi")).not.toBeInTheDocument();
  });

  it("shows result release only when results are not hidden", async () => {
    const { unmount } = render(<ExamPackageDetailPage />);
    await screen.findByText("score_pembahasan");
    expect(screen.getByText("Hasil Dibuka")).toBeInTheDocument();
    unmount();

    examState.data = { ...sampleExamWithExtendedFields, result_config: "hidden" };
    render(<ExamPackageDetailPage />);
    await screen.findByText("hidden");
    expect(screen.queryByText("Hasil Dibuka")).not.toBeInTheDocument();
  });

  it("still submits the hidden check-in and release fields when settings are saved", async () => {
    mockUpdateExam.mockReset();
    mockUpdateExam.mockResolvedValue({});
    examState.data = {
      ...sampleExamWithExtendedFields,
      requires_checkin: false,
      result_config: "hidden",
      result_release_at: "2026-07-02T10:00:00Z",
    };

    render(<ExamPackageDetailPage />);
    fireEvent.click(await screen.findByRole("button", { name: /^pengaturan$/i }));
    await screen.findByText("Edit Ujian");
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => expect(mockUpdateExam).toHaveBeenCalled());
    expect(mockUpdateExam.mock.calls[0][0]).toMatchObject({
      check_in_window_minutes: 15,
      grace_window_minutes: 5,
      result_release_at: "2026-07-02T10:00:00.000Z",
    });
  });

  it("names the tests tab 'Tes & section'", async () => {
    render(<ExamPackageDetailPage />);

    expect(await screen.findByRole("button", { name: /^tes & section$/i })).toBeInTheDocument();
  });
});

describe("ExamPackageDetailPage — grading tab", () => {
  beforeEach(() => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "exam-1" });
    examState = {
      data: sampleExam,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    gradingSessionsState = {
      data: { data: sampleSessions },
      isLoading: false,
      isError: false,
      error: null,
    };
    sessionEssaysState = {
      data: { data: sampleEssays },
      isLoading: false,
      isError: false,
      error: null,
    };
    gradeEssayState = { mutateAsync: mockGradeEssay, isPending: false, variables: undefined };
    mockGradeEssay.mockReset();
    mockGradeEssay.mockResolvedValue({ status: "graded", score: 8 });
  });

  it("renders the list of sessions needing grading", async () => {
    render(<ExamPackageDetailPage />);
    openGradingTab();

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });
    expect(screen.getByText("Siti Aminah")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("shows an empty state when no sessions need grading, via DataTable's single colSpan row", async () => {
    gradingSessionsState = { data: { data: [] }, isLoading: false, isError: false, error: null };
    render(<ExamPackageDetailPage />);
    openGradingTab();

    await waitFor(() => {
      expect(screen.getByText(/tidak ada sesi yang perlu dinilai/i)).toBeInTheDocument();
    });
    const emptyCell = screen.getByText(/tidak ada sesi yang perlu dinilai/i).closest("td");
    expect(emptyCell).toHaveAttribute("colspan", "4");
  });

  it("selecting a session shows its essays", async () => {
    render(<ExamPackageDetailPage />);
    openGradingTab();

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });

    const row = screen.getByText("Budi Santoso").closest("tr");
    expect(row).toBeTruthy();
    fireEvent.click(within(row as HTMLElement).getByRole("button", { name: /lihat detail/i }));

    await waitFor(() => {
      expect(screen.getByText("Jelaskan penyebab perang Diponegoro")).toBeInTheDocument();
    });
    expect(screen.getByText("Karena penjajahan Belanda")).toBeInTheDocument();
  });

  it("grading an essay calls useGradeEssay with the score and comment payload", async () => {
    render(<ExamPackageDetailPage />);
    openGradingTab();

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });
    fireEvent.click(
      within(screen.getByText("Budi Santoso").closest("tr") as HTMLElement).getByRole("button", {
        name: /lihat detail/i,
      }),
    );

    await waitFor(() => {
      expect(screen.getByText("Jelaskan penyebab perang Diponegoro")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/^skor$/i), { target: { value: "8" } });
    fireEvent.change(screen.getByLabelText(/komentar/i), {
      target: { value: "Jawaban cukup lengkap" },
    });
    fireEvent.click(screen.getByRole("button", { name: /simpan nilai/i }));

    await waitFor(() => {
      expect(mockGradeEssay).toHaveBeenCalledWith({
        question_id: "q-1",
        score: 8,
        comment: "Jawaban cukup lengkap",
      });
    });
  });

  it("blocks the save and does not call useGradeEssay when the score exceeds point_correct", async () => {
    render(<ExamPackageDetailPage />);
    openGradingTab();

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });
    fireEvent.click(
      within(screen.getByText("Budi Santoso").closest("tr") as HTMLElement).getByRole("button", {
        name: /lihat detail/i,
      }),
    );

    await waitFor(() => {
      expect(screen.getByText("Jelaskan penyebab perang Diponegoro")).toBeInTheDocument();
    });

    // point_correct for this essay is 10; 11 is out of bounds.
    fireEvent.change(screen.getByLabelText(/^skor$/i), { target: { value: "11" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan nilai/i }));

    expect(mockGradeEssay).not.toHaveBeenCalled();
  });

  it("clears a session from the grading queue after it is fully graded (post-invalidation refetch)", async () => {
    render(<ExamPackageDetailPage />);
    openGradingTab();

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });
    fireEvent.click(
      within(screen.getByText("Budi Santoso").closest("tr") as HTMLElement).getByRole("button", {
        name: /lihat detail/i,
      }),
    );

    await waitFor(() => {
      expect(screen.getByText("Jelaskan penyebab perang Diponegoro")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/^skor$/i), { target: { value: "8" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan nilai/i }));

    await waitFor(() => {
      expect(mockGradeEssay).toHaveBeenCalled();
    });

    // Simulate the queue refetch that useGradeEssay's onSuccess invalidation triggers.
    gradingSessionsState = {
      data: { data: sampleSessions.filter((s) => s.session_id !== "session-1") },
      isLoading: false,
      isError: false,
      error: null,
    };

    fireEvent.click(screen.getByRole("button", { name: /kembali ke daftar/i }));

    await waitFor(() => {
      expect(screen.queryByText("Budi Santoso")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Siti Aminah")).toBeInTheDocument();
  });

  it("renders essay body as stripped plain text in grading list (Task 8 audit)", async () => {
    sessionEssaysState = {
      data: {
        data: [
          {
            question_id: "q-rich-essay",
            body: "<b>Tanya</b> <i>essay</i> <script>x</script>tentang sejarah",
            answer: "Jawaban",
            point_correct: 10,
            score: null,
            grader_comment: null,
            graded_at: null,
          },
        ],
      },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamPackageDetailPage />);
    openGradingTab();

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });
    fireEvent.click(
      within(screen.getByText("Budi Santoso").closest("tr") as HTMLElement).getByRole("button", {
        name: /lihat detail/i,
      }),
    );

    // The essay body now renders as stripped plain text — no <b>/<script> elements
    // survive in the grading list (list/row context per Task 8 spec).
    await waitFor(() => {
      expect(screen.getByText("Tanya essay tentang sejarah")).toBeInTheDocument();
    });
    const bodyDiv = screen.getByText("Tanya essay tentang sejarah").closest("li");
    expect(bodyDiv).not.toBeNull();
    expect(bodyDiv?.querySelector("b")).toBeNull();
    expect(bodyDiv?.querySelector("script")).toBeNull();
  });
});

describe("ExamPackageDetailPage — leaderboard tab", () => {
  beforeEach(() => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "exam-1" });
    examState = {
      data: sampleExam,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    analyticsState = { data: sampleAnalytics, isLoading: false };
    leaderboardState = {
      data: { data: sampleLeaderboardEntries, next_cursor: undefined },
      isLoading: false,
      isFetching: false,
    };
  });

  function openLeaderboardTab() {
    fireEvent.click(screen.getByRole("button", { name: "Leaderboard" }));
  }

  it("renders analytics tiles instead of UnderMaintenance", async () => {
    render(<ExamPackageDetailPage />);
    openLeaderboardTab();

    expect(screen.getByText("75.5")).toBeInTheDocument();
    expect(screen.getByText("85%")).toBeInTheDocument();
    expect(screen.getByText("0-20")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("81-100")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.queryByText(/sedang dalam pengembangan/i)).not.toBeInTheDocument();
  });

  it("renders a leaderboard table with entries", async () => {
    render(<ExamPackageDetailPage />);
    openLeaderboardTab();

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });
    expect(screen.getByText("#1")).toBeInTheDocument();
    expect(screen.getByText("95")).toBeInTheDocument();
    expect(screen.getByText("Siti Aminah")).toBeInTheDocument();
    expect(screen.getByText("Agus Wijaya")).toBeInTheDocument();
  });

  it("shows empty-state message when no leaderboard rows, via DataTable's single colSpan row", async () => {
    leaderboardState = {
      data: { data: [], next_cursor: undefined },
      isLoading: false,
      isFetching: false,
    };

    render(<ExamPackageDetailPage />);
    openLeaderboardTab();

    await waitFor(() => {
      expect(screen.getByText("Belum ada data peringkat")).toBeInTheDocument();
    });
    const emptyCell = screen.getByText("Belum ada data peringkat").closest("td");
    expect(emptyCell).toHaveAttribute("colspan", "3");
  });

  it("renders retake rows (same student twice) without duplicate React keys", async () => {
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    leaderboardState = {
      data: {
        data: [
          { rank: 1, session_id: "sess1", student_id: "s1", student_name: "Budi Santoso", score: 95 },
          { rank: 2, session_id: "sess9", student_id: "s1", student_name: "Budi Santoso", score: 88 },
        ],
        next_cursor: undefined,
      },
      isLoading: false,
      isFetching: false,
    };

    render(<ExamPackageDetailPage />);
    openLeaderboardTab();

    await waitFor(() => {
      expect(screen.getAllByText("Budi Santoso")).toHaveLength(2);
    });

    const dupKeyWarning = errSpy.mock.calls.some((args) =>
      String(args[0]).includes("same key"),
    );
    errSpy.mockRestore();
    expect(dupKeyWarning).toBe(false);
  });

  it("renders the load-more control inside the leaderboard DataTable's footer when a next_cursor exists, and omits it otherwise", async () => {
    leaderboardState = {
      data: { data: sampleLeaderboardEntries, next_cursor: "cursor-2" },
      isLoading: false,
      isFetching: false,
    };

    const { unmount } = render(<ExamPackageDetailPage />);
    openLeaderboardTab();

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /muat lebih banyak/i })).toBeInTheDocument();
    unmount();

    leaderboardState = {
      data: { data: sampleLeaderboardEntries, next_cursor: undefined },
      isLoading: false,
      isFetching: false,
    };
    render(<ExamPackageDetailPage />);
    openLeaderboardTab();

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: /muat lebih banyak/i })).not.toBeInTheDocument();
  });
});

describe("ExamPackageDetailPage — preset buttons in tests tab", () => {
  beforeEach(() => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "exam-1" });
    analyticsState = { data: undefined, isLoading: false };
    leaderboardState = {
      data: { data: [], next_cursor: undefined },
      isLoading: false,
      isFetching: false,
    };
  });

  function openTestsTab() {
    fireEvent.click(screen.getByRole("button", { name: /^tes & section$/i }));
  }

  it("shows IELTS preset button when exam mode is ielts", async () => {
    examState = {
      data: { ...sampleExam, mode: "ielts" },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<ExamPackageDetailPage />);
    openTestsTab();

    await waitFor(() => {
      expect(screen.getByText("IELTS Preset")).toBeInTheDocument();
    });
  });

  it("shows UTBK preset button when exam mode is utbk", async () => {
    examState = {
      data: { ...sampleExam, mode: "utbk" },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<ExamPackageDetailPage />);
    openTestsTab();

    await waitFor(() => {
      expect(screen.getByText("UTBK Preset")).toBeInTheDocument();
    });
  });

  it("does NOT show preset buttons for standard mode", async () => {
    examState = {
      data: sampleExam,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<ExamPackageDetailPage />);
    openTestsTab();

    await waitFor(() => {
      expect(screen.queryByText("UTBK Preset")).not.toBeInTheDocument();
      expect(screen.queryByText("IELTS Preset")).not.toBeInTheDocument();
    });
  });

  it("IELTS preset prefills three sections typed listening/reading/writing", async () => {
    examState = {
      data: { ...sampleExam, mode: "ielts" },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<ExamPackageDetailPage />);
    openTestsTab();

    await waitFor(() => {
      expect(screen.getByText("IELTS Preset")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("IELTS Preset"));

    await waitFor(() => {
      expect(screen.getByText(/IELTS Listening/)).toBeInTheDocument();
      expect(screen.getByText(/IELTS Reading/)).toBeInTheDocument();
      expect(screen.getByText(/IELTS Writing/)).toBeInTheDocument();
    });
  });

  it("UTBK preset prefills four sections with typical durations", async () => {
    examState = {
      data: { ...sampleExam, mode: "utbk" },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<ExamPackageDetailPage />);
    openTestsTab();

    await waitFor(() => {
      expect(screen.getByText("UTBK Preset")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("UTBK Preset"));

    await waitFor(() => {
      expect(screen.getByText(/TPS - Potensi Skolastik/)).toBeInTheDocument();
      expect(screen.getByText(/Penalaran Matematika/)).toBeInTheDocument();
      expect(screen.getByText(/Literasi Bahasa Indonesia/)).toBeInTheDocument();
      expect(screen.getByText(/Literasi Bahasa Inggris/)).toBeInTheDocument();
    });
  });
});

describe("ExamPackageDetailPage — role-scoped registrations tab", () => {
  beforeEach(() => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "exam-1" });
    examState = {
      data: sampleExam,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
  });

  it("admin_school sees Overview, Registrations, and Results tabs only", async () => {
    mockRole = "admin_school";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: /^ringkasan$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^pendaftaran$/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^tes & section$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^sertifikat$/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^hasil$/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^penilaian$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Leaderboard" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^edit$/i })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^hasil$/i }));
    expect(screen.getByTestId("exam-results-tab")).toHaveTextContent("exam-1");
  });

  it("super_admin sees unified Results instead of separate Results/Leaderboard and still sees Pengaturan", async () => {
    mockRole = "super_admin";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: /^tes & section$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^sertifikat$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^hasil$/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Leaderboard" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^pengaturan$/i })).toBeInTheDocument();
  });

  it("renders the CertificateDesignTab on the Sertifikat tab", async () => {
    mockRole = "super_admin";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /^sertifikat$/i }));

    expect(screen.getByTestId("certificate-design-tab")).toHaveTextContent("exam-1");
  });

  it("admin_school never fires the tests/analytics/leaderboard/grading queries (PR review P2)", async () => {
    mockRole = "admin_school";
    useAdminTestsSpy.mockClear();
    useExamAnalyticsSpy.mockClear();
    useExamLeaderboardSpy.mockClear();
    useGradingSessionsSpy.mockClear();
    useSessionEssaysSpy.mockClear();

    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    expect(useAdminTestsSpy).toHaveBeenLastCalledWith({ q: undefined, cursor: undefined }, false);
    expect(useExamAnalyticsSpy).toHaveBeenLastCalledWith("exam-1", false);
    expect(useExamLeaderboardSpy).toHaveBeenLastCalledWith(
      "exam-1",
      { limit: 20 },
      false,
    );
    expect(useGradingSessionsSpy).toHaveBeenLastCalledWith("exam-1", false);
    expect(useSessionEssaysSpy).toHaveBeenLastCalledWith(undefined, false);
  });

  it("super_admin skips legacy leaderboard queries but keeps tests/grading enabled", async () => {
    mockRole = "super_admin";
    useAdminTestsSpy.mockClear();
    useExamAnalyticsSpy.mockClear();
    useExamLeaderboardSpy.mockClear();
    useGradingSessionsSpy.mockClear();
    useSessionEssaysSpy.mockClear();

    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    expect(useAdminTestsSpy).toHaveBeenLastCalledWith({ q: undefined, cursor: undefined }, true);
    expect(useExamAnalyticsSpy).toHaveBeenLastCalledWith("exam-1", false);
    expect(useExamLeaderboardSpy).toHaveBeenLastCalledWith(
      "exam-1",
      { limit: 20 },
      false,
    );
    expect(useGradingSessionsSpy).toHaveBeenLastCalledWith("exam-1", true);
    expect(useSessionEssaysSpy).toHaveBeenLastCalledWith(undefined, true);
  });

  it("renders ExamRegistrationsTab for admin_school on the Registrations tab", async () => {
    mockRole = "admin_school";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /^pendaftaran$/i }));

    expect(screen.getByTestId("exam-registrations-tab")).toHaveTextContent(
      `exam-1:${sampleExam.title}`,
    );
  });

  it("renders ExamRegistrationsTab for super_admin on the Registrations tab", async () => {
    mockRole = "super_admin";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /^pendaftaran$/i }));

    expect(screen.getByTestId("exam-registrations-tab")).toHaveTextContent(
      `exam-1:${sampleExam.title}`,
    );
  });

  it("admin_exam sees the Registrations tab content, not UnderMaintenance", async () => {
    mockRole = "admin_exam";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /^pendaftaran$/i }));

    expect(screen.getByTestId("exam-registrations-tab")).toHaveTextContent(
      `exam-1:${sampleExam.title}`,
    );
    expect(screen.queryByText(/sedang dalam pengembangan/i)).not.toBeInTheDocument();
  });

  it("admin_exam sees the unified Results workspace instead of Results/Leaderboard", async () => {
    mockRole = "admin_exam";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: "Leaderboard" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^hasil$/i }));

    expect(screen.getByTestId("results-workspace-tab")).toHaveTextContent("exam-1");
    expect(screen.queryByTestId("exam-results-tab")).not.toBeInTheDocument();
    expect(screen.queryByText(/sedang dalam pengembangan/i)).not.toBeInTheDocument();
  });

  it("admin_exam skips legacy leaderboard queries but keeps tests/grading enabled", async () => {
    mockRole = "admin_exam";
    useExamAnalyticsSpy.mockClear();
    useExamLeaderboardSpy.mockClear();
    useGradingSessionsSpy.mockClear();

    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    expect(useExamAnalyticsSpy).toHaveBeenLastCalledWith("exam-1", false);
    expect(useExamLeaderboardSpy).toHaveBeenLastCalledWith("exam-1", { limit: 20 }, false);
    expect(useGradingSessionsSpy).toHaveBeenLastCalledWith("exam-1", true);
  });

  it("super_admin sees the unified Results tab content", async () => {
    mockRole = "super_admin";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /^hasil$/i }));

    expect(screen.getByTestId("results-workspace-tab")).toHaveTextContent("exam-1");
    expect(screen.queryByTestId("exam-results-tab")).not.toBeInTheDocument();
  });

  it("admin_store sees UnderMaintenance on both Registrations and Results", async () => {
    mockRole = "admin_store";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /^pendaftaran$/i }));
    expect(screen.queryByTestId("exam-registrations-tab")).not.toBeInTheDocument();
    expect(screen.getByText(/sedang dalam pengembangan/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^hasil$/i }));
    expect(screen.queryByTestId("exam-results-tab")).not.toBeInTheDocument();
    expect(screen.getByText(/sedang dalam pengembangan/i)).toBeInTheDocument();
  });
});

describe("ExamPackageDetailPage — certificate enablement (FR-8/FR-11/FR-12)", () => {
  beforeEach(() => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "exam-1" });
  });

  it("FR-8: the Certificate tab is absent when certificate_enabled is false", async () => {
    examState = {
      data: { ...sampleExam, certificate_enabled: false },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    expect(screen.queryByRole("button", { name: /^sertifikat$/i })).not.toBeInTheDocument();
    expect(screen.queryByTestId("certificate-design-tab")).not.toBeInTheDocument();
  });

  it("FR-8/FR-11: the Certificate tab is present when certificate_enabled is true", async () => {
    examState = {
      data: { ...sampleExam, certificate_enabled: true },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^sertifikat$/i })).toBeInTheDocument();
    });
  });

  it("FR-11: the enable action issues the enable request and nothing else", async () => {
    examState = {
      data: { ...sampleExam, certificate_enabled: false },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /^aktifkan sertifikat$/i }));

    await waitFor(() => {
      expect(mockSetCertificateEnabled).toHaveBeenCalledTimes(1);
    });
    expect(mockSetCertificateEnabled).toHaveBeenCalledWith(true);
    expect(mockReplaceTests).not.toHaveBeenCalled();
    expect(mockGradeEssay).not.toHaveBeenCalled();
  });

  it("FR-12: the disable control warns the saved design is kept, then issues the disable request", async () => {
    examState = {
      data: { ...sampleExam, certificate_enabled: true },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /^nonaktifkan sertifikat$/i }));

    expect(
      screen.getByText(/desain sertifikat yang sudah tersimpan tidak akan dihapus/i),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^ya, nonaktifkan$/i }));

    await waitFor(() => {
      expect(mockSetCertificateEnabled).toHaveBeenCalledTimes(1);
    });
    expect(mockSetCertificateEnabled).toHaveBeenCalledWith(false);
  });
});

describe("ExamPackageDetailPage — Tes Tersedia picker", () => {
  beforeEach(() => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "exam-1" });
    examState = {
      data: sampleExam,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
  });

  async function openTestsTab() {
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /^tes & section$/i }));
  }

  it("shows the search input, shown/total summary, and load-more when a next_cursor exists", async () => {
    mockRole = "super_admin";
    adminTestsState = {
      data: { data: sampleAvailableTests, next_cursor: "cursor-2", total: 25 },
      isLoading: false,
      isFetching: false,
    };

    await openTestsTab();

    expect(screen.getByText("Tes Aljabar")).toBeInTheDocument();
    expect(screen.getByText("Tes Optik")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Cari…")).toBeInTheDocument();
    expect(screen.getByText("Menampilkan 2 dari 25 tes")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /muat lebih banyak/i }),
    ).toBeInTheDocument();
  });

  it("sends the trimmed q after the debounce and keeps search across load-more", async () => {
    mockRole = "super_admin";
    adminTestsState = {
      data: { data: sampleAvailableTests, next_cursor: "cursor-2", total: 25 },
      isLoading: false,
      isFetching: false,
    };

    vi.useFakeTimers();
    try {
      render(<ExamPackageDetailPage />);
      fireEvent.click(screen.getByRole("button", { name: /^tes & section$/i }));
      fireEvent.change(screen.getByPlaceholderText("Cari…"), {
        target: { value: "  optik  " },
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(299);
      });
      expect(useAdminTestsSpy).toHaveBeenLastCalledWith(
        { q: undefined, cursor: undefined },
        true,
      );

      await act(async () => {
        await vi.advanceTimersByTimeAsync(1);
      });
      expect(useAdminTestsSpy).toHaveBeenLastCalledWith(
        { q: "optik", cursor: undefined },
        true,
      );

      fireEvent.click(screen.getByRole("button", { name: /muat lebih banyak/i }));
      expect(useAdminTestsSpy).toHaveBeenLastCalledWith(
        { q: "optik", cursor: "cursor-2" },
        true,
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it("omits the load-more control when no next_cursor remains", async () => {
    mockRole = "super_admin";
    adminTestsState = {
      data: { data: sampleAvailableTests, total: 2 },
      isLoading: false,
      isFetching: false,
    };

    await openTestsTab();

    expect(screen.getByText("Menampilkan 2 dari 2 tes")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /muat lebih banyak/i }),
    ).not.toBeInTheDocument();
  });
});


describe("composable section presentation", () => {
  it.each(["one_per_page", "all_on_page"] as const)("preserves %s while saving the section list", async presentation => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({id:"exam-1"});
    mockReplaceTests.mockReset();
    mockReplaceTests.mockResolvedValue(undefined);
    examState = {data:{...sampleExam,format_version:2,navigation_policy:"sequential",tests:[{id:"entry",exam_id:"exam-1",test_id:"section-1",sort_order:0,question_presentation:presentation,test:{id:"section-1",title:"Reading Section",subject:"English",duration_minutes:30,question_count:2}}]},isLoading:false,isError:false,error:null,refetch:vi.fn()};
    render(<ExamPackageDetailPage />);
    fireEvent.click(screen.getByRole("button",{name:/^tes & section$/i}));
    expect(await screen.findByLabelText("Tampilan soal Reading Section")).toHaveValue(presentation);
    fireEvent.click(screen.getByRole("button",{name:/^simpan$/i}));
    await waitFor(() => expect(mockReplaceTests).toHaveBeenCalledWith([{test_id:"section-1",question_presentation:presentation}]));
  });
});

describe("ExamPackageDetailPage — student preview button", () => {
  beforeEach(() => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "exam-1" });
    examState = {
      data: sampleExam,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
  });

  it.each(["super_admin", "admin_exam"])("%s sees a Pratinjau siswa link to the preview route", async (role) => {
    mockRole = role;
    render(<ExamPackageDetailPage />);

    const link = await screen.findByRole("link", { name: /pratinjau siswa/i });
    expect(link).toHaveAttribute("href", "/admin/exam/packages/exam-1/preview");
  });

  it.each(["admin_school", "admin_store", undefined])("%s does not see the preview link", async (role) => {
    mockRole = role;
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });
    expect(screen.queryByRole("link", { name: /pratinjau siswa/i })).not.toBeInTheDocument();
  });
});

describe("composable exam — material conflicts and policy lock", () => {
  const sectionId = "5f1d1c2e-3b4a-4c6d-8e9f-0a1b2c3d4e5f";
  const composable = (over: Partial<ExamDetail> = {}, presentation: "one_per_page" | "all_on_page" = "all_on_page"): ExamDetail => ({
    ...sampleExam,
    format_version: 2,
    navigation_policy: "sequential",
    has_sessions: false,
    tests: [
      {
        id: "entry-1",
        exam_id: "exam-1",
        test_id: sectionId,
        sort_order: 0,
        question_presentation: presentation,
        material_question_positions: [2, 4],
        test: { id: sectionId, title: "Reading Section", subject: "English", duration_minutes: 30, question_count: 5 },
      },
    ],
    ...over,
  });

  function open(data: ExamDetail) {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "exam-1" });
    mockReplaceTests.mockReset();
    mockReplaceTests.mockResolvedValue(undefined);
    examState = { data, isLoading: false, isError: false, error: null, refetch: vi.fn() };
    adminTestsState = { data: { data: sampleAvailableTests }, isLoading: false, isFetching: false };
    useUIStore.setState({ lang: "id" });
    const view = render(<ExamPackageDetailPage />);
    fireEvent.click(screen.getByRole("button", { name: /^tes & section$/i }));
    return view;
  }

  it("names the section and question positions of an all_on_page conflict, never an id", async () => {
    const { container } = open(composable());

    const message = await screen.findByText(/soal 2, 4 punya materi sendiri/i);
    expect(message).toHaveTextContent("#1 · Reading Section");
    expect(container.textContent).not.toContain(sectionId);
  });

  it("numbers the conflict by the section order the admin sees after removing an earlier section", async () => {
    const otherId = "9a8b7c6d-1111-4222-8333-444455556666";
    const base = composable();
    const data = composable({
      tests: [
        { id: "entry-0", exam_id: "exam-1", test_id: otherId, sort_order: 0, question_presentation: "one_per_page", test: { id: otherId, title: "Section Awal", subject: "English", duration_minutes: 10, question_count: 3 } },
        { ...base.tests[0], sort_order: 1 },
      ],
    });
    open(data);

    expect(await screen.findByText(/^#2 · Reading Section: soal 2, 4/)).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole("button", { name: /^hapus$/i })[0]);

    expect(await screen.findByText(/^#1 · Reading Section: soal 2, 4/)).toBeInTheDocument();
    expect(screen.queryByText(/^#2 · Reading Section/)).not.toBeInTheDocument();
  });

  it("shows no conflict for one_per_page sections or sections without material", async () => {
    open(composable({}, "one_per_page"));
    await screen.findByLabelText("Tampilan soal Reading Section");
    expect(screen.queryByText(/punya materi sendiri/i)).not.toBeInTheDocument();
  });

  it("clears the conflict as soon as the section is switched to one question per page", async () => {
    open(composable());
    await screen.findByText(/punya materi sendiri/i);

    fireEvent.change(screen.getByLabelText("Tampilan soal Reading Section"), { target: { value: "one_per_page" } });

    expect(screen.queryByText(/punya materi sendiri/i)).not.toBeInTheDocument();
  });

  it("locks section controls with an explanation once the exam has sessions", async () => {
    open(composable({ has_sessions: true }));

    expect(await screen.findByText(/terkunci karena ujian ini sudah memiliki sesi/i)).toBeInTheDocument();
    expect(screen.getByLabelText("Tampilan soal Reading Section")).toBeDisabled();
    expect(screen.getAllByRole("button", { name: /^hapus$/i }).every((b) => (b as HTMLButtonElement).disabled)).toBe(true);
    expect(screen.getAllByRole("button", { name: /^tambah$/i }).every((b) => (b as HTMLButtonElement).disabled)).toBe(true);
    expect(screen.getByRole("button", { name: /^simpan$/i })).toBeDisabled();
  });

  it("keeps section controls enabled without sessions", async () => {
    open(composable());

    await screen.findByLabelText("Tampilan soal Reading Section");
    expect(screen.queryByText(/terkunci karena ujian ini sudah memiliki sesi/i)).not.toBeInTheDocument();
    expect(screen.getByLabelText("Tampilan soal Reading Section")).toBeEnabled();
    expect(screen.getAllByRole("button", { name: /^hapus$/i })[0]).toBeEnabled();
    expect(screen.getAllByRole("button", { name: /^tambah$/i })[0]).toBeEnabled();
    expect(screen.getByRole("button", { name: /^simpan$/i })).toBeEnabled();
  });

  it("locks policy controls in the settings modal but leaves other fields editable", async () => {
    open(composable({ has_sessions: true }));
    fireEvent.click(screen.getByRole("button", { name: /^pengaturan$/i }));

    await screen.findByText("Edit Ujian");
    expect(screen.getByLabelText("Berurutan")).toBeDisabled();
    expect(screen.getByLabelText(/durasi/i)).toBeDisabled();
    expect(screen.getByLabelText(/judul/i)).toBeEnabled();
  });

  it("shows a localized message and refetches when the server rejects with 409 exam_has_sessions", async () => {
    const data = composable();
    open(data);
    vi.mocked(toast.error).mockClear();
    mockReplaceTests.mockRejectedValue(new ApiError("exam_has_sessions", "exam already has sessions", 409));

    fireEvent.change(await screen.findByLabelText("Tampilan soal Reading Section"), { target: { value: "one_per_page" } });
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringMatching(/sudah memiliki sesi/i)));
    expect(examState.refetch).toHaveBeenCalled();

    act(() => useUIStore.setState({ lang: "en" }));
    vi.mocked(toast.error).mockClear();
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringMatching(/already has sessions/i)));
    expect(toast.error).not.toHaveBeenCalledWith("exam already has sessions");
    act(() => useUIStore.setState({ lang: "id" }));
  });
});

describe("ExamPackageDetailPage — exam event info", () => {
  const open = (over: Partial<ExamDetail> = {}, role?: string) => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "exam-1" });
    mockRole = role;
    examState = {
      data: { ...sampleExam, format_version: 2, navigation_policy: "sequential", has_sessions: false, ...over },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    adminTestsState = { data: { data: sampleAvailableTests }, isLoading: false, isFetching: false };
    useUIStore.setState({ lang: "id" });
    mockUpdateExam.mockReset();
    mockUpdateExam.mockResolvedValue({});
    return render(<ExamPackageDetailPage />);
  };

  it("summary tab shows the WhatsApp group card with the stored link", () => {
    open({ whatsapp_group_url: "https://chat.whatsapp.com/AbC" });
    const card = screen.getByRole("region", { name: "Grup WhatsApp kompetisi" });
    expect(within(card).getByText("Link tersedia")).toBeInTheDocument();
    expect(within(card).getByRole("link")).toHaveAttribute("href", "https://chat.whatsapp.com/AbC");
  });

  it("summary tab says the link is not provided, without a reason", () => {
    open({ whatsapp_group_url: null });
    const card = screen.getByRole("region", { name: "Grup WhatsApp kompetisi" });
    expect(within(card).getByText("Tidak dicantumkan")).toBeInTheDocument();
    expect(within(card).queryByRole("link")).not.toBeInTheDocument();
  });

  it("puts the Aturan pengerjaan card in the right column above the Test picker", () => {
    open();
    fireEvent.click(screen.getByRole("button", { name: /^tes & section$/i }));
    const rules = screen.getByRole("region", { name: "Aturan pengerjaan" });
    const picker = screen.getByRole("heading", { name: "Tes Tersedia" });
    const attached = screen.getByRole("heading", { name: "Tes Terlampir" });
    expect(rules.compareDocumentPosition(picker) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(attached.compareDocumentPosition(rules) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(rules.parentElement).toBe(picker.closest("div.md-card-outlined")!.parentElement);
  });

  it("keeps the rules editable while policy and section controls are locked by sessions", () => {
    open({ has_sessions: true });
    fireEvent.click(screen.getByRole("button", { name: /^tes & section$/i }));
    expect(screen.getAllByRole("button", { name: /^tambah$/i })[0]).toBeDisabled();
    const toggle = screen.getByRole("checkbox", { name: /tambahkan aturan tambahan/i });
    expect(toggle).toBeEnabled();
    fireEvent.click(toggle);
    fireEvent.input(screen.getByLabelText(/keterangan tambahan/i), { target: { value: "Catatan" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan aturan tambahan/i }));
    return waitFor(() =>
      expect(mockUpdateExam).toHaveBeenCalledWith({ additional_rules_note: "Catatan", additional_rules_url: null }),
    );
  });

  it("does not show the manage hint to school-scoped admins", () => {
    open({ whatsapp_group_url: null }, "admin_school");
    expect(screen.queryByText(/kelola link melalui pengaturan/i)).not.toBeInTheDocument();
  });
});
