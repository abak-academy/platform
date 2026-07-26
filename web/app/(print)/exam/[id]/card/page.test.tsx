import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

import ExamCardPrintPage from "./page";
import type { RegistrationDetail } from "@/lib/types";

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "reg-1" }),
}));

const { toastError } = vi.hoisted(() => ({ toastError: vi.fn() }));
vi.mock("sonner", () => ({ toast: { error: toastError } }));

vi.mock("@/stores/auth", () => ({
  useAuthStore: Object.assign(() => null, {
    getState: () => ({ token: "test-token" }),
  }),
}));

let registrationState = {
  data: null as RegistrationDetail | null,
  isLoading: true,
  isError: false,
};

let profileState = {
  data: { name: "Saifullah Panca" } as Record<string, unknown>,
  isLoading: false,
};

vi.mock("@/lib/hooks/exam", () => ({
  useRegistration: () => registrationState,
}));

vi.mock("@/lib/hooks/students", () => ({
  useProfile: () => profileState,
  useSchools: () => ({ data: [] }),
}));

const registration: RegistrationDetail = {
  id: "reg-1",
  student_id: "s-1",
  exam_id: "e-1",
  token: "ABC12345",
  card_key: null,
  checked_in_at: null,
  attempts_used: 0,
  status: "checked_in",
  created_at: "2026-06-01T00:00:00Z",
  participant_number: 5,
  participant_no: "260601-0001-000005",
  subject: "Matematika",
  platform: "exam.abakacademy.id",
  exam: {
    id: "e-1",
    title: "Ujian Simulasi UTBK",
    scheduled_at: "2026-08-01T02:00:00Z",
    scheduled_end_at: null,
    requires_checkin: true,
    check_in_window_minutes: 15,
    timer_mode: "fixed",
    duration_minutes: 90,
    result_config: "full",
  },
};

describe("ExamCardPrintPage — Download PDF", () => {
  beforeEach(() => {
    toastError.mockClear();
    registrationState = { data: registration, isLoading: false, isError: false };
    profileState = { data: { name: "Saifullah Panca" }, isLoading: false };
    global.fetch = vi.fn();
    global.URL.createObjectURL = vi.fn(() => "blob:mock-url");
    global.URL.revokeObjectURL = vi.fn();
  });

  it("fetches the FR-30 card endpoint with a bearer token and triggers a download", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      blob: async () => new Blob(["%PDF-"], { type: "application/pdf" }),
    });

    render(<ExamCardPrintPage />);

    await waitFor(() => {
      expect(screen.getByText("Ujian Simulasi UTBK")).toBeInTheDocument();
    });

    screen.getByRole("button", { name: /Download PDF/i }).click();

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining("/exam/registrations/reg-1/card"),
        expect.objectContaining({
          headers: { Authorization: "Bearer test-token" },
        })
      );
    });
  });

  it("shows an error toast when the download fails", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 500,
    });

    render(<ExamCardPrintPage />);

    await waitFor(() => {
      expect(screen.getByText("Ujian Simulasi UTBK")).toBeInTheDocument();
    });

    screen.getByRole("button", { name: /Download PDF/i }).click();

    await waitFor(() => {
      expect(toastError).toHaveBeenCalled();
    });
  });

  // C-01: grade arrives as a number (backend *int). The card must render it
  // without crashing — the pre-fix code called grade.trim() on a number.
  it("renders a numeric grade without crashing", async () => {
    profileState = { data: { name: "Saifullah Panca", grade: 10 }, isLoading: false };

    render(<ExamCardPrintPage />);

    await waitFor(() => {
      expect(screen.getByText("Ujian Simulasi UTBK")).toBeInTheDocument();
    });
    expect(screen.getByText("10")).toBeInTheDocument();
  });

  // C-06: scheduled_at is 02:00Z; the card labels the time "WIB", so it must be
  // formatted in Asia/Jakarta (09.00) regardless of the runner's timezone —
  // not the runner-local hour.
  it("formats the schedule time in WIB, not the runner timezone", async () => {
    render(<ExamCardPrintPage />);

    await waitFor(() => {
      expect(screen.getByText("Ujian Simulasi UTBK")).toBeInTheDocument();
    });
    const waktu = screen.getByText(/WIB/);
    expect(waktu.textContent).toMatch(/09[.:]00/);
    expect(waktu.textContent).not.toMatch(/02[.:]00/);
  });
});
