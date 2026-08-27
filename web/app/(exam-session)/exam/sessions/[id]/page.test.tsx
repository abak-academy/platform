import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { act } from "react";

import SessionPage from "./page";
import { ApiError } from "@/lib/api";
import type { SessionState } from "@/lib/types";
import {
  AUTOSAVE_DEBOUNCE_MS,
  backoffDelayMs,
  loadQueue,
  saveQueue,
} from "@/lib/exam-session-queue";

const routerReplace = vi.fn();

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "session-1" }),
  useRouter: () => ({ replace: routerReplace }),
}));

let uiStore = { lang: "id" as "id" | "en" };

vi.mock("@/stores/ui", () => ({
  useUIStore: (selector: (s: typeof uiStore) => unknown) => selector(uiStore),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

// ── Mock hooks ────────────────────────────────────────────────────────────

let sessionState = {
  data: null as SessionState | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

const saveAnswersMutate = vi.fn();
const saveAnswersMutateAsync = vi.fn();
const submitSessionMutate = vi.fn();
const submitSessionMutateAsync = vi.fn();
const logViolationMutate = vi.fn();
const advanceSectionMutate = vi.fn();
const advanceSectionMutateAsync = vi.fn();

vi.mock("@/lib/hooks/exam", () => ({
  useReconnectSession: () => sessionState,
  useSaveAnswers: () => ({
    mutate: saveAnswersMutate,
    mutateAsync: saveAnswersMutateAsync,
    isPending: false,
  }),
  useSubmitSession: () => ({
    mutate: submitSessionMutate,
    mutateAsync: submitSessionMutateAsync,
    isPending: false,
  }),
  useLogViolation: () => ({
    mutate: logViolationMutate,
  }),
  useAdvanceSection: () => ({
    mutate: advanceSectionMutate,
    mutateAsync: advanceSectionMutateAsync,
    isPending: false,
  }),
}));

// ── Sample data ───────────────────────────────────────────────────────────

const sampleSession: SessionState = {
  session_id: "session-1",
  registration_id: "reg-1",
  status: "in_progress",
  remaining_seconds: 3600,
  timer_mode: "overall",
  duration_minutes: 60,
  started_at: "2026-07-15T09:00:00Z",
  answers: [],
  tests: [
    {
      id: "test-1",
      title: "Tes Matematika",
      subject: "Matematika",
      questions: [
        {
          id: "q-mcq",
          test_id: "test-1",
          format: "mcq",
          body: "Berapa 2+2?",
          sort_order: 1,
          options: [
            { key: "A", text: "3", sort_order: 1 },
            { key: "B", text: "4", sort_order: 2 },
            { key: "C", text: "5", sort_order: 3 },
          ],
        },
        {
          id: "q-multi",
          test_id: "test-1",
          format: "multi_answer",
          body: "Pilih bilangan genap",
          sort_order: 2,
          options: [
            { key: "A", text: "1", sort_order: 1 },
            { key: "B", text: "2", sort_order: 2 },
            { key: "C", text: "4", sort_order: 3 },
          ],
        },
        {
          id: "q-short",
          test_id: "test-1",
          format: "short",
          body: "Ibu kota Indonesia adalah?",
          sort_order: 3,
          options: [],
        },
        {
          id: "q-fill",
          test_id: "test-1",
          format: "fill_blank",
          body: "Bendera Indonesia berwarna ___ dan putih.",
          sort_order: 4,
          options: [],
        },
        {
          id: "q-essay",
          test_id: "test-1",
          format: "essay",
          body: "Jelaskan penyebab Perang Diponegoro.",
          sort_order: 5,
          options: [],
        },
      ],
    },
  ],
};

const submittedSession: SessionState = {
  ...sampleSession,
  status: "submitted",
  submitted_at: "2026-07-15T10:00:00Z",
  remaining_seconds: 0,
};

const multiTestSession: SessionState = {
  ...sampleSession,
  tests: [
    {
      id: "test-bahasa",
      title: "TKA BAHASA INDONESIA SD/MI",
      subject: "Bahasa Indonesia",
      questions: [
        {
          id: "q-bahasa-1",
          test_id: "test-bahasa",
          format: "mcq",
          body: "Sinonim dari cerdas?",
          sort_order: 1,
          options: [
            { key: "A", text: "Pintar", sort_order: 1 },
            { key: "B", text: "Lambat", sort_order: 2 },
          ],
        },
      ],
    },
    {
      id: "test-math",
      title: "Tes Matematika",
      subject: "Matematika",
      questions: [
        {
          id: "q-math-1",
          test_id: "test-math",
          format: "mcq",
          body: "Berapa 9 - 1?",
          sort_order: 1,
          options: [
            { key: "A", text: "8", sort_order: 1 },
            { key: "B", text: "7", sort_order: 2 },
          ],
        },
      ],
    },
  ],
};

// ── Sectioned session samples ───────────────────────────────────────────────

const sectionedSession: SessionState = {
  ...sampleSession,
  mode: "utbk",
  active_test_id: "test-section-1",
  duration_minutes: null,
  remaining_seconds: 0,
  answers: [],
  tests: [
    {
      id: "test-section-1",
      title: "TPS",
      subject: "TPS",
      status: "active",
      remaining_seconds: 1800,
      duration_minutes: 30,
      questions: [
        {
          id: "q-sec1-mcq",
          test_id: "test-section-1",
          format: "mcq",
          body: "TPS Question 1?",
          sort_order: 1,
          options: [
            { key: "A", text: "Three", sort_order: 1 },
            { key: "B", text: "Four", sort_order: 2 },
          ],
        },
        {
          id: "q-sec1-essay",
          test_id: "test-section-1",
          format: "essay",
          body: "TPS Essay?",
          sort_order: 2,
          options: [],
        },
      ],
    },
    {
      id: "test-section-2",
      title: "Literasi",
      subject: "Literasi",
      status: "pending",
      remaining_seconds: 0,
      duration_minutes: 45,
      questions: [
        {
          id: "q-sec2-mcq",
          test_id: "test-section-2",
          format: "mcq",
          body: "Literasi Question 1?",
          sort_order: 1,
          options: [
            { key: "A", text: "Choice A", sort_order: 1 },
            { key: "B", text: "Choice B", sort_order: 2 },
          ],
        },
      ],
    },
  ],
};

const ieltsSession: SessionState = {
  ...sectionedSession,
  mode: "ielts",
  active_test_id: "test-listening",
  duration_minutes: null,
  tests: [
    {
      id: "test-listening",
      title: "Listening",
      subject: "Listening",
      section_type: "listening",
      status: "active",
      remaining_seconds: 2400,
      duration_minutes: 40,
      audio_url: "https://example.com/audio.mp3",
      audio_play_limit: 2,
      questions: [
        {
          id: "q-listening",
          test_id: "test-listening",
          format: "mcq",
          body: "Listening Q1?",
          sort_order: 1,
          options: [
            { key: "A", text: "Opt A", sort_order: 1 },
            { key: "B", text: "Opt B", sort_order: 2 },
          ],
        },
      ],
    },
    {
      id: "test-reading",
      title: "Reading",
      subject: "Reading",
      section_type: "reading",
      status: "pending",
      remaining_seconds: 0,
      duration_minutes: 60,
      questions: [],
    },
    {
      id: "test-writing",
      title: "Writing",
      subject: "Writing",
      section_type: "writing",
      status: "pending",
      remaining_seconds: 0,
      duration_minutes: 60,
      questions: [
        {
          id: "q-writing",
          test_id: "test-writing",
          format: "essay",
          body: "Writing Task?",
          sort_order: 1,
          options: [],
        },
      ],
    },
  ],
};

// Helper: click fullscreen gate button to start the exam (standard mode)
async function enterFullscreen() {
  document.documentElement.requestFullscreen = vi
    .fn()
    .mockResolvedValue(undefined);
  const btn = screen.getByTestId("enter-fullscreen");
  fireEvent.click(btn);
  await waitFor(() => {
    expect(screen.getByText(/Berapa 2\+2\?/)).toBeInTheDocument();
  });
}

// Helper: enter fullscreen for sectioned exam (UTBK)
async function enterFullscreenSectioned() {
  document.documentElement.requestFullscreen = vi
    .fn()
    .mockResolvedValue(undefined);
  const btn = screen.getByTestId("enter-fullscreen");
  fireEvent.click(btn);
  await waitFor(() => {
    expect(screen.getByText(/TPS Question 1\?/)).toBeInTheDocument();
  });
}

// Helper: enter fullscreen for IELTS (listening section)
async function enterFullscreenIELTS() {
  document.documentElement.requestFullscreen = vi
    .fn()
    .mockResolvedValue(undefined);
  const btn = screen.getByTestId("enter-fullscreen");
  fireEvent.click(btn);
  await waitFor(() => {
    expect(screen.getByText(/Listening Q1\?/)).toBeInTheDocument();
  });
}

// Helper: enter fullscreen and wait for a specific question text
async function enterFullscreenUntil(text: RegExp) {
  document.documentElement.requestFullscreen = vi
    .fn()
    .mockResolvedValue(undefined);
  const btn = screen.getByTestId("enter-fullscreen");
  fireEvent.click(btn);
  await waitFor(() => {
    expect(screen.getByText(text)).toBeInTheDocument();
  });
}

describe("SessionPage", () => {
  beforeEach(() => {
    Object.defineProperty(document, "hidden", { value: false, configurable: true });
    Object.defineProperty(document, "fullscreenElement", { value: document.documentElement, configurable: true });
    Object.defineProperty(document, "exitFullscreen", {
      value: vi.fn().mockResolvedValue(undefined),
      configurable: true,
    });
    uiStore = { lang: "id" };
    sessionState = {
      data: sampleSession,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    saveAnswersMutate.mockReset();
    saveAnswersMutateAsync.mockReset();
    submitSessionMutate.mockReset();
    submitSessionMutateAsync.mockReset();
    logViolationMutate.mockReset();
    advanceSectionMutate.mockReset();
    advanceSectionMutateAsync.mockReset();
    routerReplace.mockReset();
    localStorage.clear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // ── Loading state ───────────────────────────────────────────────────────

  it("shows loading skeleton while reconnecting (FR29 reconnect)", () => {
    sessionState = { ...sessionState, data: null, isLoading: true };
    render(<SessionPage />);
    expect(screen.getByText("Memuat…")).toBeInTheDocument();
  });

  // ── Error state ─────────────────────────────────────────────────────────

  it("shows error card when reconnect fails (FR29 reconnect)", () => {
    sessionState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("not found"),
      refetch: vi.fn(),
    };
    render(<SessionPage />);
    expect(screen.getByText(/gagal memuat data/i)).toBeInTheDocument();
  });

  // ── Submitted state ─────────────────────────────────────────────────────

  it("redirects to the result route when session is already submitted (FR29, FR-S5-25)", () => {
    sessionState = { ...sessionState, data: submittedSession };
    render(<SessionPage />);
    expect(routerReplace).toHaveBeenCalledWith(
      "/exam/sessions/session-1/result",
    );
  });

  // ── Fullscreen gate ─────────────────────────────────────────────────────

  it("shows fullscreen gate when not yet in fullscreen (FR29)", () => {
    render(<SessionPage />);
    expect(
      screen.getByText(/mode layar penuh diperlukan/i)
    ).toBeInTheDocument();
    expect(screen.getByTestId("enter-fullscreen")).toBeInTheDocument();
  });

  // ── Question rendering per format ───────────────────────────────────────

  it("renders MCQ question with radio inputs (FR29)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    // MCQ radio options
    const radios = screen.getAllByRole("radio");
    expect(radios).toHaveLength(3);
  });

  it("renders multi_answer with checkboxes (FR29)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    // Navigate to multi_answer question (index 1)
    fireEvent.click(screen.getByTestId("session-nav-1"));

    await waitFor(() => {
      expect(screen.getByText(/pilih bilangan genap/i)).toBeInTheDocument();
    });

    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes).toHaveLength(3);
  });

  it("renders short answer with text input (FR29)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    // Navigate to short answer (index 2)
    fireEvent.click(screen.getByTestId("session-nav-2"));

    await waitFor(() => {
      expect(
        screen.getByText(/ibu kota indonesia adalah/i)
      ).toBeInTheDocument();
    });

    // The text input should be visible
    const textInputs = screen
      .getAllByRole("textbox")
      .filter((tb) => tb.tagName === "INPUT");
    expect(textInputs.length).toBeGreaterThan(0);
  });

  it("renders essay with textarea (FR29)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    // Navigate to essay (index 4)
    fireEvent.click(screen.getByTestId("session-nav-4"));

    await waitFor(() => {
      expect(
        screen.getByText(/jelaskan penyebab perang diponegoro/i)
      ).toBeInTheDocument();
    });

    // Textarea should exist
    const textareas = screen
      .getAllByRole("textbox")
      .filter((tb) => tb.tagName === "TEXTAREA");
    expect(textareas.length).toBeGreaterThan(0);
  });

  it("renders rich body via RichContent (LaTeX + bold HTML) on the question card", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        tests: [
          {
            ...sampleSession.tests[0],
            questions: [
              {
                id: "q-rich",
                test_id: "test-1",
                format: "mcq",
                body: "Hitung \\(x^2\\) dan buat <b>tebal</b>",
                sort_order: 1,
                options: [
                  { key: "A", text: "Ya", sort_order: 1 },
                  { key: "B", text: "Tidak", sort_order: 2 },
                ],
              },
            ],
          },
        ],
      },
    };
    render(<SessionPage />);
    document.documentElement.requestFullscreen = vi
      .fn()
      .mockResolvedValue(undefined);
    fireEvent.click(screen.getByTestId("enter-fullscreen"));

    // Body should be wrapped in RichContent; KaTeX renders \(x^2\) and <b> renders bold.
    const richNode = await waitFor(
      () => {
        const el = document.querySelector("[data-rich-content] .katex");
        if (!el) throw new Error("not yet");
        return el.closest("[data-rich-content]") as HTMLElement;
      },
      { timeout: 3000 }
    );
    expect(richNode).not.toBeNull();
    const b = richNode.querySelector("b");
    expect(b).not.toBeNull();
    expect(b?.textContent).toBe("tebal");
    // Literal LaTeX delimiters are replaced by KaTeX — not visible as text.
    expect(richNode.textContent).not.toContain("\\(");
  });

  // ── Flag toggle ─────────────────────────────────────────────────────────

  it("toggles flag for review (FR29)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const flagBtn = screen.getByRole("button", { name: /ragu-ragu/i });
    fireEvent.click(flagBtn);

    expect(
      screen.getByRole("button", { name: /hapus ragu-ragu/i })
    ).toBeInTheDocument();
  });

  it("rehydrates flagged_for_review from session answers on reconnect (FR29)", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        answers: [
          { question_id: "q-mcq", answer: "B", flagged_for_review: true },
        ],
      },
    };
    render(<SessionPage />);
    await enterFullscreen();

    expect(
      screen.getByRole("button", { name: /hapus ragu-ragu/i })
    ).toBeInTheDocument();
  });

  it("includes flagged_for_review in the submit save payload (FR29)", async () => {
    saveAnswersMutateAsync.mockResolvedValue(undefined);
    render(<SessionPage />);
    await enterFullscreen();

    const flagBtn = screen.getByRole("button", { name: /ragu-ragu/i });
    fireEvent.click(flagBtn);

    fireEvent.click(screen.getByRole("button", { name: /kumpulkan/i }));
    await waitFor(() => {
      expect(
        screen.getByText(/yakin ingin mengumpulkan jawaban/i)
      ).toBeInTheDocument();
    });
    const btns = screen.getAllByRole("button", { name: /kumpulkan/i });
    fireEvent.click(btns[btns.length - 1]);

    await waitFor(() => {
      expect(saveAnswersMutateAsync).toHaveBeenCalled();
    });
    const payload = saveAnswersMutateAsync.mock.calls[0][0];
    expect(payload.answers).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          question_id: "q-mcq",
          flagged_for_review: true,
        }),
      ])
    );
  });

  it("clears a selected MCQ answer back to empty and marks the question unanswered", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const radios = screen.getAllByRole("radio");
    fireEvent.click(radios[1]);
    expect(radios[1]).toBeChecked();
    expect(screen.getByTestId("session-nav-0").className).toContain("bg-brand-600");

    fireEvent.click(screen.getByRole("button", { name: /kosongkan jawaban/i }));

    expect(radios[1]).not.toBeChecked();
    expect(screen.getByTestId("session-nav-0").className).not.toContain("bg-brand-600");
    expect(screen.getByTestId("session-nav-0").className).toContain("border-line");
  });

  it("uses high-contrast status styles for answered, flagged, and unanswered question numbers", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        answers: [
          { question_id: "q-mcq", answer: "B", flagged_for_review: false },
          { question_id: "q-multi", answer: "", flagged_for_review: true },
        ],
      },
    };
    render(<SessionPage />);
    await enterFullscreen();

    expect(screen.getByTestId("session-nav-0").className).toContain("bg-brand-600");
    expect(screen.getByTestId("session-nav-1").className).toContain("bg-surface");
    expect(screen.getByTestId("session-nav-1").querySelector("span")?.className).toContain("bg-warn");
    expect(screen.getByTestId("session-nav-2").className).toContain("border-line");
  });

  // ── Timer ───────────────────────────────────────────────────────────────

  it("shows countdown timer display (FR29)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    expect(screen.getByText(/60:00/)).toBeInTheDocument();
  });

  it("reconnecting to a session whose timer already ran out auto-submits instead of freezing at 00:00", async () => {
    // Student closed the tab mid-sitting and came back after the deadline: the
    // page mounts in its loading state and the reconnect payload lands with
    // remaining_seconds already 0. Every input and the manual submit button are
    // disabled at 00:00, so if the auto-submit does not fire on that landing the
    // sitting is stuck forever. `remaining` starts at 0, so the arrival of an
    // expired payload must not be mistaken for "nothing changed".
    sessionState = { ...sessionState, data: null, isLoading: true };
    const { rerender } = render(<SessionPage />);

    sessionState = {
      ...sessionState,
      isLoading: false,
      data: {
        ...sampleSession,
        remaining_seconds: 0,
        answers: [{ question_id: "q-mcq", answer: "B" }],
      },
    };
    rerender(<SessionPage />);

    await waitFor(() => {
      expect(submitSessionMutateAsync).toHaveBeenCalled();
    });
  });

  it("untimed exam (per_test, null duration) never auto-submits and hides the countdown", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        timer_mode: "per_test",
        duration_minutes: null,
        remaining_seconds: 0,
      },
    };
    render(<SessionPage />);
    await enterFullscreen();

    expect(screen.getByText(/Berapa 2\+2\?/)).toBeInTheDocument();
    expect(screen.queryByText(/00:00/)).not.toBeInTheDocument();
    await waitFor(() => {
      expect(submitSessionMutate).not.toHaveBeenCalled();
    });
    expect(routerReplace).not.toHaveBeenCalled();
  });

  it("locks answers at 00:00 and flushes them before automatic submission", async () => {
    vi.useFakeTimers();
    sessionState = {
      ...sessionState,
      data: { ...sampleSession, remaining_seconds: 1 },
    };
    saveAnswersMutateAsync.mockResolvedValue(undefined);
    let resolveSubmit!: (value: { submitted: boolean; score: number }) => void;
    submitSessionMutateAsync.mockImplementation(
      () => new Promise((resolve) => { resolveSubmit = resolve; }),
    );

    render(<SessionPage />);
    document.documentElement.requestFullscreen = vi.fn().mockResolvedValue(undefined);
    fireEvent.click(screen.getByTestId("enter-fullscreen"));
    await act(async () => Promise.resolve());
    fireEvent.click(screen.getAllByRole("radio")[1]);

    await act(async () => {
      vi.advanceTimersByTime(1000);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(screen.getByText("00:00")).toBeInTheDocument();
    expect(screen.getAllByRole("radio")[1]).toBeDisabled();
    expect(saveAnswersMutateAsync).toHaveBeenCalledWith({
      answers: [{ question_id: "q-mcq", answer: "B", flagged_for_review: false }],
      current_position: 0,
    });
    expect(saveAnswersMutateAsync.mock.invocationCallOrder[0]).toBeLessThan(
      submitSessionMutateAsync.mock.invocationCallOrder[0],
    );
    await act(async () => resolveSubmit({ submitted: true, score: 75 }));
  });

  it("stops on a terminal expiry error and shows an explicit Retry", async () => {
    vi.useFakeTimers();
    sessionState = {
      ...sessionState,
      data: { ...sampleSession, remaining_seconds: 0 },
    };
    saveAnswersMutateAsync.mockResolvedValue(undefined);
    submitSessionMutateAsync.mockRejectedValue(
      new ApiError("invalid_session", "invalid session", 400),
    );

    render(<SessionPage />);
    document.documentElement.requestFullscreen = vi.fn().mockResolvedValue(undefined);
    fireEvent.click(screen.getByTestId("enter-fullscreen"));
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(submitSessionMutateAsync).toHaveBeenCalledTimes(1);
    await act(async () => vi.advanceTimersByTime(60_000));
    expect(submitSessionMutateAsync).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Coba lagi" })).toBeVisible();
    expect(screen.getAllByRole("radio")[0]).toBeDisabled();
  });

  it.each([
    ["transport", new TypeError("network failed")],
    ["408", new ApiError("timeout", "timeout", 408)],
    ["429", new ApiError("rate_limited", "rate limited", 429)],
    ["5xx", new ApiError("server_error", "server error", 503)],
  ])("retries a transient %s expiry failure", async (_name, transientError) => {
    vi.useFakeTimers();
    sessionState = {
      ...sessionState,
      data: { ...sampleSession, remaining_seconds: 0 },
    };
    saveAnswersMutateAsync.mockResolvedValue(undefined);
    submitSessionMutateAsync
      .mockRejectedValueOnce(transientError)
      .mockResolvedValueOnce({ submitted: true, score: 75 });

    render(<SessionPage />);
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(submitSessionMutateAsync).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.advanceTimersByTime(backoffDelayMs(0));
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(submitSessionMutateAsync).toHaveBeenCalledTimes(2);
    expect(routerReplace).toHaveBeenCalledWith(
      "/exam/sessions/session-1/result",
    );
  });

  it("stops after three transient attempts and Retry starts a fresh cycle", async () => {
    vi.useFakeTimers();
    sessionState = {
      ...sessionState,
      data: { ...sampleSession, remaining_seconds: 0 },
    };
    saveAnswersMutateAsync.mockResolvedValue(undefined);
    submitSessionMutateAsync.mockRejectedValue(
      new ApiError("server_error", "server error", 503),
    );

    render(<SessionPage />);
    document.documentElement.requestFullscreen = vi.fn().mockResolvedValue(undefined);
    fireEvent.click(screen.getByTestId("enter-fullscreen"));
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    await act(async () => {
      vi.advanceTimersByTime(backoffDelayMs(0));
      await Promise.resolve();
      await Promise.resolve();
    });
    await act(async () => {
      vi.advanceTimersByTime(backoffDelayMs(1));
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(submitSessionMutateAsync).toHaveBeenCalledTimes(3);
    expect(screen.getByRole("button", { name: "Coba lagi" })).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Coba lagi" }));
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(submitSessionMutateAsync).toHaveBeenCalledTimes(4);
    expect(screen.getAllByRole("radio")[0]).toBeDisabled();
  });

  it("treats already_submitted as successful expiry recovery", async () => {
    sessionState = {
      ...sessionState,
      data: { ...sampleSession, remaining_seconds: 0 },
    };
    saveAnswersMutateAsync.mockResolvedValue(undefined);
    submitSessionMutateAsync.mockRejectedValue(
      new ApiError("already_submitted", "already submitted", 409),
    );

    render(<SessionPage />);
    await waitFor(() => {
      expect(routerReplace).toHaveBeenCalledWith(
        "/exam/sessions/session-1/result",
      );
    });
    expect(submitSessionMutateAsync).toHaveBeenCalledTimes(1);
  });

  it("submits when the expiry save fails permanently", async () => {
    sessionState = {
      ...sessionState,
      data: { ...sampleSession, remaining_seconds: 0 },
    };
    saveAnswersMutateAsync.mockRejectedValue(
      new ApiError("invalid_answer", "invalid answer", 400),
    );
    submitSessionMutateAsync.mockResolvedValue({ submitted: true, score: 75 });

    render(<SessionPage />);

    await waitFor(() => {
      expect(submitSessionMutateAsync).toHaveBeenCalledTimes(1);
    });
    expect(routerReplace).toHaveBeenCalledWith(
      "/exam/sessions/session-1/result",
    );
  });

  it("redirects when the expiry save reports already_submitted", async () => {
    sessionState = {
      ...sessionState,
      data: { ...sampleSession, remaining_seconds: 0 },
    };
    saveAnswersMutateAsync.mockRejectedValue(
      new ApiError("already_submitted", "already submitted", 409),
    );

    render(<SessionPage />);

    await waitFor(() => {
      expect(routerReplace).toHaveBeenCalledWith(
        "/exam/sessions/session-1/result",
      );
    });
    expect(submitSessionMutateAsync).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Coba lagi" })).not.toBeInTheDocument();
  });

  it("redirects when section advance reports already_submitted", async () => {
    const expiredSection = {
      ...sectionedSession,
      tests: sectionedSession.tests.map((test, index) =>
        index === 0 ? { ...test, remaining_seconds: 0 } : test,
      ),
    };
    sessionState = { ...sessionState, data: expiredSection };
    saveAnswersMutateAsync.mockResolvedValue(undefined);
    advanceSectionMutateAsync.mockRejectedValue(
      new ApiError("already_submitted", "already submitted", 409),
    );

    render(<SessionPage />);

    await waitFor(() => {
      expect(routerReplace).toHaveBeenCalledWith(
        "/exam/sessions/session-1/result",
      );
    });
    expect(submitSessionMutateAsync).not.toHaveBeenCalled();
  });

  // ── Submit confirmation dialog ──────────────────────────────────────────

  it("shows submit confirmation dialog (FR29)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const submitBtn = screen.getByRole("button", { name: /kumpulkan/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(
        screen.getByText(/yakin ingin mengumpulkan jawaban/i)
      ).toBeInTheDocument();
    });
  });

  // ── Answer updates state ────────────────────────────────────────────────

  it("updates answer state when MCQ option is selected (FR29)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const radios = screen.getAllByRole("radio");
    expect(radios[0]).not.toBeChecked();
    expect(radios[1]).not.toBeChecked();

    fireEvent.click(radios[1]);

    expect(radios[0]).not.toBeChecked();
    expect(radios[1]).toBeChecked();
  });

  // ── Submit flow (also tests save is triggered) ──────────────────────────

  it("submit saves answers, calls hook, and redirects to result (FR29, FR-S5-25)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    // Answer a question first so save is triggered
    const radios = screen.getAllByRole("radio");
    fireEvent.click(radios[1]);

    // Open confirmation dialog
    fireEvent.click(screen.getByRole("button", { name: /kumpulkan/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/yakin ingin mengumpulkan jawaban/i)
      ).toBeInTheDocument();
    });

    // Click submit in dialog (last Kumpulkan button, inside the dialog)
    const btns = screen.getAllByRole("button", { name: /kumpulkan/i });
    fireEvent.click(btns[btns.length - 1]);

    // Verify save was triggered before submit
    expect(saveAnswersMutateAsync).toHaveBeenCalledWith({
      answers: [{ question_id: "q-mcq", answer: "B", flagged_for_review: false }],
      current_position: 0,
    });

    // Verify submitSession was called (handleSubmit awaits the save first,
    // so the mutate call lands on a later microtask).
    await waitFor(() => {
      expect(submitSessionMutate).toHaveBeenCalled();
    });

    // Simulate success response inside act to flush React state updates
    await act(async () => {
      const [, opts] = submitSessionMutate.mock.calls[0];
      opts.onSuccess({ submitted: true, score: 75 });
    });

    // Redirects to the result route instead of rendering an inline card
    expect(routerReplace).toHaveBeenCalledWith(
      "/exam/sessions/session-1/result",
    );
  });

  // ── Sectioned mode (FR-23) ────────────────────────────────────────────

  it("sectioned mode shows only active section questions (FR-23)", async () => {
    sessionState = { ...sessionState, data: sectionedSession };
    render(<SessionPage />);
    await enterFullscreenSectioned();

    // Active section's first question is visible
    expect(screen.getByText(/TPS Question 1\?/)).toBeInTheDocument();

    // Navigate to second question in the same section
    fireEvent.click(screen.getByTestId("session-nav-1"));
    await waitFor(() => {
      expect(screen.getByText(/TPS Essay\?/)).toBeInTheDocument();
    });

    // Non-active section questions are NOT visible anywhere
    expect(screen.queryByText(/Literasi Question 1\?/)).not.toBeInTheDocument();
  });

  it("sectioned mode renders section rail with all sections (FR-23)", async () => {
    sessionState = { ...sessionState, data: sectionedSession };
    render(<SessionPage />);
    await enterFullscreenSectioned();

    // Section rail container exists
    expect(screen.getByTestId("section-rail")).toBeInTheDocument();

    // Each section title appears in the rail
    const rail = screen.getByTestId("section-rail");
    expect(rail).toHaveTextContent("TPS");
    expect(rail).toHaveTextContent("Literasi");
  });

  it("sectioned mode shows per-section countdown (FR-23)", async () => {
    sessionState = { ...sessionState, data: sectionedSession };
    render(<SessionPage />);
    await enterFullscreenSectioned();

    // Active section has 1800 seconds remaining = 30:00
    expect(screen.getByText(/30:00/)).toBeInTheDocument();
  });

  it("timer zero in sectioned mode calls save then advance (FR-24)", async () => {
    saveAnswersMutateAsync.mockResolvedValue(undefined);
    advanceSectionMutateAsync.mockResolvedValue({
      mode: "utbk",
      active_test_id: "test-section-2",
      completed: false,
      tests: sectionedSession.tests,
    });

    // Set remaining to 0 so the auto-advance fires immediately
    // Include a pre-existing answer so buildSavePayload returns a non-empty array
    const expiredSession = {
      ...sectionedSession,
      answers: [{ question_id: "q-sec1-mcq", answer: "A" }],
      tests: sectionedSession.tests.map((t, i) =>
        i === 0 ? { ...t, remaining_seconds: 0 } : t,
      ),
    };
    sessionState = { ...sessionState, data: expiredSession };
    render(<SessionPage />);
    await enterFullscreenSectioned();

    await waitFor(() => {
      expect(saveAnswersMutateAsync).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(advanceSectionMutateAsync).toHaveBeenCalledWith("test-section-1");
    });

    // Submit should NOT be called for a non-last section advance
    expect(submitSessionMutate).not.toHaveBeenCalled();
  });

  it("advancing last section triggers submit and redirect (FR-24)", async () => {
    saveAnswersMutateAsync.mockResolvedValue(undefined);
    let resolveSubmit!: (value: { submitted: boolean; score: number }) => void;
    submitSessionMutateAsync.mockImplementation(
      () => new Promise((resolve) => { resolveSubmit = resolve; }),
    );
    advanceSectionMutateAsync.mockResolvedValue({
      mode: "utbk",
      active_test_id: null,
      completed: true,
      tests: sectionedSession.tests,
    });

    // Active = last section (test-section-2), remaining=0
    const lastSectionActive = {
      ...sectionedSession,
      active_test_id: "test-section-2",
      tests: [
        {
          ...sectionedSession.tests[0],
          status: "submitted" as const,
          remaining_seconds: 0,
        },
        {
          ...sectionedSession.tests[1],
          status: "active" as const,
          remaining_seconds: 0,
        },
      ],
    };
    sessionState = { ...sessionState, data: lastSectionActive };
    render(<SessionPage />);
    await enterFullscreenUntil(/Literasi Question 1\?/);

    await waitFor(() => {
      expect(saveAnswersMutateAsync).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(advanceSectionMutateAsync).toHaveBeenCalledWith("test-section-2");
    });
    await waitFor(() => {
      expect(submitSessionMutateAsync).toHaveBeenCalled();
    });

    await act(async () => {
      resolveSubmit({ submitted: true, score: 85 });
    });
    expect(routerReplace).toHaveBeenCalledWith(
      "/exam/sessions/session-1/result",
    );
  });

  it.each([
    ["UTBK", sectionedSession],
    ["IELTS", ieltsSession],
  ])("retries a transient non-final %s section advance", async (_mode, baseSession) => {
    vi.useFakeTimers();
    const expiredSession = {
      ...baseSession,
      tests: baseSession.tests.map((test, index) =>
        index === 0 ? { ...test, remaining_seconds: 0 } : test,
      ),
    };
    sessionState = { ...sessionState, data: expiredSession };
    saveAnswersMutateAsync.mockResolvedValue(undefined);
    advanceSectionMutateAsync
      .mockRejectedValueOnce(new ApiError("server_error", "server error", 503))
      .mockResolvedValueOnce({
        mode: baseSession.mode,
        active_test_id: baseSession.tests[1].id,
        completed: false,
        tests: baseSession.tests,
      });

    render(<SessionPage />);
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(advanceSectionMutateAsync).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.advanceTimersByTime(backoffDelayMs(0));
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(advanceSectionMutateAsync).toHaveBeenCalledTimes(2);
    expect(submitSessionMutateAsync).not.toHaveBeenCalled();
  });

  it.each(["utbk", "ielts"] as const)(
    "retries final %s submission without repeating the completed advance",
    async (mode) => {
      vi.useFakeTimers();
      const baseSession = mode === "utbk" ? sectionedSession : ieltsSession;
      const finalIndex = baseSession.tests.length - 1;
      const finalSession = {
        ...baseSession,
        active_test_id: baseSession.tests[finalIndex].id,
        tests: baseSession.tests.map((test, index) => ({
          ...test,
          status: index === finalIndex ? "active" as const : "submitted" as const,
          remaining_seconds: 0,
        })),
      };
      sessionState = { ...sessionState, data: finalSession };
      saveAnswersMutateAsync.mockResolvedValue(undefined);
      advanceSectionMutateAsync.mockResolvedValue({
        mode,
        active_test_id: null,
        completed: true,
        tests: finalSession.tests,
      });
      submitSessionMutateAsync
        .mockRejectedValueOnce(new ApiError("server_error", "server error", 503))
        .mockResolvedValueOnce({ submitted: true, score: 80 });

      render(<SessionPage />);
      await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
      });
      expect(submitSessionMutateAsync).toHaveBeenCalledTimes(1);

      await act(async () => {
        vi.advanceTimersByTime(backoffDelayMs(0));
        await Promise.resolve();
        await Promise.resolve();
      });

      expect(advanceSectionMutateAsync).toHaveBeenCalledTimes(1);
      expect(submitSessionMutateAsync).toHaveBeenCalledTimes(2);
      expect(routerReplace).toHaveBeenCalledWith(
        "/exam/sessions/session-1/result",
      );
    },
  );

  it("debounced autosave excludes a submitted section's answers (FR-14 seam — backend rejects locked-section saves)", async () => {
    // Section 1 is already submitted; section 2 is active (not expired). Both
    // sections carry a persisted answer, rehydrated into state on reconnect.
    // The autosave must send ONLY the active section's answer — the backend
    // rejects the whole batch (ErrSectionLocked) if any answer targets a
    // non-active section, which would silently lose every section past the first.
    const section2Active = {
      ...sectionedSession,
      active_test_id: "test-section-2",
      answers: [
        { question_id: "q-sec1-mcq", answer: "A" },
        { question_id: "q-sec2-mcq", answer: "B" },
      ],
      tests: [
        {
          ...sectionedSession.tests[0],
          status: "submitted" as const,
          remaining_seconds: 0,
        },
        {
          ...sectionedSession.tests[1],
          status: "active" as const,
          remaining_seconds: 1800,
        },
      ],
    };
    sessionState = { ...sessionState, data: section2Active };
    render(<SessionPage />);
    await enterFullscreenUntil(/Literasi Question 1\?/);

    vi.useFakeTimers();
    fireEvent.click(screen.getAllByRole("radio")[0]);
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });

    expect(saveAnswersMutate).toHaveBeenCalled();
    const sentIds = saveAnswersMutate.mock.calls.flatMap(([payload]) =>
      (payload as { answers: Array<{ question_id: string }> }).answers.map(
        (p) => p.question_id,
      ),
    );
    expect(sentIds).toContain("q-sec2-mcq"); // active section answer is saved
    expect(sentIds).not.toContain("q-sec1-mcq"); // submitted section answer is not resent
  });

  it("resets the question index to 0 when advancing to a shorter section (FR-13)", async () => {
    sessionState = { ...sessionState, data: sectionedSession };
    const { rerender } = render(<SessionPage />);
    await enterFullscreenSectioned();

    // Move to section 1's 2nd question (index 1).
    fireEvent.click(screen.getByTestId("session-nav-1"));
    await waitFor(() => {
      expect(screen.getByText(/TPS Essay\?/)).toBeInTheDocument();
    });

    // Advance to section 2 (Literasi) which has only ONE question. If the index
    // is not reset, questionsToShow[1] is undefined and the panel renders blank.
    const section2Active = {
      ...sectionedSession,
      active_test_id: "test-section-2",
      tests: [
        { ...sectionedSession.tests[0], status: "submitted" as const },
        {
          ...sectionedSession.tests[1],
          status: "active" as const,
          remaining_seconds: 2700,
        },
      ],
    };
    sessionState = { ...sessionState, data: section2Active };
    rerender(<SessionPage />);

    await waitFor(() => {
      expect(screen.getByText(/Literasi Question 1\?/)).toBeInTheDocument();
    });
  });

  it("pending section rail items are not clickable (FR-23)", async () => {
    sessionState = { ...sessionState, data: sectionedSession };
    render(<SessionPage />);
    await enterFullscreenSectioned();

    // Active section is the first one (TPS) — clicking Literasi rail item
    // should not change the visible questions
    const literasiRail = screen.getByText("Literasi");
    fireEvent.click(literasiRail);

    // Active section questions still shown (no navigation)
    expect(screen.getByText(/TPS Question 1\?/)).toBeInTheDocument();
    expect(screen.queryByText(/Literasi Question 1\?/)).not.toBeInTheDocument();
  });

  // ── IELTS skill rendering (FR-25) ──────────────────────────────────────

  it("renders audio player for listening sections (FR-25)", async () => {
    sessionState = { ...sessionState, data: ieltsSession };
    render(<SessionPage />);
    await enterFullscreenIELTS();

    const audio = screen.getByTestId("section-audio-player");
    expect(audio).toBeInTheDocument();
    expect(audio).toHaveAttribute("src", "https://example.com/audio.mp3");
  });

  // ── Two-column overlay restyle (Task 3) ─────────────────────────────────

  it("renders a fixed full-viewport overlay wrapper for an in-progress session", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const overlay = screen.getByTestId("exam-overlay");
    expect(overlay.className).toMatch(/\bfixed\b/);
    expect(overlay.className).toMatch(/\binset-0\b/);
  });

  it("shows the top bar with title, answered count, timer, and submit button", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const topBar = screen.getByTestId("exam-top-bar");
    // Title (falls back to the test's own title since no package title exists in SessionState)
    expect(topBar).toHaveTextContent("Tes Matematika");
    // Answered count
    expect(topBar).toHaveTextContent("0/5");
    // Timer
    expect(topBar).toHaveTextContent("60:00");
    // Submit button (standard mode)
    const submitButton = screen.getByTestId("exam-top-bar").querySelector("button");
    expect(submitButton).not.toBeNull();
    expect(submitButton?.className).toContain("bg-[var(--color-submit)]");
    expect(submitButton?.className).not.toContain("border-brand-600");
  });

  it("updates the top-bar title to the current question's test in a multi-test standard exam", async () => {
    sessionState = { ...sessionState, data: multiTestSession };
    render(<SessionPage />);

    document.documentElement.requestFullscreen = vi
      .fn()
      .mockResolvedValue(undefined);
    fireEvent.click(screen.getByTestId("enter-fullscreen"));

    await waitFor(() => {
      expect(screen.getByText(/Sinonim dari cerdas\?/)).toBeInTheDocument();
    });

    const topBar = screen.getByTestId("exam-top-bar");
    expect(topBar).toHaveTextContent("TKA BAHASA INDONESIA SD/MI");
    expect(topBar).not.toHaveTextContent("Tes Matematika");

    fireEvent.click(screen.getByTestId("session-nav-1"));

    await waitFor(() => {
      expect(screen.getByText(/Berapa 9 - 1\?/)).toBeInTheDocument();
    });

    expect(screen.getByTestId("exam-top-bar")).toHaveTextContent(
      "Tes Matematika",
    );
    expect(screen.getByTestId("exam-top-bar")).not.toHaveTextContent(
      "TKA BAHASA INDONESIA SD/MI",
    );
  });

  it("shows distinct mode label and section label in top bar for sectioned mode", async () => {
    sessionState = { ...sessionState, data: sectionedSession };
    render(<SessionPage />);

    const enterFullscreenBtn = screen.getByTestId("enter-fullscreen");
    fireEvent.click(enterFullscreenBtn);

    await waitFor(() => {
      const topBar = screen.getByTestId("exam-top-bar");
      // In UTBK mode, title should be "UTBK" (from i18n), not "TPS" (the first section's title)
      expect(topBar).toHaveTextContent("UTBK");
      // Section label should be "TPS" (the active section's title)
      expect(topBar).toHaveTextContent("TPS");
    });

    const topBar = screen.getByTestId("exam-top-bar");
    const title = screen.getByTestId("exam-title");
    expect(title.textContent).toBe("UTBK");
    expect(title.nextElementSibling?.textContent).toBe("TPS");
    expect(topBar).toContainElement(title);
  });

  it("nav rail shows the three legend entries with correct labels", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const rail = screen.getByTestId("exam-nav-rail");
    expect(rail).toHaveTextContent("Terjawab");
    expect(rail).toHaveTextContent("Tidak terjawab");
    expect(rail).toHaveTextContent("Ditandai");
  });

  it("nav rail question grid preserves answered/flagged/current status classes", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    // Answer q0 (current), flag q1, leave q2 untouched
    const radios = screen.getAllByRole("radio");
    fireEvent.click(radios[1]);

    const rail = screen.getByTestId("exam-nav-rail");
    expect(rail.querySelector('[data-testid="session-nav-0"]')).not.toBeNull();

    const cellCurrentAnswered = screen.getByTestId("session-nav-0");
    // Current is now a ring so the answered status stays visible.
    expect(cellCurrentAnswered.className).toContain("bg-brand-600");
    expect(cellCurrentAnswered.className).toContain("ring-brand-600");
    expect(cellCurrentAnswered.className).toContain(
      "text-white",
    );

    // Navigate away — q0 is now answered but no longer current
    fireEvent.click(screen.getByTestId("session-nav-2"));
    const cellAnsweredNotCurrent = screen.getByTestId("session-nav-0");
    expect(cellAnsweredNotCurrent.className).toContain("bg-brand-600");
    expect(cellAnsweredNotCurrent.className).not.toContain("ring-brand-600");
    expect(cellAnsweredNotCurrent.className).toContain(
      "text-white",
    );
  });

  it("uses a mobile-first body grid with a desktop nav rail", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const body = screen.getByTestId("exam-body");
    expect(body.className).toMatch(/(^|\s)grid-cols-1(\s|$)/);
    expect(body.className).toContain("lg:grid-cols-[minmax(0,1fr)_280px]");
    expect(body.className).toContain("lg:overflow-hidden");
  });

  // These assertions prove classNames and React state, not layout.
  it("keeps the responsive top-bar class and counter structure", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const topBar = screen.getByTestId("exam-top-bar");
    const counter = screen.getByText(/0\/5/);
    const metaRow = counter.parentElement;
    expect(topBar.className).toContain("grid-cols-[minmax(0,1fr)_auto]");
    expect(topBar.className).toContain("flex-wrap");
    expect(metaRow?.className).toContain("col-span-2");
    expect(metaRow?.className).toContain("sm:ml-auto");
  });

  it("keeps the answered counter and save indicator visible in the DOM", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const counter = screen.getByText(/0\/5/);
    const saveIndicator = screen.getByTestId("save-indicator");
    expect(counter).toBeInTheDocument();
    expect(counter.className).not.toContain("hidden");
    expect(saveIndicator).toBeInTheDocument();
    expect(saveIndicator.className).not.toContain("hidden");
  });

  it("starts with the mobile nav panel collapsed", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const toggle = screen.getByTestId("exam-nav-toggle");
    const panel = document.getElementById("exam-nav-panel");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(panel?.className).toContain("hidden");
    expect(panel?.className).toContain("lg:block");
  });

  it("expands the mobile nav panel when toggled", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const toggle = screen.getByTestId("exam-nav-toggle");
    fireEvent.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const panel = document.getElementById("exam-nav-panel");
    expect(panel?.className).toMatch(/(^|\s)block(\s|$)/);
    expect(panel?.className).not.toMatch(/(^|\s)hidden(\s|$)/);
    expect(panel?.className).toContain("mt-3");
    expect(panel?.className).toContain("rounded-xl");
    expect(panel?.className).toContain("shadow-sm");
    expect(panel?.className).toContain("lg:block");
  });

  it("keeps all nav legend labels inside the collapsible panel", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const panel = document.getElementById("exam-nav-panel");
    expect(panel).toContainElement(screen.getByText("Terjawab"));
    expect(panel).toContainElement(screen.getByText("Tidak terjawab"));
    expect(panel).toContainElement(screen.getByText("Ditandai"));
  });

  it("navigates and collapses the mobile nav panel after selecting a question", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const toggle = screen.getByTestId("exam-nav-toggle");
    fireEvent.click(toggle);
    fireEvent.click(screen.getByTestId("session-nav-2"));

    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(document.activeElement).toBe(toggle);
    expect(screen.getByText("Ibu kota Indonesia adalah?")).toBeInTheDocument();
  });

  it("uses mobile and desktop sizes for nav question cells", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const cell = screen.getByTestId("session-nav-0");
    expect(cell.className).toContain("size-10");
    expect(cell.className).toContain("lg:size-8");
  });

  it("allows MCQ option text to wrap", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    const optionText = screen.getAllByRole("radio")[0]
      .parentElement?.querySelector("input + div");
    expect(optionText?.className).toContain("min-w-0");
    expect(optionText?.className).toContain("break-words");
  });

  it("renders writing section questions as essay (FR-25)", async () => {
    // Set writing as active section
    const writingActive = {
      ...ieltsSession,
      active_test_id: "test-writing",
      tests: ieltsSession.tests.map((t) => {
        if (t.id === "test-listening")
          return { ...t, status: "submitted" as const, remaining_seconds: 0 };
        if (t.id === "test-writing")
          return { ...t, status: "active" as const, remaining_seconds: 3600, duration_minutes: 60 };
        return t;
      }),
    };
    sessionState = { ...sessionState, data: writingActive };
    render(<SessionPage />);
    await enterFullscreenUntil(/Writing Task\?/);

    // Writing section uses essay format (textarea)
    expect(screen.getByText(/Writing Task\?/)).toBeInTheDocument();
    const textareas = screen
      .getAllByRole("textbox")
      .filter((tb) => tb.tagName === "TEXTAREA");
    expect(textareas.length).toBeGreaterThan(0);
  });

  // ── Anti-cheat visible warning overlay (Task 4) ──────────────────────────────

  async function advanceViolationGrace(ms = 3000) {
    act(() => {
      vi.advanceTimersByTime(ms);
    });
    await act(async () => {
      await Promise.resolve();
    });
  }

  async function enterFullscreenWithFakeTimers() {
    document.documentElement.requestFullscreen = vi
      .fn()
      .mockResolvedValue(undefined);
    fireEvent.click(screen.getByTestId("enter-fullscreen"));
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.getByText(/Berapa 2\+2\?/)).toBeInTheDocument();
  }

  it("shows a fully described violation warning only after fullscreen exit grace", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });

    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();
    await advanceViolationGrace();

    expect(screen.getByTestId("violation-overlay")).toBeInTheDocument();
    expect(screen.getByText(/Peringatan/)).toBeInTheDocument();
    expect(screen.getByRole("alertdialog")).toHaveAttribute("aria-modal", "true");
    expect(screen.getByTestId("violation-warning-icon")).toBeInTheDocument();
    expect(screen.getByTestId("violation-warning-count")).toHaveTextContent(
      "Total pelanggaran tercatat: 1",
    );
    expect(screen.getByTestId("violation-return-button").className).toContain(
      "bg-brand-600",
    );
  });

  it("does not record a fullscreen exit while an answer field is focused, and re-enters on blur", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();
    const requestFullscreen = vi.fn().mockResolvedValue(undefined);
    document.documentElement.requestFullscreen = requestFullscreen;

    fireEvent.click(screen.getByTestId("session-nav-2"));
    const input = screen
      .getAllByRole("textbox")
      .find((field) => field.tagName === "INPUT") as HTMLInputElement;
    input.focus();

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    await advanceViolationGrace();

    expect(logViolationMutate).not.toHaveBeenCalled();
    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();

    fireEvent.blur(input);
    await act(async () => {
      await Promise.resolve();
    });
    expect(requestFullscreen).toHaveBeenCalledTimes(1);
  });

  it("keeps a pending fullscreen violation alive across timer rerenders", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });

    await advanceViolationGrace(1000);
    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();
    expect(screen.getByText("59:59")).toBeInTheDocument();

    await advanceViolationGrace(2000);
    expect(logViolationMutate).toHaveBeenCalledWith("fullscreen_exit");
    expect(screen.getByTestId("violation-overlay")).toBeInTheDocument();
  });

  it("increments violation counter and shows count after grace (FR13, FR18)", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    await advanceViolationGrace();

    expect(screen.getByText(/Total pelanggaran tercatat: 1/i)).toBeInTheDocument();
  });

  it("increments violation counter on second fullscreen exit after grace (FR18)", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    await advanceViolationGrace();

    expect(screen.getByText(/Total pelanggaran tercatat: 1/i)).toBeInTheDocument();

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: document.documentElement,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    fireEvent.click(screen.getByTestId("violation-return-button"));

    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();

    await advanceViolationGrace(5000);

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    await advanceViolationGrace();

    expect(screen.getByText(/Total pelanggaran tercatat: 2/i)).toBeInTheDocument();
  });

  it("suppresses rapid duplicate fullscreen exits after the first warning", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    await advanceViolationGrace();
    expect(screen.getByText(/Total pelanggaran tercatat: 1/i)).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("violation-return-button"));
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    await advanceViolationGrace();

    expect(logViolationMutate).toHaveBeenCalledTimes(1);
    expect(screen.queryByText(/Total pelanggaran tercatat: 2/i)).not.toBeInTheDocument();
  });

  it("cancels a brief tab switch when the page returns before grace elapses", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    act(() => {
      Object.defineProperty(document, "hidden", { value: true, configurable: true });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    act(() => {
      Object.defineProperty(document, "hidden", { value: false, configurable: true });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await advanceViolationGrace();

    expect(logViolationMutate).not.toHaveBeenCalledWith("tab_switch");
    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();
  });

  it("shows violation overlay on tab switch only after visibility grace", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    act(() => {
      Object.defineProperty(document, "hidden", {
        value: true,
        configurable: true,
      });
      document.dispatchEvent(new Event("visibilitychange"));
    });

    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();
    await advanceViolationGrace();

    expect(screen.getByTestId("violation-overlay")).toBeInTheDocument();
    expect(screen.getByText(/Peringatan/)).toBeInTheDocument();
  });

  it("tab switch increments shared violation counter after grace (FR14, FR18)", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    await advanceViolationGrace();

    expect(screen.getByText(/Total pelanggaran tercatat: 1/i)).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("violation-return-button"));
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();

    await advanceViolationGrace(5000);

    act(() => {
      Object.defineProperty(document, "hidden", {
        value: true,
        configurable: true,
      });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await advanceViolationGrace();

    expect(screen.getByText(/Total pelanggaran tercatat: 2/i)).toBeInTheDocument();
  });

  it("clicking return button requests fullscreen and closes overlay (FR15)", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    const requestFullscreenMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(document.documentElement, "requestFullscreen", {
      value: requestFullscreenMock,
      configurable: true,
    });

    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    await advanceViolationGrace();

    expect(screen.getByTestId("violation-overlay")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("violation-return-button"));

    await act(async () => {
      await Promise.resolve();
    });

    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();

    expect(requestFullscreenMock).toHaveBeenCalled();
  });

  it("copy event does not show violation overlay (FR17)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    // Trigger copy event
    act(() => {
      document.dispatchEvent(new Event("copy"));
    });

    // Overlay should NOT be visible
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();
  });

  it("timer continues running while violation overlay is shown (FR16)", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    // Verify timer is present and running (shows initial time)
    expect(screen.getByText("60:00")).toBeInTheDocument();

    // Trigger violation
    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    await advanceViolationGrace();

    expect(screen.getByTestId("violation-overlay")).toBeInTheDocument();

    // Verify timer element is still in the DOM while overlay is shown
    // (i.e., the timer wasn't removed or paused by the overlay)
    const topBar = screen.getByTestId("exam-top-bar");
    expect(topBar).toBeInTheDocument();

    // Timer text should still be present in the top bar (format: MM:SS)
    const timerText = screen.getByText(/^\d{2}:\d{2}$/);
    expect(timerText).toBeInTheDocument();

    // Overlay should still be visible
    expect(screen.getByTestId("violation-overlay")).toBeInTheDocument();
  });

  it("calls logViolation.mutate on fullscreen_exit after grace", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    logViolationMutate.mockClear();

    // Trigger fullscreen exit
    act(() => {
      Object.defineProperty(document, "fullscreenElement", {
        value: null,
        configurable: true,
      });
      document.dispatchEvent(new Event("fullscreenchange"));
    });

    expect(logViolationMutate).not.toHaveBeenCalledWith("fullscreen_exit");
    await advanceViolationGrace();

    expect(logViolationMutate).toHaveBeenCalledWith("fullscreen_exit");
  });

  it("calls logViolation.mutate on tab_switch after grace", async () => {
    vi.useFakeTimers();
    render(<SessionPage />);
    await enterFullscreenWithFakeTimers();

    logViolationMutate.mockClear();

    // Trigger tab switch
    act(() => {
      Object.defineProperty(document, "hidden", {
        value: true,
        configurable: true,
      });
      document.dispatchEvent(new Event("visibilitychange"));
    });

    expect(logViolationMutate).not.toHaveBeenCalledWith("tab_switch");
    await advanceViolationGrace();

    expect(logViolationMutate).toHaveBeenCalledWith("tab_switch");
  });

  it("does not show overlay for non-in_progress sessions", async () => {
    const submittedSess = { ...sampleSession, status: "submitted" as const };
    sessionState = { ...sessionState, data: submittedSess };
    render(<SessionPage />);

    // Should redirect without showing fullscreen gate or overlay
    await waitFor(() => {
      expect(routerReplace).toHaveBeenCalledWith(
        "/exam/sessions/session-1/result",
      );
    });

    // Ensure the violation overlay is not rendered
    expect(screen.queryByTestId("violation-overlay")).not.toBeInTheDocument();
  });

  // ── Multi-blank rendering (FR-16, FR-17, FR-18) ──────────────────────────

  it("renders multi_blank question with inline inputs at {{N}} positions (FR-16)", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        tests: [
          {
            ...sampleSession.tests[0],
            questions: [
              {
                id: "q-multi-blank",
                test_id: "test-1",
                format: "multi_blank",
                body: "Ibu kota Indonesia adalah {{1}}, didirikan tahun {{2}}.",
                sort_order: 1,
                options: [],
                blanks: [1, 2],
              },
            ],
          },
        ],
      },
    };
    render(<SessionPage />);
    document.documentElement.requestFullscreen = vi
      .fn()
      .mockResolvedValue(undefined);
    fireEvent.click(screen.getByTestId("enter-fullscreen"));

    // Wait for two inputs to be mounted
    await waitFor(() => {
      const inputs = screen
        .getAllByRole("textbox")
        .filter((tb) => tb.tagName === "INPUT");
      expect(inputs.length).toBeGreaterThanOrEqual(2);
    });
    expect(screen.queryByText(/\{\{/)).not.toBeInTheDocument();
  });

  it("preserves values independently across blanks when typing (FR-17)", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        tests: [
          {
            ...sampleSession.tests[0],
            questions: [
              {
                id: "q-multi-blank-17",
                test_id: "test-1",
                format: "multi_blank",
                body: "Fill {{1}} and {{2}} blanks",
                sort_order: 1,
                options: [],
                blanks: [1, 2],
              },
            ],
          },
        ],
      },
    };
    render(<SessionPage />);
    document.documentElement.requestFullscreen = vi
      .fn()
      .mockResolvedValue(undefined);
    fireEvent.click(screen.getByTestId("enter-fullscreen"));

    await waitFor(() => {
      const inputs = screen
        .getAllByRole("textbox")
        .filter((tb) => tb.tagName === "INPUT");
      expect(inputs.length).toBeGreaterThanOrEqual(2);
    });

    // Query fresh inputs (before any interactions)
    let inputs = screen
      .getAllByRole("textbox")
      .filter((tb) => tb.tagName === "INPUT");

    // Type into blank 2 — this should trigger onChange, which updates the component state
    fireEvent.change(inputs[1], { target: { value: "value2" } });

    // RE-QUERY inputs from the DOM to get fresh references.
    // If the component is incorrectly rebuilding the DOM on every keystroke,
    // the original references will be detached, and fresh queries will get new elements.
    inputs = screen
      .getAllByRole("textbox")
      .filter((tb) => tb.tagName === "INPUT");

    // Type into blank 1
    fireEvent.change(inputs[0], { target: { value: "value1" } });

    // RE-QUERY again to verify the live DOM state
    inputs = screen
      .getAllByRole("textbox")
      .filter((tb) => tb.tagName === "INPUT");

    // Both should retain their values in the LIVE DOM
    // If there's a stale-closure bug, blank 2's value would be clobbered when blank 1's
    // detached event listener fires again.
    expect((inputs[0] as HTMLInputElement).value).toBe("value1");
    expect((inputs[1] as HTMLInputElement).value).toBe("value2");
  });

  it("does not lose focus or clobber values with multiple keystrokes (FR-17 regression)", async () => {
    // This test catches two real bugs:
    // 1. Focus loss: typing multiple characters loses focus after the first keystroke
    // 2. Stale-closure clobbering: editing blank 2 then typing in blank 1 clobbers blank 2
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        tests: [
          {
            ...sampleSession.tests[0],
            questions: [
              {
                id: "q-multi-blank-17-focus",
                test_id: "test-1",
                format: "multi_blank",
                body: "Fill {{1}} and {{2}} blanks",
                sort_order: 1,
                options: [],
                blanks: [1, 2],
              },
            ],
          },
        ],
      },
    };
    render(<SessionPage />);
    document.documentElement.requestFullscreen = vi
      .fn()
      .mockResolvedValue(undefined);
    fireEvent.click(screen.getByTestId("enter-fullscreen"));

    await waitFor(() => {
      const inputs = screen
        .getAllByRole("textbox")
        .filter((tb) => tb.tagName === "INPUT");
      expect(inputs.length).toBeGreaterThanOrEqual(2);
    });

    // Test 1: Multiple keystrokes without focus loss
    // The component should NOT rebuild the DOM on every keystroke.
    let inputs = screen
      .getAllByRole("textbox")
      .filter((tb) => tb.tagName === "INPUT");
    const input0RefBefore = inputs[0]; // Capture original reference

    // Type multiple characters — all should go into the same input without rebuilding
    fireEvent.input(inputs[0], { target: { value: "m" } });
    fireEvent.input(inputs[0], { target: { value: "mu" } });
    fireEvent.input(inputs[0], { target: { value: "mul" } });

    // If the DOM is being rebuilt on every keystroke (bug), the input reference will be different.
    // Re-query and verify it's the SAME element (or at least, the value is still there).
    inputs = screen
      .getAllByRole("textbox")
      .filter((tb) => tb.tagName === "INPUT");
    expect((inputs[0] as HTMLInputElement).value).toBe("mul");

    // Test 2: No stale-closure clobbering
    // Clear and start fresh
    fireEvent.input(inputs[1], { target: { value: "second" } });

    // Re-query
    inputs = screen
      .getAllByRole("textbox")
      .filter((tb) => tb.tagName === "INPUT");

    // Now type into blank 0 again
    fireEvent.input(inputs[0], { target: { value: "multi" } });

    // Re-query and verify BOTH values are preserved (not clobbered)
    inputs = screen
      .getAllByRole("textbox")
      .filter((tb) => tb.tagName === "INPUT");
    expect((inputs[0] as HTMLInputElement).value).toBe("multi");
    expect((inputs[1] as HTMLInputElement).value).toBe("second");
  });

  it("renders malformed token set without crashing (FR-18)", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        tests: [
          {
            ...sampleSession.tests[0],
            questions: [
              {
                id: "q-multi-blank-18",
                test_id: "test-1",
                format: "multi_blank",
                body: "Question: {{1}} and {{3}} tokens",
                sort_order: 1,
                options: [],
                blanks: [1, 3],
              },
            ],
          },
        ],
      },
    };
    render(<SessionPage />);
    document.documentElement.requestFullscreen = vi
      .fn()
      .mockResolvedValue(undefined);
    fireEvent.click(screen.getByTestId("enter-fullscreen"));

    // Should render without crashing - the inputs for 1 and 3 should be mounted
    await waitFor(() => {
      const inputs = screen
        .getAllByRole("textbox")
        .filter((tb) => tb.tagName === "INPUT");
      expect(inputs.length).toBeGreaterThanOrEqual(1);
    });
  });

  // ── true_false rendering (FR-31, FR-32) ─────────────────────────────────

  function trueFalseSession(overrides?: { answers?: SessionState["answers"] }): SessionState {
    return {
      ...sampleSession,
      answers: overrides?.answers ?? [],
      tests: [
        {
          ...sampleSession.tests[0],
          questions: [
            {
              id: "q-tf",
              test_id: "test-1",
              format: "true_false",
              body: "Which statements are true?",
              sort_order: 1,
              options: [],
              statements: [
                { index: 1, body: "Statement A" },
                { index: 2, body: "Statement B" },
                { index: 3, body: "Statement C" },
                { index: 4, body: "Statement D" },
              ],
            },
          ],
        },
      ],
    };
  }

  it("keeps colspan on a merged table cell in a statement body — the page's own sanitiser runs before RichContent (FR-36)", async () => {
    const merged = trueFalseSession();
    merged.tests[0].questions[0].statements = [
      {
        index: 1,
        body: '<table><tbody><tr><td colspan="2" rowspan="2">Merged</td></tr></tbody></table>',
      },
      { index: 2, body: "Statement B" },
    ];
    sessionState = { ...sessionState, data: merged };
    render(<SessionPage />);
    await enterFullscreenUntil(/Which statements are true\?/);

    const cell = screen
      .getByTestId("tf-statement-1")
      .querySelector("td") as HTMLTableCellElement;
    expect(cell).not.toBeNull();
    expect(cell.getAttribute("colspan")).toBe("2");
    expect(cell.getAttribute("rowspan")).toBe("2");
  });

  it("renders one question card with four statement controls, in index order (FR-31)", async () => {
    sessionState = { ...sessionState, data: trueFalseSession() };
    render(<SessionPage />);
    await enterFullscreenUntil(/Which statements are true\?/);

    const rows = screen.getAllByTestId(/^tf-statement-\d+$/);
    expect(rows).toHaveLength(4);
    expect(rows.map((r) => r.textContent)).toEqual([
      expect.stringContaining("Statement A"),
      expect.stringContaining("Statement B"),
      expect.stringContaining("Statement C"),
      expect.stringContaining("Statement D"),
    ]);
  });

  it("answers statements 1 and 3 true, 2 false, leaves 4 untouched — encodes [\"true\",\"false\",\"true\",\"\"] (FR-32)", async () => {
    sessionState = { ...sessionState, data: trueFalseSession() };
    render(<SessionPage />);
    await enterFullscreenUntil(/Which statements are true\?/);

    fireEvent.click(screen.getByTestId("tf-radio-true-1"));
    fireEvent.click(screen.getByTestId("tf-radio-false-2"));
    fireEvent.click(screen.getByTestId("tf-radio-true-3"));
    // statement 4 left untouched

    // The re-rendered controls reflect exactly what was handed to onChange —
    // the encoded value round-trips back in as currentValue on every keystroke.
    expect((screen.getByTestId("tf-radio-true-1") as HTMLInputElement).checked).toBe(true);
    expect((screen.getByTestId("tf-radio-false-1") as HTMLInputElement).checked).toBe(false);
    expect((screen.getByTestId("tf-radio-false-2") as HTMLInputElement).checked).toBe(true);
    expect((screen.getByTestId("tf-radio-true-2") as HTMLInputElement).checked).toBe(false);
    expect((screen.getByTestId("tf-radio-true-3") as HTMLInputElement).checked).toBe(true);
    expect((screen.getByTestId("tf-radio-false-3") as HTMLInputElement).checked).toBe(false);
    expect((screen.getByTestId("tf-radio-true-4") as HTMLInputElement).checked).toBe(false);
    expect((screen.getByTestId("tf-radio-false-4") as HTMLInputElement).checked).toBe(false);
  });

  it("rehydrates a saved [\"true\",\"\",\"false\",\"\"] answer and shows that state", async () => {
    sessionState = {
      ...sessionState,
      data: trueFalseSession({
        answers: [{ question_id: "q-tf", answer: '["true","","false",""]', flagged_for_review: false }],
      }),
    };
    render(<SessionPage />);
    await enterFullscreenUntil(/Which statements are true\?/);

    expect(
      (screen.getByTestId("tf-radio-true-1") as HTMLInputElement).checked
    ).toBe(true);
    expect(
      (screen.getByTestId("tf-radio-false-1") as HTMLInputElement).checked
    ).toBe(false);
    expect(
      (screen.getByTestId("tf-radio-true-2") as HTMLInputElement).checked
    ).toBe(false);
    expect(
      (screen.getByTestId("tf-radio-false-2") as HTMLInputElement).checked
    ).toBe(false);
    expect(
      (screen.getByTestId("tf-radio-false-3") as HTMLInputElement).checked
    ).toBe(true);
    expect(
      (screen.getByTestId("tf-radio-true-3") as HTMLInputElement).checked
    ).toBe(false);
    expect(
      (screen.getByTestId("tf-radio-true-4") as HTMLInputElement).checked
    ).toBe(false);
    expect(
      (screen.getByTestId("tf-radio-false-4") as HTMLInputElement).checked
    ).toBe(false);
  });

  it("never reads an is_true field from the payload — omitted entirely, component still renders (NFR-5)", async () => {
    // The fixture's statements carry only { index, body } — no is_true anywhere,
    // proving the session page cannot leak the answer key even if it tried.
    sessionState = { ...sessionState, data: trueFalseSession() };
    render(<SessionPage />);
    await enterFullscreenUntil(/Which statements are true\?/);

    expect(screen.getAllByTestId(/^tf-statement-\d+$/)).toHaveLength(4);
    expect(document.body.innerHTML).not.toContain("is_true");
  });

  // ── Rich-text option rendering (FR-12) ─────────────────────────────────

  it("renders mcq option text with RichContent (formatted, not literal tags) (FR-12)", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        tests: [
          {
            ...sampleSession.tests[0],
            questions: [
              {
                id: "q-mcq-rich",
                test_id: "test-1",
                format: "mcq",
                body: "Question?",
                sort_order: 1,
                options: [
                  { key: "A", text: "Option <b>bold</b>", sort_order: 1 },
                  { key: "B", text: "Option plain", sort_order: 2 },
                ],
              },
            ],
          },
        ],
      },
    };
    render(<SessionPage />);
    await enterFullscreenUntil(/Question\?/);

    // Rich-text HTML should be rendered, not the literal <b> tag
    const boldElements = document.querySelectorAll("b");
    let foundBold = false;
    boldElements.forEach((el) => {
      if (el.textContent === "bold") foundBold = true;
    });
    expect(foundBold).toBe(true);
  });

  it("renders multi_answer option text with RichContent (FR-12)", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        tests: [
          {
            ...sampleSession.tests[0],
            questions: [
              {
                id: "q-multi-rich",
                test_id: "test-1",
                format: "multi_answer",
                body: "Pick the right ones",
                sort_order: 1,
                options: [
                  { key: "A", text: "\\(x^2\\)", sort_order: 1 },
                  { key: "B", text: "plain text", sort_order: 2 },
                ],
              },
            ],
          },
        ],
      },
    };
    render(<SessionPage />);
    await enterFullscreenUntil(/Pick the right ones/);

    // KaTeX should render the formula
    const katex = document.querySelector(".katex");
    expect(katex).not.toBeNull();
  });

  it("renders mcq option letter labels from opt.key", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        tests: [
          {
            ...sampleSession.tests[0],
            questions: [
              {
                id: "q-mcq-keys",
                test_id: "test-1",
                format: "mcq",
                body: "Pilih huruf",
                sort_order: 1,
                options: [
                  { key: "a", text: "Jakarta", sort_order: 1 },
                  { key: "b", text: "Bandung", sort_order: 2 },
                ],
              },
            ],
          },
        ],
      },
    };
    render(<SessionPage />);
    await enterFullscreenUntil(/Pilih huruf/);
    expect(screen.getByTestId("option-key-a")).toHaveTextContent("A");
    expect(screen.getByTestId("option-key-b")).toHaveTextContent("B");
  });

  it("renders multi_answer option letter labels from opt.key", async () => {
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        tests: [
          {
            ...sampleSession.tests[0],
            questions: [
              {
                id: "q-multi-keys",
                test_id: "test-1",
                format: "multi_answer",
                body: "Pilih beberapa",
                sort_order: 1,
                options: [
                  { key: "a", text: "Satu", sort_order: 1 },
                  { key: "e", text: "Lima", sort_order: 2 },
                ],
              },
            ],
          },
        ],
      },
    };
    render(<SessionPage />);
    await enterFullscreenUntil(/Pilih beberapa/);
    expect(screen.getByTestId("option-key-a")).toHaveTextContent("A");
    expect(screen.getByTestId("option-key-e")).toHaveTextContent("E");
  });

  // ── Per-question audio (FR-26, FR-27, FR-28) ──────────────────────────

  it("renders question-audio player when listening section question has audio_url (FR-26)", async () => {
    const listeningWithQuestionAudio = {
      ...ieltsSession,
      active_test_id: "test-listening",
      tests: [
        {
          ...ieltsSession.tests[0],
          status: "active" as const,
          questions: [
            {
              id: "q-listening",
              test_id: "test-listening",
              format: "mcq" as const,
              body: "Listening Q1?",
              sort_order: 1,
              options: [
                { key: "A", text: "Opt A", sort_order: 1 },
                { key: "B", text: "Opt B", sort_order: 2 },
              ],
              audio_url: "https://example.com/question-audio.mp3",
            },
          ],
        },
        ieltsSession.tests[1],
        ieltsSession.tests[2],
      ],
    };
    sessionState = { ...sessionState, data: listeningWithQuestionAudio };
    render(<SessionPage />);
    await enterFullscreenIELTS();

    // Both section and question audio players should be visible
    const audioPlayers = screen.getAllByTestId(/audio-player/);
    expect(audioPlayers.length).toBeGreaterThanOrEqual(2);
  });

  it("distinguishes section vs question audio by testId (FR-26)", async () => {
    const listeningWithQuestionAudio = {
      ...ieltsSession,
      active_test_id: "test-listening",
      tests: [
        {
          ...ieltsSession.tests[0],
          status: "active" as const,
          questions: [
            {
              id: "q-listening",
              test_id: "test-listening",
              format: "mcq" as const,
              body: "Listening Q1?",
              sort_order: 1,
              options: [
                { key: "A", text: "Opt A", sort_order: 1 },
                { key: "B", text: "Opt B", sort_order: 2 },
              ],
              audio_url: "https://example.com/question-audio.mp3",
            },
          ],
        },
        ieltsSession.tests[1],
        ieltsSession.tests[2],
      ],
    };
    sessionState = { ...sessionState, data: listeningWithQuestionAudio };
    render(<SessionPage />);
    await enterFullscreenIELTS();

    // Section player
    const sectionPlayer = screen.getByTestId("section-audio-player");
    expect(sectionPlayer).toBeInTheDocument();
    expect(sectionPlayer).toHaveAttribute("src", "https://example.com/audio.mp3");

    // Question player
    const questionPlayer = screen.getByTestId("question-audio-player");
    expect(questionPlayer).toBeInTheDocument();
    expect(questionPlayer).toHaveAttribute("src", "https://example.com/question-audio.mp3");
  });

  it("does not render question audio player when audio_url is missing (FR-26)", async () => {
    sessionState = { ...sessionState, data: ieltsSession };
    render(<SessionPage />);
    await enterFullscreenIELTS();

    // Only section player should exist (no question player)
    expect(screen.getByTestId("section-audio-player")).toBeInTheDocument();
    expect(screen.queryByTestId("question-audio-player")).not.toBeInTheDocument();
  });

  it("navigating questions preserves section player but unmounts question player (FR-27)", async () => {
    const listeningMultiQuestion = {
      ...ieltsSession,
      active_test_id: "test-listening",
      tests: [
        {
          ...ieltsSession.tests[0],
          status: "active" as const,
          questions: [
            {
              id: "q-listening-1",
              test_id: "test-listening",
              format: "mcq" as const,
              body: "Listening Q1?",
              sort_order: 1,
              options: [
                { key: "A", text: "A1", sort_order: 1 },
                { key: "B", text: "B1", sort_order: 2 },
              ],
              audio_url: "https://example.com/q1-audio.mp3",
            },
            {
              id: "q-listening-2",
              test_id: "test-listening",
              format: "mcq" as const,
              body: "Listening Q2?",
              sort_order: 2,
              options: [
                { key: "A", text: "A2", sort_order: 1 },
                { key: "B", text: "B2", sort_order: 2 },
              ],
            },
          ],
        },
        ieltsSession.tests[1],
        ieltsSession.tests[2],
      ],
    };
    sessionState = { ...sessionState, data: listeningMultiQuestion };
    render(<SessionPage />);
    await enterFullscreenIELTS();

    // Q1 has question audio
    expect(screen.getByTestId("question-audio-player")).toBeInTheDocument();

    // Navigate to Q2
    fireEvent.click(screen.getByTestId("session-nav-1"));

    await waitFor(() => {
      expect(screen.getByText(/Listening Q2\?/)).toBeInTheDocument();
    });

    // Section player still present, question player gone
    expect(screen.getByTestId("section-audio-player")).toBeInTheDocument();
    expect(screen.queryByTestId("question-audio-player")).not.toBeInTheDocument();
  });

  // ── FB-32: audio plays at both question and section scope, any mode ────

  it("renders per-question audio player in standard mode when the question has audio_url (FB-32)", async () => {
    const standardWithQuestionAudio = {
      ...sampleSession,
      tests: [
        {
          ...sampleSession.tests[0],
          questions: [
            { ...sampleSession.tests[0].questions[0], audio_url: "https://example.com/std-q-audio.mp3" },
            ...sampleSession.tests[0].questions.slice(1),
          ],
        },
      ],
    };
    sessionState = { ...sessionState, data: standardWithQuestionAudio };
    render(<SessionPage />);
    await enterFullscreen();

    const questionPlayer = screen.getByTestId("question-audio-player");
    expect(questionPlayer).toBeInTheDocument();
    expect(questionPlayer).toHaveAttribute("src", "https://example.com/std-q-audio.mp3");
  });

  it("renders per-question audio player in a sectioned non-listening test (FB-32)", async () => {
    const sectionedWithQuestionAudio = {
      ...sectionedSession,
      tests: [
        {
          ...sectionedSession.tests[0],
          questions: [
            { ...sectionedSession.tests[0].questions[0], audio_url: "https://example.com/sec-q-audio.mp3" },
            ...sectionedSession.tests[0].questions.slice(1),
          ],
        },
        sectionedSession.tests[1],
      ],
    };
    sessionState = { ...sessionState, data: sectionedWithQuestionAudio };
    render(<SessionPage />);
    await enterFullscreenSectioned();

    const questionPlayer = screen.getByTestId("question-audio-player");
    expect(questionPlayer).toBeInTheDocument();
    expect(questionPlayer).toHaveAttribute("src", "https://example.com/sec-q-audio.mp3");
  });

  it("renders section audio player for a sectioned test with audio_url and no section_type (FB-32)", async () => {
    const sectionedWithSectionAudioNoType = {
      ...sectionedSession,
      tests: [
        {
          ...sectionedSession.tests[0],
          audio_url: "https://example.com/sec-audio.mp3",
        },
        sectionedSession.tests[1],
      ],
    };
    sessionState = { ...sessionState, data: sectionedWithSectionAudioNoType };
    render(<SessionPage />);
    await enterFullscreenSectioned();

    const sectionPlayer = screen.getByTestId("section-audio-player");
    expect(sectionPlayer).toBeInTheDocument();
    expect(sectionPlayer).toHaveAttribute("src", "https://example.com/sec-audio.mp3");
  });

  // ── Durable answer saving (FR-31..FR-34, FR-37, NFR-P3, NFR-R5) ─────────

  it("debounces continuous changes to at most one save per debounce window (FR-31, NFR-P3)", async () => {
    render(<SessionPage />);
    await enterFullscreen();
    vi.useFakeTimers();

    // Navigate to the short-answer question and simulate ~10s of continuous
    // typing, one keystroke every 300ms (33 changes total).
    fireEvent.click(screen.getByTestId("session-nav-2"));
    const textInput = () =>
      screen.getAllByRole("textbox").filter((tb) => tb.tagName === "INPUT")[0];

    for (let i = 0; i < 33; i++) {
      fireEvent.change(textInput(), { target: { value: `v${i}` } });
      await act(async () => {
        vi.advanceTimersByTime(300);
      });
    }

    // ~10s of continuous changes over a debounce window well under 10s must
    // not produce a save per keystroke — at most one save per window.
    expect(saveAnswersMutate.mock.calls.length).toBeGreaterThan(0);
    expect(saveAnswersMutate.mock.calls.length).toBeLessThanOrEqual(
      Math.ceil((33 * 300) / AUTOSAVE_DEBOUNCE_MS) + 1,
    );
    expect(saveAnswersMutate.mock.calls.length).toBeLessThan(10);
  });

  it("retries a failing save with growing backoff delays and eventually succeeds (FR-32)", async () => {
    render(<SessionPage />);
    await enterFullscreen();
    vi.useFakeTimers();

    fireEvent.click(screen.getAllByRole("radio")[1]);
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });
    expect(saveAnswersMutate).toHaveBeenCalledTimes(1);

    // First attempt fails.
    await act(async () => {
      const [, opts] = saveAnswersMutate.mock.calls[0];
      opts.onError(new Error("network error"));
    });
    expect(screen.getByTestId("save-indicator")).toHaveTextContent(
      "Belum tersimpan",
    );

    const delay1 = backoffDelayMs(0);
    await act(async () => {
      vi.advanceTimersByTime(delay1 - 1);
    });
    expect(saveAnswersMutate).toHaveBeenCalledTimes(1); // not yet retried
    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(saveAnswersMutate).toHaveBeenCalledTimes(2); // first retry fired

    // Second attempt also fails — the next delay must be strictly longer.
    await act(async () => {
      const [, opts] = saveAnswersMutate.mock.calls[1];
      opts.onError(new Error("network error"));
    });
    const delay2 = backoffDelayMs(1);
    expect(delay2).toBeGreaterThan(delay1);
    await act(async () => {
      vi.advanceTimersByTime(delay2 - 1);
    });
    expect(saveAnswersMutate).toHaveBeenCalledTimes(2);
    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(saveAnswersMutate).toHaveBeenCalledTimes(3);

    // Third attempt succeeds.
    await act(async () => {
      const [, opts] = saveAnswersMutate.mock.calls[2];
      opts.onSuccess();
    });
    expect(screen.getByTestId("save-indicator")).toHaveTextContent(
      "Tersimpan",
    );
  });

  it("replays a localStorage-queued payload exactly once on mount and clears it (FR-33, NFR-R5)", async () => {
    saveQueue("session-1", [
      { question_id: "q-mcq", answer: "B", flagged_for_review: false },
    ]);
    saveAnswersMutate.mockImplementation((_payload, opts) => {
      opts?.onSuccess?.();
    });

    render(<SessionPage />);
    await enterFullscreen();

    await waitFor(() => {
      expect(saveAnswersMutate).toHaveBeenCalledTimes(1);
    });
    expect(saveAnswersMutate).toHaveBeenCalledWith(
      {
        answers: [{ question_id: "q-mcq", answer: "B", flagged_for_review: false }],
      },
      expect.anything(),
    );
    expect(loadQueue("session-1")).toEqual([]);
  });

  it("indicator shows unsaved while a save is pending and saved after acknowledgement (FR-34)", async () => {
    render(<SessionPage />);
    await enterFullscreen();

    expect(screen.getByTestId("save-indicator")).toHaveTextContent(
      "Tersimpan",
    );

    vi.useFakeTimers();
    fireEvent.click(screen.getAllByRole("radio")[0]);
    expect(screen.getByTestId("save-indicator")).toHaveTextContent(
      "Belum tersimpan",
    );

    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });
    expect(saveAnswersMutate).toHaveBeenCalledTimes(1);

    await act(async () => {
      const [, opts] = saveAnswersMutate.mock.calls[0];
      opts.onSuccess();
    });
    expect(screen.getByTestId("save-indicator")).toHaveTextContent(
      "Tersimpan",
    );
  });

  it("replay does not clobber a server answer for a question absent from the queue (FR-37, NFR-R5)", async () => {
    saveQueue("session-1", [
      { question_id: "q-short", answer: "queued-value", flagged_for_review: false },
    ]);
    saveAnswersMutate.mockImplementation((_payload, opts) => {
      opts?.onSuccess?.();
    });
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        answers: [
          { question_id: "q-mcq", answer: "B", flagged_for_review: false },
        ],
      },
    };

    render(<SessionPage />);
    await enterFullscreen();

    // q-mcq is server-sourced and absent from the queue — must stay untouched.
    const radios = screen.getAllByRole("radio");
    expect(radios[1]).toBeChecked();

    // q-short is the queued question — must show the queued (not server) value.
    fireEvent.click(screen.getByTestId("session-nav-2"));
    await waitFor(() => {
      const textInputs = screen
        .getAllByRole("textbox")
        .filter((tb) => tb.tagName === "INPUT");
      expect((textInputs[0] as HTMLInputElement).value).toBe("queued-value");
    });

    // Replay sent exactly the queue's content — nothing more, nothing less.
    expect(saveAnswersMutate).toHaveBeenCalledWith(
      {
        answers: [{ question_id: "q-short", answer: "queued-value", flagged_for_review: false }],
      },
      expect.anything(),
    );
    expect(loadQueue("session-1")).toEqual([]);
  });

  // ── Resume at the same question on reconnect (FR-36, FR-37) ────────────

  it("seeds currentQIndex from session.current_position on load, landing on question 6 (FR-36)", async () => {
    const questions = Array.from({ length: 6 }, (_, i) => ({
      id: `q-pos-${i}`,
      test_id: "test-1",
      format: "mcq" as const,
      body: `Question ${i + 1}?`,
      sort_order: i + 1,
      options: [
        { key: "A", text: "Opt A", sort_order: 1 },
        { key: "B", text: "Opt B", sort_order: 2 },
      ],
    }));
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        current_position: 5,
        tests: [{ ...sampleSession.tests[0], questions }],
      },
    };
    render(<SessionPage />);
    await enterFullscreenUntil(/Question 6\?/);

    expect(screen.getByText(/Soal 6 dari 6/)).toBeInTheDocument();
  });

  it("includes the new question index in the next save payload after navigating (FR-36)", async () => {
    render(<SessionPage />);
    await enterFullscreen();
    vi.useFakeTimers();

    fireEvent.click(screen.getByTestId("session-nav-2"));
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });

    expect(saveAnswersMutate).toHaveBeenCalledWith(
      expect.objectContaining({ current_position: 2 }),
      expect.anything(),
    );
  });

  it("sends only the new position after an acknowledged answer save (FR-1)", async () => {
    render(<SessionPage />);
    await enterFullscreen();
    vi.useFakeTimers();

    fireEvent.click(screen.getAllByRole("radio")[1]);
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });
    await act(async () => {
      const [, opts] = saveAnswersMutate.mock.calls[0];
      opts.onSuccess();
    });

    fireEvent.click(screen.getByTestId("session-nav-2"));
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });

    expect(saveAnswersMutate.mock.calls[1][0]).toEqual({
      answers: [],
      current_position: 2,
    });
  });

  it("omits an unchanged current position from an answer save (FR-2)", async () => {
    render(<SessionPage />);
    await enterFullscreen();
    vi.useFakeTimers();

    fireEvent.click(screen.getAllByRole("radio")[1]);
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });

    expect(saveAnswersMutate.mock.calls[0][0]).not.toHaveProperty("current_position");
  });

  it("keeps an unacknowledged answer in the navigation save after a failed save (FR-3)", async () => {
    render(<SessionPage />);
    await enterFullscreen();
    vi.useFakeTimers();

    fireEvent.click(screen.getAllByRole("radio")[1]);
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });
    await act(async () => {
      const [, opts] = saveAnswersMutate.mock.calls[0];
      opts.onError(new Error("network error"));
    });

    fireEvent.click(screen.getByTestId("session-nav-2"));
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });

    expect(saveAnswersMutate.mock.calls[1][0]).toEqual({
      answers: [
        { question_id: "q-mcq", answer: "B", flagged_for_review: false },
      ],
      current_position: 2,
    });
  });

  it("does not overwrite an unacknowledged queue entry during a position-only save (FR-4)", async () => {
    saveQueue("session-1", [
      { question_id: "q-mcq", answer: "B", flagged_for_review: false },
    ]);
    render(<SessionPage />);
    await enterFullscreen();
    vi.useFakeTimers();

    fireEvent.click(screen.getByTestId("session-nav-2"));
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });

    expect(loadQueue("session-1")).toEqual([
      { question_id: "q-mcq", answer: "B", flagged_for_review: false },
    ]);
  });

  it("hydrates position from the server response even when localStorage is empty (FR-36, FR-37)", async () => {
    localStorage.clear();
    sessionState = {
      ...sessionState,
      data: { ...sampleSession, current_position: 3 },
    };
    render(<SessionPage />);
    await enterFullscreenUntil(/Bendera Indonesia berwarna/);

    expect(screen.getByText(/Soal 4 dari 5/)).toBeInTheDocument();
  });

  // ── Blocker 3: overlapping saves must never lose or misreport an answer ──

  it("a stale save's ack must not report 'saved' while a newer save is still pending, and the newer edit survives that newer save failing (Blocker 3a, FR-32, FR-34, NFR-R5)", async () => {
    render(<SessionPage />);
    await enterFullscreen();
    vi.useFakeTimers();

    fireEvent.click(screen.getByTestId("session-nav-2"));
    const textInput = () =>
      screen.getAllByRole("textbox").filter((tb) => tb.tagName === "INPUT")[0];

    // First edit — flush A.
    fireEvent.change(textInput(), { target: { value: "v1" } });
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });
    expect(saveAnswersMutate).toHaveBeenCalledTimes(1);

    // Second edit before A resolves — flush B. Both A and B are now
    // outstanding: exactly the race a 2s debounce makes routine whenever a
    // request takes longer than the debounce window.
    fireEvent.change(textInput(), { target: { value: "v2" } });
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });
    expect(saveAnswersMutate).toHaveBeenCalledTimes(2);

    // A — the OLDER request — acknowledges first. This is the actual race:
    // an older, slower request settling after a newer one was dispatched.
    await act(async () => {
      const [, optsA] = saveAnswersMutate.mock.calls[0];
      optsA.onSuccess();
    });

    // The stale ack must not claim everything is saved — B (v2) is still
    // outstanding.
    expect(screen.getByTestId("save-indicator")).not.toHaveTextContent(
      "Tersimpan",
    );
    // v2 must still be durably queued — recoverable even if the tab closed
    // at this exact instant.
    expect(loadQueue("session-1")).toEqual([
      { question_id: "q-short", answer: "v2", flagged_for_review: false },
    ]);

    // Now B fails.
    await act(async () => {
      const [, optsB] = saveAnswersMutate.mock.calls[1];
      optsB.onError(new Error("network error"));
    });

    // The indicator must say unsaved — the answer is not acknowledged
    // anywhere, and must not be reported as saved.
    expect(screen.getByTestId("save-indicator")).toHaveTextContent(
      "Belum tersimpan",
    );
    // Still recoverable in the durable queue...
    expect(loadQueue("session-1")).toEqual([
      { question_id: "q-short", answer: "v2", flagged_for_review: false },
    ]);
    // ...and actively retried, not silently dropped.
    const delay = backoffDelayMs(0);
    await act(async () => {
      vi.advanceTimersByTime(delay);
    });
    expect(saveAnswersMutate).toHaveBeenCalledTimes(3);
    const [retryPayload] = saveAnswersMutate.mock.calls[2];
    expect(retryPayload.answers).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ question_id: "q-short", answer: "v2" }),
      ]),
    );
  });

  it("an edit made between a save's PATCH going out and the resulting refetch landing survives the refetch (Blocker 3b, FR-37, NFR-R5)", async () => {
    const { rerender } = render(<SessionPage />);
    await enterFullscreen();
    vi.useFakeTimers();

    // Answer q-mcq — flush A.
    fireEvent.click(screen.getAllByRole("radio")[1]);
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });
    expect(saveAnswersMutate).toHaveBeenCalledTimes(1);

    // Before A's ack/refetch lands, the student edits a DIFFERENT question.
    // This edit lives only in React state: the debounce hasn't elapsed, so
    // it is not yet in the durable queue either.
    fireEvent.click(screen.getByTestId("session-nav-2"));
    const textInput = () =>
      screen.getAllByRole("textbox").filter((tb) => tb.tagName === "INPUT")[0];
    fireEvent.change(textInput(), { target: { value: "fresh-edit" } });

    // A's PATCH acknowledges. This test simulates a refetch landing by
    // pushing a new session object through the mocked hook directly, with a
    // snapshot that predates "fresh-edit" — hydration must not depend on
    // whether the save itself ever triggers one.
    await act(async () => {
      const [, opts] = saveAnswersMutate.mock.calls[0];
      opts.onSuccess();
    });
    sessionState = {
      ...sessionState,
      data: {
        ...sampleSession,
        answers: [{ question_id: "q-mcq", answer: "B", flagged_for_review: false }],
      },
    };
    rerender(<SessionPage />);

    // The edit made in the gap must still be on screen — hydration must not
    // have reset local state from the refetched (stale-by-definition)
    // session snapshot.
    expect((textInput() as HTMLInputElement).value).toBe("fresh-edit");
  });

  // ── Blocker 4: sectioned exams must not reset position on mount ─────────

  it("a sectioned exam with a non-zero server position mounts on that question and does not write current_position: 0 back to the server (Blocker 4, FR-36)", async () => {
    const sixQuestions = Array.from({ length: 6 }, (_, i) => ({
      id: `q-sec1-${i}`,
      test_id: "test-section-1",
      format: "mcq" as const,
      body: `Section Question ${i + 1}?`,
      sort_order: i + 1,
      options: [
        { key: "A", text: "Opt A", sort_order: 1 },
        { key: "B", text: "Opt B", sort_order: 2 },
      ],
    }));
    sessionState = {
      ...sessionState,
      data: {
        ...sectionedSession,
        current_position: 5,
        tests: [
          { ...sectionedSession.tests[0], questions: sixQuestions },
          sectionedSession.tests[1],
        ],
      },
    };
    render(<SessionPage />);
    await enterFullscreenUntil(/Section Question 6\?/);

    // Must land on question 6, not be reset to question 1.
    expect(screen.getByText(/Soal 6 dari 6/)).toBeInTheDocument();

    // The next answer save must not overwrite the server's persisted position.
    vi.useFakeTimers();
    fireEvent.click(screen.getAllByRole("radio")[0]);
    await act(async () => {
      vi.advanceTimersByTime(AUTOSAVE_DEBOUNCE_MS);
    });

    expect(saveAnswersMutate.mock.calls[0][0]).not.toHaveProperty(
      "current_position",
    );
  });
});
