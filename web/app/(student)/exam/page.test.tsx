import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

import ExamPage from "./page";
import type { RegistrationListItem } from "@/lib/types";

const { pushMock } = vi.hoisted(() => ({ pushMock: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

let uiStore = { lang: "id" as "id" | "en" };

vi.mock("@/stores/ui", () => ({
  useUIStore: (selector: (s: typeof uiStore) => unknown) => selector(uiStore),
}));

const { toastSuccess, toastError } = vi.hoisted(() => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: toastError },
}));

const checkInMutate = vi.fn();
const startSessionMutate = vi.fn();

let registrationsState = {
  data: null as RegistrationListItem[] | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

vi.mock("@/lib/hooks/exam", () => ({
  useRegistrations: () => registrationsState,
  useCheckIn: () => ({ mutate: checkInMutate, isPending: false }),
  useStartSession: () => ({ mutateAsync: startSessionMutate, isPending: false }),
}));

const freeUpcoming: RegistrationListItem = {
  id: "reg-1",
  student_id: "s-1",
  exam_id: "e-1",
  token: "ABC12345",
  card_key: null,
  checked_in_at: null,
  attempts_used: 0,
  status: "registered",
  created_at: "2026-06-01T00:00:00Z",
  exam_title: "Try Out UTBK Gratis #12",
  scheduled_at: "2026-07-15T09:00:00Z",
  scheduled_end_at: null,
  is_free: true,
  requires_checkin: false,
  check_in_window_minutes: null,
  duration_minutes: 90,
  session_id: null,
  max_attempts: null,
};

const paidNoSchedule: RegistrationListItem = {
  id: "reg-2",
  student_id: "s-1",
  exam_id: "e-2",
  token: "XYZ98765",
  card_key: null,
  checked_in_at: null,
  attempts_used: 0,
  status: "registered",
  created_at: "2026-06-02T00:00:00Z",
  exam_title: "Ujian Akhir Matematika",
  scheduled_at: null,
  scheduled_end_at: null,
  is_free: false,
  requires_checkin: true,
  check_in_window_minutes: 15,
  duration_minutes: 60,
  session_id: null,
  max_attempts: null,
};

const sample: RegistrationListItem[] = [freeUpcoming, paidNoSchedule];

describe("ExamPage", () => {
  beforeEach(() => {
    uiStore = { lang: "id" };
    pushMock.mockClear();
    registrationsState = {
      data: sample,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
  });

  it("renders the competition list with exam titles grouped by free/paid", async () => {
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Try Out UTBK Gratis #12")).toBeInTheDocument();
      expect(screen.getByText("Ujian Akhir Matematika")).toBeInTheDocument();
    });
    expect(screen.getByText("Paket gratis")).toBeInTheDocument();
    expect(screen.getByText("Paket saya")).toBeInTheDocument();
  });

  it("translates copy when language is en", () => {
    uiStore = { lang: "en" };
    render(<ExamPage />);

    expect(
      screen.getByRole("heading", { name: /Competition & Tryout/i })
    ).toBeInTheDocument();
  });

  it("shows a start button for a free exam with no check-in requirement", async () => {
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Try Out UTBK Gratis #12")).toBeInTheDocument();
    });
    expect(screen.getAllByRole("button", { name: /Mulai/i }).length).toBeGreaterThan(0);
  });

  it("shows the check-in token input when no schedule is set but check-in is required", async () => {
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Ujian Akhir Matematika")).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText("Token dari kartu ujian")).toBeInTheDocument();
  });

  it("shows an empty state when there are no registrations", async () => {
    registrationsState = {
      data: [],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Anda belum terdaftar pada ujian apapun")).toBeInTheDocument();
    });
  });

  it("keeps check-in open past scheduled_at when scheduled_end_at is set (availability window)", async () => {
    const windowedReg: RegistrationListItem = {
      ...paidNoSchedule,
      id: "reg-3",
      exam_title: "Tryout Terbuka",
      scheduled_at: new Date(Date.now() - 60 * 60_000).toISOString(), // 1h ago
      scheduled_end_at: new Date(Date.now() + 60 * 60_000).toISOString(), // 1h from now
    };
    registrationsState = {
      data: [windowedReg],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout Terbuka")).toBeInTheDocument();
    });
    // Without a window this would already show "Kadaluarsa" (expired) since
    // scheduled_at is in the past — the window keeps check-in open instead.
    expect(screen.getByPlaceholderText("Token dari kartu ujian")).toBeInTheDocument();
    expect(screen.queryByText("Kadaluarsa")).not.toBeInTheDocument();
  });

  it("shows expired once scheduled_end_at has passed", async () => {
    const expiredWindowReg: RegistrationListItem = {
      ...paidNoSchedule,
      id: "reg-4",
      exam_title: "Tryout Sudah Tutup",
      scheduled_at: new Date(Date.now() - 3 * 60 * 60_000).toISOString(), // 3h ago
      scheduled_end_at: new Date(Date.now() - 60 * 60_000).toISOString(), // 1h ago
    };
    registrationsState = {
      data: [expiredWindowReg],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout Sudah Tutup")).toBeInTheDocument();
    });
    expect(screen.getAllByText("Kadaluarsa").length).toBeGreaterThan(0);
  });

  it("navigates to the printable card page when download card is clicked", async () => {
    const lockedReg: RegistrationListItem = {
      ...paidNoSchedule,
      id: "reg-5",
      exam_title: "Tryout Terkunci",
      requires_checkin: true,
      scheduled_at: new Date(Date.now() + 60 * 60_000).toISOString(), // 1h from now
      check_in_window_minutes: 15,
    };
    registrationsState = {
      data: [lockedReg],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout Terkunci")).toBeInTheDocument();
    });
    screen.getByRole("button", { name: /Unduh kartu/i }).click();
    expect(pushMock).toHaveBeenCalledWith("/exam/reg-5/card");
  });

  it("shows a Lihat hasil action linking to the result page when submitted with a session", async () => {
    const submittedReg: RegistrationListItem = {
      ...paidNoSchedule,
      id: "reg-6",
      exam_title: "Tryout Sudah Selesai",
      status: "submitted",
      session_id: "sess-42",
    };
    registrationsState = {
      data: [submittedReg],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout Sudah Selesai")).toBeInTheDocument();
    });
    const link = screen.getByRole("link", { name: /Lihat hasil/i });
    expect(link).toHaveAttribute("href", "/exam/sessions/sess-42/result");
  });

  it("does not render a broken result link when submitted but session_id is null", async () => {
    const submittedNoSession: RegistrationListItem = {
      ...paidNoSchedule,
      id: "reg-7",
      exam_title: "Tryout Tanpa Sesi",
      status: "submitted",
      session_id: null,
    };
    registrationsState = {
      data: [submittedNoSession],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout Tanpa Sesi")).toBeInTheDocument();
    });
    expect(screen.queryByRole("link", { name: /Lihat hasil/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/exam\/sessions\/null/)).not.toBeInTheDocument();
  });

  it("offers a retake alongside the result link when attempts remain (FR20)", async () => {
    const submittedWithAttemptsLeft: RegistrationListItem = {
      ...paidNoSchedule,
      id: "reg-8",
      exam_title: "Tryout Bisa Diulang",
      status: "submitted",
      session_id: "sess-8",
      attempts_used: 1,
      max_attempts: 3,
    };
    registrationsState = {
      data: [submittedWithAttemptsLeft],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout Bisa Diulang")).toBeInTheDocument();
    });
    expect(screen.getByRole("link", { name: /Lihat hasil/i })).toHaveAttribute(
      "href",
      "/exam/sessions/sess-8/result",
    );
    const retakeButton = screen.getByRole("button", { name: /Ulangi ujian/i });
    retakeButton.click();
    expect(startSessionMutate).toHaveBeenCalledWith("reg-8");
  });

  it("offers only the result link when attempts are exhausted (FR21)", async () => {
    const submittedExhausted: RegistrationListItem = {
      ...paidNoSchedule,
      id: "reg-9",
      exam_title: "Tryout Sudah Habis",
      status: "submitted",
      session_id: "sess-9",
      attempts_used: 3,
      max_attempts: 3,
    };
    registrationsState = {
      data: [submittedExhausted],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout Sudah Habis")).toBeInTheDocument();
    });
    expect(screen.getByRole("link", { name: /Lihat hasil/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Ulangi ujian/i })).not.toBeInTheDocument();
  });

  it("treats max_attempts null/0 as single-attempt: no retake after the one attempt (FR18/FR21)", async () => {
    const submittedSingleAttempt: RegistrationListItem = {
      ...paidNoSchedule,
      id: "reg-10",
      exam_title: "Tryout Sekali Percobaan",
      status: "submitted",
      session_id: "sess-10",
      attempts_used: 1,
      max_attempts: null,
    };
    const submittedSingleAttemptZero: RegistrationListItem = {
      ...submittedSingleAttempt,
      id: "reg-11",
      exam_title: "Tryout Sekali Percobaan Zero",
      session_id: "sess-11",
      max_attempts: 0,
    };
    registrationsState = {
      data: [submittedSingleAttempt, submittedSingleAttemptZero],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout Sekali Percobaan")).toBeInTheDocument();
      expect(screen.getByText("Tryout Sekali Percobaan Zero")).toBeInTheDocument();
    });
    expect(screen.getAllByRole("link", { name: /Lihat hasil/i })).toHaveLength(2);
    expect(screen.queryByRole("button", { name: /Ulangi ujian/i })).not.toBeInTheDocument();
  });

  it("exposes a route to the card from every registration state (FR-45)", async () => {
    const cases: Array<{ label: string; reg: RegistrationListItem }> = [
      {
        label: "locked",
        reg: {
          ...paidNoSchedule,
          id: "reg-locked",
          exam_title: "Tryout Status Locked",
          scheduled_at: new Date(Date.now() + 60 * 60_000).toISOString(),
          check_in_window_minutes: 15,
        },
      },
      {
        label: "checkin",
        reg: {
          ...paidNoSchedule,
          id: "reg-checkin",
          exam_title: "Tryout Status Checkin",
        },
      },
      {
        label: "start",
        reg: {
          ...paidNoSchedule,
          id: "reg-start",
          exam_title: "Tryout Status Start",
          status: "checked_in",
        },
      },
      {
        label: "in_progress",
        reg: {
          ...paidNoSchedule,
          id: "reg-in-progress",
          exam_title: "Tryout Status In Progress",
          status: "in_progress",
        },
      },
      {
        label: "submitted",
        reg: {
          ...paidNoSchedule,
          id: "reg-submitted",
          exam_title: "Tryout Status Submitted",
          status: "submitted",
          session_id: "sess-submitted",
        },
      },
    ];

    for (const { label, reg } of cases) {
      registrationsState = {
        data: [reg],
        isLoading: false,
        isError: false,
        error: null,
        refetch: vi.fn(),
      };
      const { unmount } = render(<ExamPage />);

      await waitFor(() => {
        expect(screen.getByText(reg.exam_title)).toBeInTheDocument();
      });

      const link = screen.getByRole("link", { name: /Lihat Detail/i });
      expect(link, `state "${label}" should expose a route to the card`).toHaveAttribute(
        "href",
        `/exam/${reg.id}`,
      );

      unmount();
    }
  });

  it("shows an error state with a retry action", async () => {
    const refetch = vi.fn();
    registrationsState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("network down"),
      refetch,
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText(/Gagal memuat data ujian/)).toBeInTheDocument();
    });
    screen.getByRole("button", { name: "Coba lagi" }).click();
    expect(refetch).toHaveBeenCalled();
  });
});
