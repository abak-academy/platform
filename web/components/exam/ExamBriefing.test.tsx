import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { ExamBriefing, type BriefingGate } from "./ExamBriefing";
import type { RegistrationExam } from "@/lib/types";

let uiStore = { lang: "id" as "id" | "en" };
vi.mock("@/stores/ui", () => ({
  useUIStore: (selector: (s: typeof uiStore) => unknown) => selector(uiStore),
}));

const WA = "https://chat.whatsapp.com/AbCdEf";

const exam: RegistrationExam = {
  id: "e-1",
  title: "Try Out Nasional",
  scheduled_at: "2026-07-15T09:00:00Z",
  scheduled_end_at: null,
  requires_checkin: false,
  check_in_window_minutes: null,
  timer_mode: "overall",
  navigation_policy: "free",
  max_attempts: 1,
  duration_minutes: 90,
  result_config: "hidden",
  card_enabled: false,
  card_notes: [],
  section_count: 1,
  question_count: 10,
  sections_duration_minutes: 90,
  sections: [{ test_id: "t-1", title: "Penalaran", duration_minutes: 90, question_count: 10, question_presentation: "one_per_page" }],
};

const gate = { kind: "start" as const, disabled: false, pending: false, onStart: vi.fn() };

function renderBriefing(
  over: Partial<RegistrationExam> = {},
  props: { preview?: boolean; status?: "ready" | "upcoming"; gate?: BriefingGate; afterHowTo?: React.ReactNode } = {},
) {
  return render(
    <ExamBriefing
      exam={{ ...exam, ...over }}
      backHref="/exam"
      backLabel="Kembali"
      gate={props.preview ? null : (props.gate ?? gate)}
      preview={props.preview}
      status={props.status}
      afterHowTo={props.afterHowTo}
    />,
  );
}

const waLink = () => screen.queryByRole("link", { name: /gabung grup whatsapp/i });
const rulesHeading = () => screen.queryByRole("heading", { name: /aturan tambahan dari penyelenggara/i });

describe("ExamBriefing whatsapp invitation", () => {
  beforeEach(() => {
    uiStore = { lang: "id" };
    gate.onStart.mockReset();
  });

  it("sits after Informasi ujian and before Cara mengerjakan", () => {
    renderBriefing({ whatsapp_group_url: WA });
    const info = screen.getByRole("heading", { name: "Informasi ujian" });
    const community = screen.getByRole("heading", { name: "Grup WhatsApp kompetisi" });
    const howto = screen.getByRole("heading", { name: "Cara mengerjakan" });
    expect(info.compareDocumentPosition(community) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(community.compareDocumentPosition(howto) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("links to the group in a new tab with noopener noreferrer and says it is optional", () => {
    renderBriefing({ whatsapp_group_url: WA });
    const link = waLink()!;
    expect(link).toHaveAttribute("href", WA);
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.getByText(/opsional, bukan syarat mengikuti ujian/i)).toBeInTheDocument();
  });

  it.each([undefined, null, ""])("is absent when the url is %s", (value) => {
    renderBriefing({ whatsapp_group_url: value });
    expect(waLink()).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Grup WhatsApp kompetisi" })).not.toBeInTheDocument();
  });

  it("renders for an upcoming exam and for an open exam alike", () => {
    renderBriefing({ whatsapp_group_url: WA, scheduled_at: "2999-01-01T00:00:00Z" });
    expect(waLink()).toBeInTheDocument();
  });

  it("does not affect starting: start stays gated by the understanding checkbox only", () => {
    renderBriefing({ whatsapp_group_url: WA });
    const start = screen.getByRole("button", { name: /mulai/i });
    expect(start).toBeDisabled();
    fireEvent.click(waLink()!);
    expect(start).toBeDisabled();
    fireEvent.click(screen.getByRole("checkbox", { name: /saya memahami aturan/i }));
    expect(start).toBeEnabled();
    fireEvent.click(start);
    expect(gate.onStart).toHaveBeenCalledTimes(1);
  });

  it("renders in preview mode too", () => {
    renderBriefing({ whatsapp_group_url: WA }, { preview: true });
    expect(waLink()).toHaveAttribute("href", WA);
  });
});

describe("ExamBriefing additional rules", () => {
  beforeEach(() => {
    uiStore = { lang: "id" };
  });

  it("shows nothing when neither note nor link is stored", () => {
    renderBriefing({ additional_rules_note: null, additional_rules_url: null });
    expect(rulesHeading()).not.toBeInTheDocument();
  });

  it("shows a note alone, inside the Cara mengerjakan card", () => {
    renderBriefing({ additional_rules_note: "Baca panduan." });
    expect(screen.getByText("Baca panduan.")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /baca aturan lengkap/i })).not.toBeInTheDocument();
    const howto = screen.getByRole("heading", { name: "Cara mengerjakan" });
    expect(howto.parentElement!.contains(rulesHeading())).toBe(true);
  });

  it("shows a link alone, opening in a new tab safely", () => {
    renderBriefing({ additional_rules_url: "https://example.com/aturan" });
    const link = screen.getByRole("link", { name: /baca aturan lengkap/i });
    expect(link).toHaveAttribute("href", "https://example.com/aturan");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
    expect(rulesHeading()).toBeInTheDocument();
  });

  it("shows both note and link", () => {
    renderBriefing({ additional_rules_note: "Catatan", additional_rules_url: "https://example.com/aturan" });
    expect(screen.getByText("Catatan")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /baca aturan lengkap/i })).toBeInTheDocument();
  });

  it("keeps newlines and renders markup as plain text", () => {
    const { container } = renderBriefing({ additional_rules_note: "Baris satu\nBaris <b>dua</b>" });
    const note = screen.getByText(/baris satu/i);
    expect(note.textContent).toBe("Baris satu\nBaris <b>dua</b>");
    expect(note).toHaveClass("whitespace-pre-line");
    expect(container.querySelector("b")).toBeNull();
  });

  it("does not gate the start button, and renders in preview mode", () => {
    renderBriefing({ additional_rules_url: "https://example.com/aturan" });
    const start = screen.getByRole("button", { name: /mulai/i });
    fireEvent.click(screen.getByRole("link", { name: /baca aturan lengkap/i }));
    expect(start).toBeDisabled();
    fireEvent.click(screen.getByRole("checkbox", { name: /saya memahami aturan/i }));
    expect(start).toBeEnabled();
  });

  it("renders in preview mode", () => {
    renderBriefing({ additional_rules_note: "Catatan" }, { preview: true });
    expect(screen.getByText("Catatan")).toBeInTheDocument();
  });
});

