import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";

import ExamDetailPage from "./page";
import type { RegistrationDetail } from "@/lib/types";

const { mockPush } = vi.hoisted(() => ({ mockPush: vi.fn() }));

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "reg-1" }),
  useRouter: () => ({ push: mockPush }),
}));

let uiStore = { lang: "id" as "id" | "en" };

vi.mock("@/stores/ui", () => ({
  useUIStore: (selector: (s: typeof uiStore) => unknown) => selector(uiStore),
}));

const { toastSuccess, toastError } = vi.hoisted(() => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));

const checkInMutate = vi.fn();
const startSessionMutate = vi.fn();

let registrationState = {
  data: null as RegistrationDetail | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

vi.mock("@/lib/hooks/exam", () => ({
  useRegistration: () => registrationState,
  useCheckIn: () => ({ mutate: checkInMutate, isPending: false }),
  useStartSession: () => ({ mutateAsync: startSessionMutate, isPending: false }),
}));

vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: toastError },
}));

const sampleWithCheckin: RegistrationDetail = {
  id: "reg-1",
  student_id: "s-1",
  exam_id: "e-1",
  token: "AB12CD34",
  card_key: null,
  checked_in_at: null,
  attempts_used: 0,
  status: "registered",
  created_at: "2026-06-01T00:00:00Z",
  participant_number: 1,
  participant_no: "260715-000001",
  subject: "TPS — Penalaran Umum",
  platform: "exam.abakacademy.id",
  exam: {
    id: "e-1",
    title: "Try Out UTBK Nasional",
    scheduled_at: "2026-07-15T09:00:00Z",
    scheduled_end_at: null,
    requires_checkin: true,
    check_in_window_minutes: 30,
    timer_mode: "overall",
    duration_minutes: 120,
    result_config: "{}",
  },
};

const sampleNoCheckin: RegistrationDetail = {
  ...sampleWithCheckin,
  exam: { ...sampleWithCheckin.exam, requires_checkin: false },
};

describe("ExamDetailPage", () => {
  beforeEach(() => {
    uiStore = { lang: "id" };
    mockPush.mockReset();
    checkInMutate.mockReset();
    startSessionMutate.mockReset();
    toastSuccess.mockReset();
    toastError.mockReset();
    registrationState = {
      data: sampleWithCheckin,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
  });

  it("shows exam title and token masked by default (FR12)", async () => {
    render(<ExamDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: /Try Out UTBK Nasional/i })
      ).toBeInTheDocument();
    });

    // Token should be masked — not the raw value
    expect(screen.queryByText("AB12CD34")).not.toBeInTheDocument();
    expect(screen.getByText("••••••••")).toBeInTheDocument();
  });

  it("token toggle reveals and then re-hides the token (FR12)", async () => {
    render(<ExamDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("••••••••")).toBeInTheDocument();
    });

    const showBtn = screen.getByRole("button", { name: /tampilkan token/i });
    fireEvent.click(showBtn);

    expect(screen.getByText("AB12CD34")).toBeInTheDocument();
    expect(screen.queryByText("••••••••")).not.toBeInTheDocument();

    const hideBtn = screen.getByRole("button", { name: /sembunyikan token/i });
    fireEvent.click(hideBtn);

    expect(screen.queryByText("AB12CD34")).not.toBeInTheDocument();
    expect(screen.getByText("••••••••")).toBeInTheDocument();
  });

  it("card button opens the printable card page for the registration (FR12)", async () => {
    render(<ExamDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /unduh kartu peserta/i })
      ).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /unduh kartu peserta/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/exam/reg-1/card");
    });
  });

  it("renders check-in section when requires_checkin=true (FR12)", async () => {
    render(<ExamDetailPage />);

    await waitFor(() => {
      expect(screen.getByText(/petunjuk check-in/i)).toBeInTheDocument();
    });
  });

  it("does NOT render check-in section when requires_checkin=false (FR12)", async () => {
    registrationState = {
      ...registrationState,
      data: sampleNoCheckin,
    };
    render(<ExamDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: /Try Out UTBK Nasional/i })
      ).toBeInTheDocument();
    });

    expect(screen.queryByText(/petunjuk check-in/i)).not.toBeInTheDocument();
  });

  it("shows not-found message when data is null and not loading", async () => {
    registrationState = {
      data: null,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<ExamDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/pendaftaran tidak ditemukan/i)
      ).toBeInTheDocument();
    });
  });

  it("shows error card when isError=true", async () => {
    registrationState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("not found"),
      refetch: vi.fn(),
    };
    render(<ExamDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/gagal memuat data ujian/i)
      ).toBeInTheDocument();
    });
  });
});

