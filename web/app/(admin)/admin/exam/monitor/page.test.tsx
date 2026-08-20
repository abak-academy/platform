import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import ExamMonitorPage from "./page";
import type { ExamListItem, SessionMonitorResponse, SessionMonitorRow, ViolationRecent } from "@/lib/types";

// ── Mutable mock state ──

let examsState: {
  data: { data: ExamListItem[] } | null;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
} = {
  data: null,
  isLoading: true,
  isError: false,
  error: null,
};

let monitorState: {
  data: SessionMonitorResponse | null;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
} = {
  data: null,
  isLoading: true,
  isError: false,
  error: null,
};

const reopenMutate = vi.fn();
const forceSubmitMutate = vi.fn();

vi.mock("@/lib/hooks/admin-exams", () => ({
  useExams: () => examsState,
}));

vi.mock("@/lib/hooks/admin-sessions", () => ({
  useSessionMonitor: () => monitorState,
  useReopenSession: () => ({ mutate: reopenMutate, isPending: false }),
  useForceSubmitSession: () => ({ mutate: forceSubmitMutate, isPending: false }),
}));

vi.mock("@/stores/ui", () => ({
  useUIStore: (sel: any) => sel({ lang: "id", theme: "light", toggleTheme: vi.fn(), setLang: vi.fn() }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

// ── Helpers ──

const sampleExams: ExamListItem[] = [
  {
    id: "exam-1",
    title: "UTBK 2026",
    scheduled_at: "2026-08-01T07:00:00Z",
    has_published_product: true,
    is_free: true,
    requires_checkin: true,
    allow_leaderboard: true,
    randomize: false,
    timer_mode: "overall",
    duration_minutes: 120,
    grace_window_minutes: 5,
    status: "active",
    registration_count: 0,
  },
  {
    id: "exam-2",
    title: "Tryout 1",
    scheduled_at: "2026-07-15T07:00:00Z",
    has_published_product: false,
    is_free: true,
    requires_checkin: true,
    allow_leaderboard: true,
    randomize: false,
    timer_mode: "overall",
    duration_minutes: 90,
    grace_window_minutes: 5,
    status: "draft",
    registration_count: 0,
  },
];

const sampleRows: SessionMonitorRow[] = [
  {
    registration_id: "r1",
    student_id: "u1",
    student_name: "Budi Santoso",
    school_name: "SMAN 1 Jakarta",
    status: "registered",
    answers_saved: 0,
    total_questions: 40,
    checked_in_at: null,
    last_saved_at: null,
    violation_count: 0,
    session_id: null,
    admin_submitted: false,
    extended_until: null,
  },
  {
    registration_id: "r2",
    student_id: "u2",
    student_name: "Siti Aisyah",
    school_name: "SMAN 2 Jakarta",
    status: "checked_in",
    answers_saved: 0,
    total_questions: 40,
    checked_in_at: "2026-07-06T06:45:00Z",
    last_saved_at: null,
    violation_count: 0,
    session_id: "s1",
    admin_submitted: false,
    extended_until: null,
  },
  {
    registration_id: "r3",
    student_id: "u3",
    student_name: "Ahmad Fauzi",
    school_name: "SMAN 1 Bogor",
    status: "in_progress",
    answers_saved: 15,
    total_questions: 40,
    checked_in_at: "2026-07-06T06:50:00Z",
    last_saved_at: "2026-07-06T07:15:00Z",
    violation_count: 1,
    session_id: "s2",
    admin_submitted: false,
    extended_until: null,
  },
  {
    registration_id: "r4",
    student_id: "u4",
    student_name: "Dewi Lestari",
    school_name: "SMAN 3 Depok",
    status: "overdue",
    answers_saved: 30,
    total_questions: 40,
    checked_in_at: "2026-07-06T06:40:00Z",
    last_saved_at: "2026-07-06T07:50:00Z",
    violation_count: 3,
    session_id: "s3",
    admin_submitted: false,
    extended_until: null,
  },
  {
    registration_id: "r5",
    student_id: "u5",
    student_name: "Rudi Hermawan",
    school_name: null,
    status: "submitted",
    answers_saved: 40,
    total_questions: 40,
    checked_in_at: "2026-07-06T06:30:00Z",
    last_saved_at: "2026-07-06T08:00:00Z",
    violation_count: 0,
    session_id: "s4",
    admin_submitted: false,
    extended_until: null,
  },
];

const sampleViolations: ViolationRecent[] = [
  {
    session_id: "s3",
    student_name: "Dewi Lestari",
    count: 3,
    latest_type: "tab_switch",
    latest_occurred_at: "2026-07-06T07:55:00Z",
  },
  {
    session_id: "s2",
    student_name: "Ahmad Fauzi",
    count: 1,
    latest_type: "face_missing",
    latest_occurred_at: "2026-07-06T07:10:00Z",
  },
];

// ── Tests ──

describe("ExamMonitorPage", () => {
  beforeEach(() => {
    reopenMutate.mockReset();
    forceSubmitMutate.mockReset();

    examsState = {
      data: { data: sampleExams },
      isLoading: false,
      isError: false,
      error: null,
    };

    monitorState = {
      data: {
        exam: {
          id: "exam-1",
          title: "UTBK 2026",
          scheduled_at: "2026-08-01T07:00:00Z",
          duration_minutes: 120,
          grace_window_minutes: 5,
          status: "published",
        },
        rows: sampleRows,
        violations_recent: sampleViolations,
      },
      isLoading: false,
      isError: false,
      error: null,
    };
  });

  it("automatically selects first published exam and renders monitor rows", async () => {
    render(<ExamMonitorPage />);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
      expect(screen.getByText("Siti Aisyah")).toBeInTheDocument();
    });

    // Ahmad Fauzi and Dewi Lestari appear in both table and sidebar
    const ahmadElements = screen.getAllByText("Ahmad Fauzi");
    expect(ahmadElements.length).toBeGreaterThanOrEqual(1);

    const dewiElements = screen.getAllByText("Dewi Lestari");
    expect(dewiElements.length).toBeGreaterThanOrEqual(1);

    expect(screen.getByText("Rudi Hermawan")).toBeInTheDocument();
  });

  it("renders each status with correct badge label", async () => {
    render(<ExamMonitorPage />);

    await waitFor(() => {
      expect(screen.getByText("Terdaftar")).toBeInTheDocument();
      expect(screen.getByText("Tercheck-in")).toBeInTheDocument();
      expect(screen.getByText("Sedang berjalan")).toBeInTheDocument();
      expect(screen.getByText("Terlambat")).toBeInTheDocument();
      expect(screen.getByText("Terkirim")).toBeInTheDocument();
    });
  });

  it("renders progress values for each row", async () => {
    render(<ExamMonitorPage />);

    await waitFor(() => {
      // 0/40 appears for both registered + checked_in rows
      const zeroProgress = screen.getAllByText("0/40");
      expect(zeroProgress.length).toBeGreaterThanOrEqual(2);

      // 15/40, 30/40, 40/40 are unique values
      expect(screen.getByText("15/40")).toBeInTheDocument();
      expect(screen.getByText("30/40")).toBeInTheDocument();
      expect(screen.getByText("40/40")).toBeInTheDocument();
    });
  });

  it("only shows Reopen and Force Submit actions on overdue rows", async () => {
    render(<ExamMonitorPage />);

    await waitFor(() => {
      // Dewi Lestari appears in both table and sidebar — use getAllByText
      const dewiElements = screen.getAllByText("Dewi Lestari");
      expect(dewiElements.length).toBeGreaterThanOrEqual(1);
    });

    // Should have exactly 1 Reopen and 1 Force Submit button
    const reopenButtons = screen.getAllByRole("button", { name: "Perpanjang" });
    expect(reopenButtons).toHaveLength(1);

    const forceSubmitButtons = screen.getAllByRole("button", { name: "Paksa kumpulkan" });
    expect(forceSubmitButtons).toHaveLength(1);
  });

  it("shows no actions on non-overdue rows", async () => {
    render(<ExamMonitorPage />);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
      expect(screen.getByText("Siti Aisyah")).toBeInTheDocument();
    });

    // Only 1 Reopen button total (for the one overdue row)
    const allReopen = screen.queryAllByRole("button", { name: "Perpanjang" });
    expect(allReopen).toHaveLength(1);

    const allForceSubmit = screen.queryAllByRole("button", { name: "Paksa kumpulkan" });
    expect(allForceSubmit).toHaveLength(1);
  });

  it("renders the violation sidebar", async () => {
    render(<ExamMonitorPage />);

    await waitFor(() => {
      expect(screen.getByText("Pelanggaran")).toBeInTheDocument();
      // ×3 and ×1 are unique to sidebar
      expect(screen.getByText("×3")).toBeInTheDocument();
      expect(screen.getByText("×1")).toBeInTheDocument();
    });
  });

  it("shows loading skeletons when monitor is loading", async () => {
    examsState = { data: { data: [sampleExams[0]] }, isLoading: false, isError: false, error: null };
    monitorState = { data: null, isLoading: true, isError: false, error: null };

    render(<ExamMonitorPage />);

    await waitFor(() => {
      const skeletons = document.querySelectorAll("[data-slot='skeleton']");
      expect(skeletons.length).toBeGreaterThanOrEqual(3);
    });
  });

  it("surfaces monitor API error as inline error text", async () => {
    examsState = { data: { data: [sampleExams[0]] }, isLoading: false, isError: false, error: null };
    monitorState = { data: null, isLoading: false, isError: true, error: new Error("Gagal memuat data") };

    render(<ExamMonitorPage />);

    await waitFor(() => {
      expect(screen.getByText(/Gagal memuat data/i)).toBeInTheDocument();
    });
  });

  it("shows empty state when no rows exist", async () => {
    monitorState = {
      data: {
        exam: {
          id: "exam-1",
          title: "UTBK 2026",
          scheduled_at: "2026-08-01T07:00:00Z",
          duration_minutes: 120,
          grace_window_minutes: 5,
          status: "published",
        },
        rows: [],
        violations_recent: [],
      },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamMonitorPage />);

    await waitFor(() => {
      expect(screen.getByText("Belum ada peserta")).toBeInTheDocument();
    });
  });

  it("shows 'Select an exam' prompt when no exam is selected", async () => {
    examsState = { data: { data: [] }, isLoading: false, isError: false, error: null };

    render(<ExamMonitorPage />);

    await waitFor(() => {
      expect(screen.getByText("Pilih ujian untuk melihat data")).toBeInTheDocument();
    });
  });

  it("renders the exam picker with published exams only", async () => {
    render(<ExamMonitorPage />);

    await waitFor(() => {
      const trigger = screen.getByRole("combobox", { name: /pilih ujian/i });
      expect(trigger).toBeInTheDocument();
    });
  });

  it("renders the AdminPageHeader with Live chip", async () => {
    render(<ExamMonitorPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1, name: /Monitor Sesi/i })).toBeInTheDocument();
      expect(screen.getByText("Live")).toBeInTheDocument();
    });
  });

  it("renders active section column for sectioned rows and dash for standard rows", async () => {
    const sectionedRows: SessionMonitorRow[] = [
      ...sampleRows,
      {
        registration_id: "r6",
        student_id: "u6",
        student_name: "Rina Wijaya",
        school_name: "SMAN UTBK",
        status: "in_progress",
        answers_saved: 5,
        total_questions: 40,
        checked_in_at: "2026-07-06T06:50:00Z",
        last_saved_at: "2026-07-06T07:15:00Z",
        violation_count: 0,
        session_id: "s5",
        admin_submitted: false,
        extended_until: null,
        active_section_test_id: "t1",
        active_section_title: "Tes Potensi Skolastik",
        active_section_started_at: "2026-07-06T06:50:00Z",
        active_section_duration_minutes: 30,
        active_section_extended_until: null,
        active_section_remaining_seconds: 1200,
      },
      {
        registration_id: "r7",
        student_id: "u7",
        student_name: "Bayu Pratama",
        school_name: "SMAN 1 Bandung",
        status: "overdue",
        answers_saved: 20,
        total_questions: 40,
        checked_in_at: "2026-07-06T06:40:00Z",
        last_saved_at: "2026-07-06T07:30:00Z",
        violation_count: 2,
        session_id: "s6",
        admin_submitted: false,
        extended_until: null,
        active_section_test_id: "t2",
        active_section_title: "Penalaran Umum",
        active_section_started_at: "2026-07-06T07:20:00Z",
        active_section_duration_minutes: 15,
        active_section_extended_until: null,
        active_section_remaining_seconds: 0,
      },
    ];

    monitorState = {
      data: {
        exam: {
          id: "exam-1",
          title: "UTBK 2026",
          scheduled_at: "2026-08-01T07:00:00Z",
          duration_minutes: 120,
          grace_window_minutes: 5,
          status: "published",
        },
        rows: sectionedRows,
        violations_recent: sampleViolations,
      },
      isLoading: false,
      isError: false,
      error: null,
    };

    render(<ExamMonitorPage />);

    await waitFor(() => {
      // Column header for active section should be present
      expect(screen.getByText("Sesi Aktif")).toBeInTheDocument();

      // Sectioned rows render their section titles
      expect(screen.getByText("Tes Potensi Skolastik")).toBeInTheDocument();
      expect(screen.getByText("Penalaran Umum")).toBeInTheDocument();

      // Active sectioned row shows remaining time in MM:SS format (20 min)
      expect(screen.getByText("20:00")).toBeInTheDocument();

      // Overdue sectioned row shows 00:00
      expect(screen.getByText("00:00")).toBeInTheDocument();

      // Standard rows unchanged — check their names appear
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
      const ahmadElements = screen.getAllByText("Ahmad Fauzi");
      expect(ahmadElements.length).toBeGreaterThanOrEqual(1);
    });
  });

  it("renders rows through DataTable as non-focusable (rows are not clickable)", async () => {
    render(<ExamMonitorPage />);

    await waitFor(() => expect(screen.getByText("Budi Santoso")).toBeInTheDocument());

    const row = screen.getByText("Budi Santoso").closest("tr") as HTMLElement;
    expect(row).not.toHaveAttribute("role", "button");
    expect(row).not.toHaveAttribute("tabIndex");
  });

  it("shows 'No violations yet' when there are no recent violations", async () => {
    monitorState = {
      ...monitorState,
      data: {
        ...monitorState.data!,
        violations_recent: [],
      },
    };

    render(<ExamMonitorPage />);

    await waitFor(() => {
      expect(screen.getByText("Belum ada pelanggaran")).toBeInTheDocument();
    });
  });

  it("uses ink/brand utility classes instead of raw M3 tokens", async () => {
    render(<ExamMonitorPage />);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });

    const table = screen.getByTestId("exam-monitor-table");
    expect(table.innerHTML).not.toContain("md-sys-color");
    expect(screen.getByText("SMAN 1 Jakarta")).toHaveClass("text-ink-600");
    expect(table.querySelector(".bg-brand-600")).not.toBeNull();
  });
});
