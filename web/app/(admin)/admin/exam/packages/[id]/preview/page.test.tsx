import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

import ExamPreviewPage from "./page";
import type { RegistrationExam } from "@/lib/types";

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "exam-1" }),
  useRouter: () => ({ push: vi.fn() }),
}));

let uiStore = { lang: "id" as "id" | "en" };

vi.mock("@/stores/ui", () => ({
  useUIStore: (selector: (s: typeof uiStore) => unknown) => selector(uiStore),
}));

const { startSession, checkIn } = vi.hoisted(() => ({ startSession: vi.fn(), checkIn: vi.fn() }));

vi.mock("@/lib/hooks/exam", () => ({
  useStartSession: () => {
    startSession();
    return { mutateAsync: vi.fn(), isPending: false };
  },
  useCheckIn: () => {
    checkIn();
    return { mutate: vi.fn(), isPending: false };
  },
}));

let previewState = {
  data: undefined as RegistrationExam | undefined,
  isLoading: false,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

vi.mock("@/lib/hooks/admin-exam-preview", () => ({
  useExamPreview: () => previewState,
}));

const exam: RegistrationExam = {
  id: "exam-1",
  title: "Try Out UTBK Nasional",
  scheduled_at: "2026-07-15T02:00:00Z",
  scheduled_end_at: "2026-07-15T04:00:00Z",
  requires_checkin: true,
  check_in_window_minutes: 30,
  timer_mode: "per_test",
  navigation_policy: "sequential",
  max_attempts: 2,
  duration_minutes: 120,
  result_config: "{}",
  card_enabled: true,
  card_notes: [],
  section_count: 1,
  question_count: 30,
  sections_duration_minutes: 60,
  sections: [
    { test_id: "t-1", title: "Penalaran Umum", duration_minutes: 60, question_count: 30, question_presentation: "one_per_page" },
  ],
};

describe("ExamPreviewPage", () => {
  beforeEach(() => {
    uiStore = { lang: "id" };
    startSession.mockClear();
    checkIn.mockClear();
    previewState = { data: exam, isLoading: false, isError: false, error: null, refetch: vi.fn() };
  });

  it("renders the student briefing read-only with start and check-in disabled", () => {
    render(<ExamPreviewPage />);

    expect(screen.getByRole("heading", { name: "Try Out UTBK Nasional" })).toBeInTheDocument();
    expect(screen.getByText(/pratinjau tampilan siswa/i)).toBeInTheDocument();
    expect(screen.getByText("Penalaran Umum")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /kembali ke detail ujian/i })).toHaveAttribute(
      "href",
      "/admin/exam/packages/exam-1",
    );

    const start = screen.getByRole("button", { name: /pratinjau: mulai dinonaktifkan/i });
    expect(start).toBeDisabled();
    expect(screen.getByRole("button", { name: /pratinjau: check-in dinonaktifkan/i })).toBeDisabled();
    expect(screen.queryByRole("button", { name: /^mulai ujian$/i })).not.toBeInTheDocument();
  });

  it("never wires the student start or check-in mutations", () => {
    render(<ExamPreviewPage />);

    expect(startSession).not.toHaveBeenCalled();
    expect(checkIn).not.toHaveBeenCalled();
  });

  it("shows an error state with retry when the preview fails to load", () => {
    previewState = { data: undefined, isLoading: false, isError: true, error: new Error("boom"), refetch: vi.fn() };
    render(<ExamPreviewPage />);

    expect(screen.getByText(/gagal memuat data ujian/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /coba lagi/i })).toBeInTheDocument();
  });
});