describe("ExamBriefing stored-value hardening", () => {
  it("renders no join anchor when the stored whatsapp value is not a group link", () => {
    renderBriefing({ whatsapp_group_url: "javascript:alert(1)" });
    expect(waLink()).not.toBeInTheDocument();
  });

  it("renders no rules anchor for a javascript: url but keeps the note", () => {
    renderBriefing({ additional_rules_note: "Catatan", additional_rules_url: "javascript:alert(1)" });
    expect(screen.getByText("Catatan")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /baca aturan lengkap/i })).not.toBeInTheDocument();
  });
});

describe("ExamBriefing information card", () => {
  beforeEach(() => {
    uiStore = { lang: "id" };
  });
  const schedule = { scheduled_at: "2026-11-07T01:00:00Z", scheduled_end_at: "2026-11-07T03:00:00Z" };

  it.each([
    ["ready", { status: "ready" as const }, "Siap dikerjakan"],
    ["upcoming", { status: "upcoming" as const }, "Belum dibuka"],
    ["resume", { gate: { ...gate, kind: "resume" as const } }, "Sedang berjalan"],
  ])("shows the %s status badge in the card header", (_n, props, label) => {
    renderBriefing(schedule, props);
    const card = screen.getByRole("heading", { name: "Informasi ujian" }).closest("[data-slot='card']") as HTMLElement;
    expect(within(card).getByText(label)).toBeInTheDocument();
  });

  it("shows the full weekday date and large hours with WIB for a same-day window", () => {
    renderBriefing(schedule);
    expect(screen.getByText("Sabtu, 7 Nov 2026")).toBeInTheDocument();
    const hours = screen.getByTestId("access-hours");
    expect(hours.textContent).toMatch(/08[.:]00\s*–\s*10[.:]00\s*WIB/);
  });

  it("splits a cross-day window into two dated lines", () => {
    renderBriefing({ scheduled_at: "2026-11-07T01:00:00Z", scheduled_end_at: "2026-11-08T03:00:00Z" });
    expect(screen.getByText(/Sabtu, 7 Nov 2026, 08[.:]00 WIB/)).toBeInTheDocument();
    expect(screen.getByText(/Minggu, 8 Nov 2026, 10[.:]00 WIB/)).toBeInTheDocument();
  });

  it("shows total minutes and the per-section timer scope", () => {
    renderBriefing({ timer_mode: "per_test", sections_duration_minutes: 35 });
    const panel = screen.getByTestId("duration-panel");
    expect(within(panel).getByText("35")).toBeInTheDocument();
    expect(within(panel).getByText(/menit total/)).toBeInTheDocument();
    expect(within(panel).getByText("Timer per section")).toBeInTheDocument();
  });

  it("shows plain minutes and the single-timer scope for an overall timer", () => {
    renderBriefing({ timer_mode: "overall", duration_minutes: 90 });
    const panel = screen.getByTestId("duration-panel");
    expect(within(panel).getByText("90")).toBeInTheDocument();
    expect(within(panel).queryByText(/total/)).not.toBeInTheDocument();
    expect(within(panel).getByText("Satu timer ujian")).toBeInTheDocument();
  });
});

describe("ExamBriefing how-to card", () => {
  beforeEach(() => {
    uiStore = { lang: "id" };
  });
  const howto = () => screen.getByRole("heading", { name: "Cara mengerjakan" }).closest("[data-slot='card']") as HTMLElement;

  it("states the total and the carry-over rule for per-section timers", () => {
    renderBriefing({ timer_mode: "per_test", sections_duration_minutes: 35, section_count: 2 });
    expect(within(howto()).getByText(/Total 35 menit untuk 2 section/)).toBeInTheDocument();
    expect(within(howto()).getByText(/tidak dibawa ke section berikutnya/)).toBeInTheDocument();
  });

  it("states the single timer rule for an overall timer", () => {
    renderBriefing({ timer_mode: "overall", duration_minutes: 90 });
    expect(within(howto()).getByText(/90 menit sejak kamu mulai/)).toBeInTheDocument();
    expect(within(howto()).queryByText(/tidak dibawa/)).not.toBeInTheDocument();
  });

  it("summarises the question and section counts", () => {
    renderBriefing({ question_count: 6, section_count: 2 });
    expect(within(howto()).getByText("6 soal · 2 section")).toBeInTheDocument();
  });

  it("places the token and check-in cards after the how-to card", () => {
    renderBriefing({ requires_checkin: true }, { afterHowTo: <div>Kartu token</div> });
    const h = screen.getByRole("heading", { name: "Cara mengerjakan" });
    expect(h.compareDocumentPosition(screen.getByText("Kartu token")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const checkin = screen.getByRole("heading", { name: /check-in/i });
    expect(h.compareDocumentPosition(checkin) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
