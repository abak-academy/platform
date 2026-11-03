import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";

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
  navigation_policy: "free",
  timer_mode: "overall",
  section_count: 2,
  question_count: 40,
  sections_duration_minutes: 90,
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
  navigation_policy: "sequential",
  timer_mode: "per_test",
  section_count: 3,
  question_count: 60,
  sections_duration_minutes: 75,
};

const sample: RegistrationListItem[] = [freeUpcoming, paidNoSchedule];

describe("ExamPage", () => {
  beforeEach(() => {
    uiStore = { lang: "id" };
    pushMock.mockClear();
    startSessionMutate.mockReset();
    toastError.mockReset();
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

  it("offers the rules page, not a start button, for a ready exam without check-in", async () => {
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Try Out UTBK Gratis #12")).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: /Mulai/i })).not.toBeInTheDocument();
    const hrefs = screen.getAllByRole("link", { name: /Lihat aturan ujian/i }).map((a) => a.getAttribute("href"));
    expect(hrefs).toContain("/exam/reg-1");
    expect(startSessionMutate).not.toHaveBeenCalled();
  });

  it("offers the rules page instead of an inline token input when check-in is required", async () => {
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Ujian Akhir Matematika")).toBeInTheDocument();
    });
    expect(screen.queryByPlaceholderText("Token dari kartu ujian")).not.toBeInTheDocument();
    const card = screen.getByText("Ujian Akhir Matematika").closest("[data-slot='card']") as HTMLElement;
    expect(within(card).getByRole("link", { name: /Lihat aturan ujian/i })).toHaveAttribute("href", "/exam/reg-2");
  });

  it("shows time, section and question facts with the navigation rule", async () => {
    render(<ExamPage />);

    const card = (await screen.findByText("Ujian Akhir Matematika")).closest("[data-slot='card']") as HTMLElement;
    expect(within(card).getByText("Total waktu")).toBeInTheDocument();
    expect(within(card).getByText("75")).toBeInTheDocument();
    expect(within(card).getByText("Timer per section")).toBeInTheDocument();
    expect(within(card).getByText("3")).toBeInTheDocument();
    expect(within(card).getByText("60")).toBeInTheDocument();
    expect(within(card).getByText("Section berurutan")).toBeInTheDocument();

    const free = (await screen.findByText("Try Out UTBK Gratis #12")).closest("[data-slot='card']") as HTMLElement;
    expect(within(free).getByText("Waktu pengerjaan")).toBeInTheDocument();
    expect(within(free).getByText("90")).toBeInTheDocument();
    expect(within(free).getByText("Satu timer ujian")).toBeInTheDocument();
    expect(within(free).getByText("Bebas pindah section")).toBeInTheDocument();
  });

  it("shows a same-day schedule as one date and a time range, and a cross-day one as two dates", async () => {
    registrationsState = {
      data: [
        { ...freeUpcoming, id: "same", exam_title: "Satu Hari", scheduled_at: "2026-07-15T02:00:00Z", scheduled_end_at: "2026-07-15T04:00:00Z" },
        { ...freeUpcoming, id: "cross", exam_title: "Lintas Hari", scheduled_at: "2026-07-15T02:00:00Z", scheduled_end_at: "2026-07-17T04:00:00Z" },
      ],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    const same = (await screen.findByText("Satu Hari")).closest("[data-slot='card']") as HTMLElement;
    expect(same.textContent?.match(/15 Jul 2026/g)).toHaveLength(1);
    expect(within(same).getByText(/09[.:]00.*11[.:]00/)).toBeInTheDocument();

    const cross = screen.getByText("Lintas Hari").closest("[data-slot='card']") as HTMLElement;
    expect(within(cross).getByText(/15 Jul 2026/)).toBeInTheDocument();
    expect(within(cross).getByText(/17 Jul 2026/)).toBeInTheDocument();
  });

  it("shows a neutral not-open badge, countdown and detail link for an upcoming exam", async () => {
    registrationsState = {
      data: [
        {
          ...paidNoSchedule,
          id: "reg-up",
          exam_title: "Tryout Belum Buka",
          scheduled_at: new Date(Date.now() + 3 * 60 * 60_000).toISOString(),
          check_in_window_minutes: 15,
        },
      ],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    const card = (await screen.findByText("Tryout Belum Buka")).closest("[data-slot='card']") as HTMLElement;
    expect(within(card).getByText("Belum dibuka")).toBeInTheDocument();
    expect(within(card).getByText(/Dibuka dalam 2 jam/)).toBeInTheDocument();
    expect(within(card).getByRole("link", { name: "Lihat Detail" })).toHaveAttribute("href", "/exam/reg-up");
    expect(within(card).queryByRole("link", { name: /Lihat aturan ujian/i })).not.toBeInTheDocument();
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
    // scheduled_at is in the past — the window keeps the exam ready instead.
    expect(screen.getByRole("link", { name: /Lihat aturan ujian/i })).toBeInTheDocument();
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
    expect(screen.queryByRole("button", { name: "Lanjutkan ujian" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Lihat Detail/i })).toHaveAttribute("href", "/exam/reg-7");
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

  it("offers a retake when max_attempts is null (unlimited)", async () => {
    const submittedUnlimited: RegistrationListItem = {
      ...paidNoSchedule,
      id: "reg-10",
      exam_title: "Tryout Tidak Terbatas",
      status: "submitted",
      session_id: "sess-10",
      attempts_used: 3,
      max_attempts: null,
    };
    registrationsState = {
      data: [submittedUnlimited],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout Tidak Terbatas")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /Ulangi ujian/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Lihat hasil/i })).toBeInTheDocument();
  });

  it("treats max_attempts 0 as single-attempt: no retake after the one attempt", async () => {
    const submittedSingleAttemptZero: RegistrationListItem = {
      ...paidNoSchedule,
      id: "reg-11",
      exam_title: "Tryout Sekali Percobaan Zero",
      status: "submitted",
      session_id: "sess-11",
      attempts_used: 1,
      max_attempts: 0,
    };
    registrationsState = {
      data: [submittedSingleAttemptZero],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    await waitFor(() => {
      expect(screen.getByText("Tryout Sekali Percobaan Zero")).toBeInTheDocument();
    });
    expect(screen.getByRole("link", { name: /Lihat hasil/i })).toBeInTheDocument();
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

      const ready = label === "checkin" || label === "start";
      const link = screen.getByRole("link", { name: ready ? /Lihat aturan ujian/i : /Lihat Detail/i });
      expect(link, `state "${label}" should expose a route to the card`).toHaveAttribute(
        "href",
        `/exam/${reg.id}`,
      );

      unmount();
    }
  });

  it("resumes an in_progress exam directly from the card", async () => {
    startSessionMutate.mockResolvedValue({ session_id: "sess-live" });
    registrationsState = {
      data: [{ ...paidNoSchedule, id: "reg-live", exam_title: "Tryout Live", status: "in_progress" }],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Lanjutkan ujian" }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/exam/sessions/sess-live"));
    expect(startSessionMutate).toHaveBeenCalledWith("reg-live");
    expect(screen.getByRole("link", { name: /Lihat Detail/i })).toHaveAttribute("href", "/exam/reg-live");
  });

  it("shows the server error when resuming is rejected", async () => {
    startSessionMutate.mockRejectedValue(new Error("device mismatch"));
    registrationsState = {
      data: [{ ...paidNoSchedule, id: "reg-live", exam_title: "Tryout Live", status: "in_progress" }],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Lanjutkan ujian" }));

    await waitFor(() => expect(toastError).toHaveBeenCalledWith("device mismatch"));
    expect(pushMock).not.toHaveBeenCalled();
  });

  it("shows the continue label in English", async () => {
    uiStore = { lang: "en" };
    registrationsState = {
      data: [{ ...paidNoSchedule, status: "in_progress" }],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamPage />);

    expect(await screen.findByRole("button", { name: "Continue exam" })).toBeInTheDocument();
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
