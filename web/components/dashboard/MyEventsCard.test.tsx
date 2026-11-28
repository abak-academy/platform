import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MyEventsCard } from "./MyEventsCard";
import type { RegistrationListItem, SessionResult, StudentCompetitionResult } from "@/lib/types";

const mockAuthFetch = vi.fn();

vi.mock("@/lib/api", () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
  ApiError: class extends Error {},
}));
vi.mock("@/stores/auth", () => ({ useAuthStore: { getState: () => ({ token: "t" }) } }));

const NOW = Date.parse("2026-11-26T05:00:00Z");
const HOUR = 3600_000;
const iso = (offsetMs: number) => new Date(NOW + offsetMs).toISOString();

function reg(over: Partial<RegistrationListItem> = {}): RegistrationListItem {
  return {
    id: "reg-1",
    student_id: "stu-1",
    exam_id: "exam-1",
    token: "TKN",
    card_key: null,
    checked_in_at: null,
    attempts_used: 0,
    status: "registered",
    created_at: iso(-48 * HOUR),
    exam_title: "Tryout Alpha",
    scheduled_at: iso(48 * HOUR),
    scheduled_end_at: iso(50 * HOUR),
    is_free: true,
    requires_checkin: false,
    check_in_window_minutes: null,
    duration_minutes: 90,
    session_id: null,
    max_attempts: 1,
    navigation_policy: "free",
    timer_mode: "overall",
    section_count: 1,
    question_count: 10,
    sections_duration_minutes: null,
    competition_result: null,
    ...over,
  };
}

const submitted = (over: Partial<RegistrationListItem> = {}) =>
  reg({
    id: "reg-sub",
    exam_title: "Tryout Selesai",
    status: "submitted",
    attempts_used: 1,
    scheduled_at: iso(-30 * HOUR),
    scheduled_end_at: iso(-28 * HOUR),
    session_id: "sess-1",
    ...over,
  });

const compResult = (over: Partial<StudentCompetitionResult> = {}): StudentCompetitionResult => ({
  score: 87.5,
  rank: 12,
  award: "Medali Emas",
  certificate_url: "https://certs.example.test/c1.pdf",
  published_at: iso(-HOUR),
  ...over,
});

const fullResult = {
  state: "result",
  result_config: "score_only",
  score: 80,
  correct_count: 8,
  wrong_count: 2,
  empty_count: 0,
  rank: 3,
} as SessionResult;

function renderCard(regs: RegistrationListItem[], state: { isLoading?: boolean; isError?: boolean } = {}) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MyEventsCard registrations={regs} now={NOW} {...state} />
    </QueryClientProvider>,
  );
}

function answerResult(result: SessionResult | Error) {
  if (result instanceof Error) mockAuthFetch.mockRejectedValue(result);
  else mockAuthFetch.mockResolvedValue(result);
}

const resultCalls = () => mockAuthFetch.mock.calls.filter((c) => String(c[0]).includes("/result"));

beforeEach(() => {
  mockAuthFetch.mockReset();
});