// ── Check-in form (FR27) ────────────────────────────────────────────────────
describe("ExamDetailPage — check-in form (FR27)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    uiStore = { lang: "id" };
    checkInMutate.mockReset();
    toastSuccess.mockReset();
    toastError.mockReset();
    registrationState = {
      data: sampleWithCheckin,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows token input and enabled check-in button inside the window", () => {
    // Window: 08:30–09:00; set time to 08:45 (inside)
    vi.setSystemTime("2026-07-15T08:45:00Z");
    render(<ExamDetailPage />);

    const input = screen.getByPlaceholderText(/token dari kartu ujian/i);
    fireEvent.change(input, { target: { value: "MYTOKEN" } });

    expect(screen.getByText(/check-in ujian/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /check-in/i })).not.toBeDisabled();
  });

  it("shows disabled form with 'check-in belum dibuka' before window opens", () => {
    // Before window: 08:00
    vi.setSystemTime("2026-07-15T08:00:00Z");
    render(<ExamDetailPage />);

    expect(screen.getByText(/check-in belum dibuka/i)).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText(/token dari kartu ujian/i)
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: /check-in/i })).toBeDisabled();
  });

  it("shows disabled form with 'check-in sudah ditutup' after window closes", () => {
    // After window: 09:30
    vi.setSystemTime("2026-07-15T09:30:00Z");
    render(<ExamDetailPage />);

    expect(screen.getByText(/check-in sudah ditutup/i)).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText(/token dari kartu ujian/i)
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: /check-in/i })).toBeDisabled();
  });

  it("does not show check-in form when status is not 'registered' (already checked in)", () => {
    registrationState = {
      ...registrationState,
      data: { ...sampleWithCheckin, status: "checked_in", checked_in_at: "2026-07-15T08:45:00Z" },
    };
    vi.setSystemTime("2026-07-15T08:45:00Z");
    render(<ExamDetailPage />);

    expect(screen.queryByText(/check-in ujian/i)).not.toBeInTheDocument();
    expect(
      screen.queryByPlaceholderText(/token dari kartu ujian/i)
    ).not.toBeInTheDocument();
  });

  it("calls check-in mutation with token on submit", () => {
    vi.setSystemTime("2026-07-15T08:45:00Z");
    render(<ExamDetailPage />);

    const input = screen.getByPlaceholderText(/token dari kartu ujian/i);
    fireEvent.change(input, { target: { value: "MYTOKEN" } });
    fireEvent.click(screen.getByRole("button", { name: /check-in/i }));

    expect(checkInMutate).toHaveBeenCalledWith(
      { token: "MYTOKEN" },
      expect.any(Object)
    );
  });

  it("shows success toast when check-in succeeds", () => {
    vi.setSystemTime("2026-07-15T08:45:00Z");
    render(<ExamDetailPage />);

    const input = screen.getByPlaceholderText(/token dari kartu ujian/i);
    fireEvent.change(input, { target: { value: "MYTOKEN" } });
    fireEvent.click(screen.getByRole("button", { name: /check-in/i }));

    const [, options] = checkInMutate.mock.calls[0];
    options.onSuccess();

    expect(toastSuccess).toHaveBeenCalledWith(
      expect.stringMatching(/token diterima/i)
    );
  });

  it("transitions UI after check-in success (form hides, start gate appears)", async () => {
    vi.setSystemTime("2026-07-15T08:45:00Z");
    const { rerender } = render(<ExamDetailPage />);

    // Check-in form visible
    expect(
      screen.getByPlaceholderText(/token dari kartu ujian/i)
    ).toBeInTheDocument();

    // Submit check-in
    fireEvent.change(
      screen.getByPlaceholderText(/token dari kartu ujian/i),
      { target: { value: "MYTOKEN" } }
    );
    fireEvent.click(screen.getByRole("button", { name: /check-in/i }));

    // Simulate success callback
    const [, options] = checkInMutate.mock.calls[0];
    options.onSuccess();

    // Update mock to checked_in state and rerender to simulate refetch
    registrationState = {
      ...registrationState,
      data: {
        ...sampleWithCheckin,
        status: "checked_in",
        checked_in_at: "2026-07-15T08:45:00Z",
      },
    };
    rerender(<ExamDetailPage />);

    // Check-in form gone (synchronous after rerender with fake timers)
    expect(
      screen.queryByPlaceholderText(/token dari kartu ujian/i)
    ).not.toBeInTheDocument();
    // Start gate shown
    expect(
      screen.getByRole("button", { name: /mulai ujian/i })
    ).toBeInTheDocument();
  });
});

