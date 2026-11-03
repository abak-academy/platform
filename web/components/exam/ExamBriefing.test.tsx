import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ExamBriefing } from "./ExamBriefing";
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

function renderBriefing(over: Partial<RegistrationExam> = {}, props: { preview?: boolean } = {}) {
  return render(
    <ExamBriefing exam={{ ...exam, ...over }} backHref="/exam" backLabel="Kembali" gate={props.preview ? null : gate} preview={props.preview} />,
  );
}

const waLink = () => screen.queryByRole("link", { name: /gabung grup whatsapp/i });
const rulesHeading = () => screen.queryByRole("heading", { name: /aturan tambahan dari penyelenggara/i });

describe("ExamBriefing whatsapp invitation", () => {
  beforeEach(() => {
    uiStore = { lang: "id" };
    gate.onStart.mockReset();
  });

  it("sits after Informasi Ujian and before Cara mengerjakan", () => {
    renderBriefing({ whatsapp_group_url: WA });
    const info = screen.getByRole("heading", { name: "Informasi Ujian" });
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