describe("MyEventsCard", () => {
  it("shows the empty state with a link to the catalog", () => {
    renderCard([]);
    expect(screen.getByText("Belum ada event yang kamu ikuti.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Lihat event" })).toHaveAttribute("href", "/catalog");
  });

  it("shows skeleton rows, not the empty copy, while registrations load", () => {
    renderCard([], { isLoading: true });
    expect(screen.getByTestId("events-loading")).toBeInTheDocument();
    expect(screen.queryByText("Belum ada event yang kamu ikuti.")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Lihat semua" })).toBeInTheDocument();
  });

  it("shows a neutral failure line, not the empty copy, when registrations fail", () => {
    renderCard([], { isError: true });
    expect(screen.getByText("Gagal memuat dashboard.")).toBeInTheDocument();
    expect(screen.queryByText("Belum ada event yang kamu ikuti.")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Lihat event" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Lihat semua" })).toHaveAttribute("href", "/exam");
  });

  it("has a header link to /exam", () => {
    renderCard([reg()]);
    expect(screen.getByRole("link", { name: "Lihat semua" })).toHaveAttribute("href", "/exam");
  });

  it("renders title, minutes, date tile and Detail for an upcoming exam", () => {
    renderCard([reg()]);
    expect(screen.getByText("Tryout Alpha")).toBeInTheDocument();
    expect(screen.getByText("90 menit")).toBeInTheDocument();
    expect(screen.getByText("28")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Detail" })).toHaveAttribute("href", "/exam/reg-1");
    expect(screen.queryByText("Sedang berlangsung")).not.toBeInTheDocument();
  });

  it("omits minutes when unknown and shows a dash when unscheduled", () => {
    renderCard([reg({ duration_minutes: null, scheduled_at: null, scheduled_end_at: null })]);
    expect(screen.queryByText(/menit/)).not.toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
  });

  it("shows Mulai and the live label for a live exam", () => {
    renderCard([reg({ scheduled_at: iso(-HOUR), scheduled_end_at: iso(HOUR) })]);
    expect(screen.getByText("Sedang berlangsung")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Mulai" })).toHaveAttribute("href", "/exam/reg-1");
  });

  it("shows Terlewat for an expired exam", () => {
    renderCard([reg({ scheduled_at: iso(-30 * HOUR), scheduled_end_at: iso(-28 * HOUR) })]);
    expect(screen.getByText("Terlewat")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Detail" })).not.toBeInTheDocument();
  });

  it("caps rows at 4", () => {
    renderCard(
      [1, 2, 3, 4, 5].map((i) => reg({ id: `r${i}`, exam_title: `Ujian ${i}`, scheduled_at: iso(i * HOUR), scheduled_end_at: iso((i + 1) * HOUR + 24 * HOUR) })),
    );
    expect(screen.getAllByRole("listitem")).toHaveLength(4);
  });

  it("never prints the participant number", () => {
    renderCard([reg({ token: "SECRET-TOKEN-1" })]);
    expect(screen.queryByText(/SECRET-TOKEN-1/)).not.toBeInTheDocument();
  });

  it("does not request results for rows that are not submitted", async () => {
    renderCard([reg(), reg({ id: "r2", scheduled_at: iso(-HOUR), scheduled_end_at: iso(HOUR) })]);
    await screen.findAllByRole("link", { name: /Detail|Mulai/ });
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });
});

describe("MyEventsCard on-platform result", () => {
  it.each(["grading", "locked"] as const)("shows Menunggu hasil while %s", async (state) => {
    answerResult({ state, result_release_at: iso(HOUR) } as SessionResult);
    renderCard([submitted()]);
    expect(await screen.findByText("Menunggu hasil")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Lihat hasil" })).not.toBeInTheDocument();
    expect(resultCalls()).toHaveLength(1);
    expect(String(resultCalls()[0][0])).toContain("/exam/sessions/sess-1/result");
  });

  it("shows a Selesai chip without a link when hidden", async () => {
    answerResult({ state: "hidden" } as SessionResult);
    renderCard([submitted()]);
    expect(await screen.findByText("Selesai")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Lihat hasil" })).not.toBeInTheDocument();
    expect(screen.queryByText("Menunggu hasil")).not.toBeInTheDocument();
  });

  it("links Lihat hasil to the leaderboard for a full result", async () => {
    answerResult(fullResult);
    renderCard([submitted()]);
    expect(await screen.findByRole("link", { name: "Lihat hasil" })).toHaveAttribute(
      "href",
      "/exam/sessions/sess-1/leaderboard",
    );
  });

  it("shows a placeholder while the result loads", () => {
    mockAuthFetch.mockReturnValue(new Promise(() => {}));
    renderCard([submitted()]);
    expect(screen.getByTestId("event-result-loading")).toBeInTheDocument();
    expect(screen.queryByText("Menunggu hasil")).not.toBeInTheDocument();
  });

  it("falls back to a Detail link when the result call fails", async () => {
    answerResult(new Error("boom"));
    renderCard([submitted()]);
    expect(await screen.findByRole("link", { name: "Detail" })).toHaveAttribute("href", "/exam/reg-sub");
    expect(screen.queryByText("Menunggu hasil")).not.toBeInTheDocument();
  });
});

describe("MyEventsCard competition result", () => {
  it("opens a dialog with score, rank, award and certificate", async () => {
    const user = userEvent.setup();
    renderCard([reg({ exam_title: "Olimpiade Nasional", competition_result: compResult() })]);
    expect(mockAuthFetch).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Lihat hasil" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Hasil Olimpiade Nasional")).toBeInTheDocument();
    expect(within(dialog).getByText(/87[.,]5/)).toBeInTheDocument();
    expect(within(dialog).getByText(/12/)).toBeInTheDocument();
    expect(within(dialog).getByText("Medali Emas")).toBeInTheDocument();
    const cert = within(dialog).getAllByRole("link", { name: "Lihat sertifikat" });
    expect(cert).toHaveLength(1);
    expect(cert[0]).toHaveAttribute("href", "https://certs.example.test/c1.pdf");
    expect(cert[0]).toHaveAttribute("target", "_blank");
    expect(cert[0]).toHaveAttribute("rel", "noopener noreferrer");
    expect(within(dialog).queryByRole("link", { name: "Lihat papan peringkat" })).not.toBeInTheDocument();
  });

  it("omits rank, award and certificate when the result has none", async () => {
    const user = userEvent.setup();
    renderCard([reg({ competition_result: compResult({ rank: null, award: null, certificate_url: null }) })]);
    await user.click(screen.getByRole("button", { name: "Lihat hasil" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).queryByRole("link", { name: "Lihat sertifikat" })).not.toBeInTheDocument();
    expect(within(dialog).queryByText("Medali Emas")).not.toBeInTheDocument();
    expect(within(dialog).queryByText(/#/)).not.toBeInTheDocument();
  });

  it("adds a leaderboard link when an on-platform result is also released", async () => {
    answerResult(fullResult);
    const user = userEvent.setup();
    renderCard([submitted({ competition_result: compResult() })]);
    await user.click(screen.getByRole("button", { name: "Lihat hasil" }));
    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByRole("link", { name: "Lihat papan peringkat" })).toHaveAttribute(
      "href",
      "/exam/sessions/sess-1/leaderboard",
    );
  });

  it("omits the leaderboard link when the on-platform result is gated", async () => {
    answerResult({ state: "grading" } as SessionResult);
    const user = userEvent.setup();
    renderCard([submitted({ competition_result: compResult() })]);
    await user.click(screen.getByRole("button", { name: "Lihat hasil" }));
    const dialog = await screen.findByRole("dialog");
    await waitFor(() => expect(resultCalls()).toHaveLength(1));
    await mockAuthFetch.mock.results[0].value;
    await waitFor(() => expect(within(dialog).queryByRole("link", { name: "Lihat papan peringkat" })).not.toBeInTheDocument());
    expect(within(dialog).queryByRole("link", { name: "Lihat papan peringkat" })).not.toBeInTheDocument();
  });

  it("does not request the result for a competition row that is not submitted", async () => {
    renderCard([reg({ competition_result: compResult(), session_id: "sess-x" })]);
    await screen.findByRole("button", { name: "Lihat hasil" });
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("a null competition_result falls through to the normal action", () => {
    renderCard([reg({ competition_result: null })]);
    expect(screen.getByRole("link", { name: "Detail" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Lihat hasil" })).not.toBeInTheDocument();
  });
});

describe("MyEventsCard button mapping (prototype evListRow)", () => {
  it("draws Detail and Mulai as outline pills", () => {
    renderCard([reg(), reg({ id: "r2", scheduled_at: iso(-HOUR), scheduled_end_at: iso(HOUR) })]);
    expect(screen.getByRole("link", { name: "Detail" })).toHaveAttribute("data-variant", "outline");
    expect(screen.getByRole("link", { name: "Mulai" })).toHaveAttribute("data-variant", "outline");
  });

  it("draws Lihat hasil as the filled ember pill", async () => {
    answerResult(fullResult);
    renderCard([submitted()]);
    expect(await screen.findByRole("link", { name: "Lihat hasil" })).toHaveAttribute("data-variant", "default");
  });

  it("draws the competition Lihat hasil as the filled ember pill", () => {
    renderCard([reg({ competition_result: compResult() })]);
    expect(screen.getByRole("button", { name: "Lihat hasil" })).toHaveAttribute("data-variant", "default");
  });

  it("draws Menunggu hasil and Terlewat as mute chips, not buttons", async () => {
    answerResult({ state: "grading", result_release_at: iso(HOUR) } as SessionResult);
    renderCard([submitted(), reg({ id: "r-exp", scheduled_at: iso(-30 * HOUR), scheduled_end_at: iso(-28 * HOUR) })]);
    expect(await screen.findByText("Menunggu hasil")).toHaveAttribute("data-variant", "mute");
    expect(screen.getByText("Terlewat")).toHaveAttribute("data-variant", "mute");
  });
});
