import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import { useParams } from "next/navigation";
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

vi.mock("@/components/admin/CertificateDesignTab", () => ({
  CertificateDesignTab: ({ examId }: { examId: string }) => (
    <div data-testid="certificate-design-tab">{examId}</div>
  ),
}));

beforeEach(() => {
  mockRole = undefined;
});

const mockReplaceTests = vi.fn();
const mockGradeEssay = vi.fn();

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
  useUpdateExam: () => ({ mutateAsync: vi.fn(), isPending: false }),
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

vi.mock("@/lib/hooks/admin-tests", () => ({
  useAdminTests: (...args: unknown[]) => {
    useAdminTestsSpy(...args);
    return {
      data: { data: [] as Test[] },
      isLoading: false,
    };
  },
}));

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
      expect(screen.getByText("UTBK")).toBeInTheDocument();
    });

    expect(screen.getByText("score_pembahasan")).toBeInTheDocument();
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

  it("opens the edit modal when the overview edit button is clicked", async () => {
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^edit$/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /^edit$/i }));

    await waitFor(() => {
      expect(screen.getByText("Edit Ujian")).toBeInTheDocument();
    });
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

  it("shows an empty state when no sessions need grading", async () => {
    gradingSessionsState = { data: { data: [] }, isLoading: false, isError: false, error: null };
    render(<ExamPackageDetailPage />);
    openGradingTab();

    await waitFor(() => {
      expect(screen.getByText(/tidak ada sesi yang perlu dinilai/i)).toBeInTheDocument();
    });
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
    expect(screen.queryByText(/Under Maintenance/i)).not.toBeInTheDocument();
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

  it("shows empty-state message when no leaderboard rows", async () => {
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
    fireEvent.click(screen.getByRole("button", { name: /^tes$/i }));
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

  it("admin_school sees only Overview and Registrations tabs, no Edit button", async () => {
    mockRole = "admin_school";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: /^ringkasan$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^pendaftaran$/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^tes$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^sertifikat$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^penilaian$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Leaderboard" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^edit$/i })).not.toBeInTheDocument();
  });

  it("super_admin still sees all seven tabs and the Edit button", async () => {
    mockRole = "super_admin";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: /^tes$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^sertifikat$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Leaderboard" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^edit$/i })).toBeInTheDocument();
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

    expect(useAdminTestsSpy).toHaveBeenLastCalledWith(undefined, false);
    expect(useExamAnalyticsSpy).toHaveBeenLastCalledWith("exam-1", false);
    expect(useExamLeaderboardSpy).toHaveBeenLastCalledWith(
      "exam-1",
      { limit: 20 },
      false,
    );
    expect(useGradingSessionsSpy).toHaveBeenLastCalledWith("exam-1", false);
    expect(useSessionEssaysSpy).toHaveBeenLastCalledWith(undefined, false);
  });

  it("super_admin does fire the tests/analytics/leaderboard/grading queries", async () => {
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

    expect(useAdminTestsSpy).toHaveBeenLastCalledWith(undefined, true);
    expect(useExamAnalyticsSpy).toHaveBeenLastCalledWith("exam-1", true);
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

  it("admin_exam (unscoped, non-super_admin) still sees the under-development stub on Registrations", async () => {
    mockRole = "admin_exam";
    render(<ExamPackageDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: sampleExam.title })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /^pendaftaran$/i }));

    expect(screen.queryByTestId("exam-registrations-tab")).not.toBeInTheDocument();
  });
});