// ── Start gate (FR28) ───────────────────────────────────────────────────────
describe("ExamDetailPage — start gate (FR28)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    uiStore = { lang: "id" };
    startSessionMutate.mockReset();
    toastError.mockReset();
    registrationState = {
      data: sampleWithCheckin,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows enabled 'Mulai Ujian' button when checked_in and now >= scheduled_at", () => {
    registrationState = {
      ...registrationState,
      data: {
        ...sampleWithCheckin,
        status: "checked_in",
        checked_in_at: "2026-07-15T08:45:00Z",
      },
    };
    vi.setSystemTime("2026-07-15T09:00:00Z");
    render(<ExamDetailPage />);

    const btn = screen.getByRole("button", { name: /mulai ujian/i });
    expect(btn).toBeInTheDocument();
    expect(btn).not.toBeDisabled();
  });

  it("shows disabled 'Mulai Ujian' with 'Belum dimulai' when checked_in and now < scheduled_at", () => {
    registrationState = {
      ...registrationState,
      data: {
        ...sampleWithCheckin,
        status: "checked_in",
        checked_in_at: "2026-07-15T08:45:00Z",
      },
    };
    vi.setSystemTime("2026-07-15T08:55:00Z");
    render(<ExamDetailPage />);

    const btn = screen.getByRole("button", { name: /mulai ujian/i });
    expect(btn).toBeDisabled();
    expect(screen.getByText(/belum dimulai/i)).toBeInTheDocument();
  });

  it("does not show start gate when status is 'registered' (before check-in)", () => {
    registrationState = {
      ...registrationState,
      data: sampleWithCheckin, // status: "registered"
    };
    vi.setSystemTime("2026-07-15T09:00:00Z");
    render(<ExamDetailPage />);

    expect(screen.queryByRole("button", { name: /mulai ujian/i })).not.toBeInTheDocument();
  });

  it("shows enabled 'Mulai Ujian' immediately when requires_checkin=false", () => {
    registrationState = {
      ...registrationState,
      data: sampleNoCheckin,
    };
    // Any time — even well before scheduled_at
    vi.setSystemTime("2026-06-01T00:00:00Z");
    render(<ExamDetailPage />);

    const btn = screen.getByRole("button", { name: /mulai ujian/i });
    expect(btn).not.toBeDisabled();
  });

  it("calls start session mutation on click", () => {
    registrationState = {
      ...registrationState,
      data: {
        ...sampleWithCheckin,
        status: "checked_in",
        checked_in_at: "2026-07-15T08:45:00Z",
      },
    };
    vi.setSystemTime("2026-07-15T09:00:00Z");

    startSessionMutate.mockResolvedValue({ session_id: "sess-1", remaining_seconds: 7200, timer_mode: "overall", tests: [] });

    render(<ExamDetailPage />);
    fireEvent.click(screen.getByRole("button", { name: /mulai ujian/i }));

    expect(startSessionMutate).toHaveBeenCalledWith("reg-1");
  });
});
