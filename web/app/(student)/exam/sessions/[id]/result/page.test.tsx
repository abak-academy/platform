import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

import SessionResultPage from "./page";
import type { SessionResult } from "@/lib/types";

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "session-1" }),
}));

let uiStore = { lang: "id" as "id" | "en" };

vi.mock("@/stores/ui", () => ({
  useUIStore: (selector: (s: typeof uiStore) => unknown) => selector(uiStore),
}));

let resultState = {
  data: null as SessionResult | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

vi.mock("@/lib/hooks/exam", () => ({
  useSessionResult: () => resultState,
}));

describe("SessionResultPage", () => {
  beforeEach(() => {
    uiStore = { lang: "id" };
    resultState = {
      data: null,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
  });

  it("shows loading skeleton while fetching (FR-S5-26)", () => {
    render(<SessionResultPage />);
    expect(screen.getByText("Memuat…")).toBeInTheDocument();
  });

  it("shows error card with retry when the fetch fails (FR-S5-26)", () => {
    resultState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("not found"),
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByText(/gagal memuat data/i)).toBeInTheDocument();
  });

  it("renders hidden state (FR-S5-27)", () => {
    resultState = {
      data: { state: "hidden" },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByText("Hasil belum dipublikasikan.")).toBeInTheDocument();
  });

  it("renders grading state (FR-S5-27)", () => {
    resultState = {
      data: { state: "grading" },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByText("Sedang dinilai")).toBeInTheDocument();
  });

  it("renders locked state with formatted result_release_at (FR-S5-27)", () => {
    resultState = {
      data: {
        state: "locked",
        result_release_at: "2026-08-01T10:00:00Z",
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByText(/Hasil tersedia pada/)).toBeInTheDocument();
    expect(screen.getByText(/2026/)).toBeInTheDocument();
  });

  // ── Certificate link behaviours (FR-8 / resolved-discrepancy §2) ──────────

  it("shows certificate link on non-result state when certificate_url present (FR-8)", () => {
    resultState = {
      data: {
        state: "locked",
        result_release_at: "2026-08-01T10:00:00Z",
        certificate_url: "https://cdn/cert.pdf",
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByRole("link", { name: "Sertifikat" })).toBeInTheDocument();
  });

  it("hides certificate link on non-result state when certificate_url absent", () => {
    resultState = {
      data: {
        state: "locked",
        result_release_at: "2026-08-01T10:00:00Z",
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.queryByRole("link", { name: "Sertifikat" })).not.toBeInTheDocument();
  });

  // ── Leaderboard link (result state only) ──────────────────────────────────

  it("hides leaderboard link on non-result state", () => {
    resultState = {
      data: {
        state: "locked",
        result_release_at: "2026-08-01T10:00:00Z",
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.queryByRole("link", { name: "Peringkat" })).not.toBeInTheDocument();
  });

  it("renders leaderboard link on result state", () => {
    resultState = {
      data: {
        state: "result",
        result_config: "score_only",
        score: 80,
        correct_count: 8,
        wrong_count: 1,
        empty_count: 1,
        rank: 3,
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByRole("link", { name: "Peringkat" })).toBeInTheDocument();
  });

  // ── Result state (pre-existing, adjusted for certificate URL) ──────────

  it("renders score card for score_only result without breakdown and with certificate link (FR-S5-26)", () => {
    resultState = {
      data: {
        state: "result",
        result_config: "score_only",
        score: 80,
        correct_count: 8,
        wrong_count: 1,
        empty_count: 1,
        rank: 3,
        certificate_url: "https://cdn/cert.pdf",
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByText("80")).toBeInTheDocument();
    expect(screen.getByText("#3")).toBeInTheDocument();
    expect(screen.queryByText("Berdasarkan Topik")).not.toBeInTheDocument();
    expect(screen.queryByText("Pembahasan")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Sertifikat" })).toBeInTheDocument();
  });

  // ── Section-type labels (FR-29) ─────────────────────────────────────────

  it("renders section_type label for IELTS listening breakdown row (FR-29)", () => {
    resultState = {
      data: {
        state: "result",
        result_config: "score_pembahasan",
        score: 85,
        correct_count: 17,
        wrong_count: 2,
        empty_count: 1,
        rank: 5,
        breakdown: [
          {
            test_id: "test-1",
            title: "Section 1",
            subject: "English",
            topic: "IELTS Listening",
            section_type: "listening",
            earned: 15,
            max: 20,
          },
        ],
        pembahasan: [],
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByText("Listening")).toBeInTheDocument();
    expect(screen.getByText("15/20")).toBeInTheDocument();
  });

  it("renders section_type label for IELTS reading/breaking row (FR-29)", () => {
    resultState = {
      data: {
        state: "result",
        result_config: "score_pembahasan",
        score: 75,
        correct_count: 15,
        wrong_count: 4,
        empty_count: 1,
        rank: 8,
        breakdown: [
          {
            test_id: "test-2",
            title: "Section 2",
            subject: "English",
            topic: "IELTS Reading",
            section_type: "reading",
            earned: 18,
            max: 25,
          },
        ],
        pembahasan: [],
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByText("Reading")).toBeInTheDocument();
    expect(screen.getByText("18/25")).toBeInTheDocument();
  });

  it("renders section_type label for IELTS writing breakdown row (FR-29)", () => {
    resultState = {
      data: {
        state: "result",
        result_config: "score_pembahasan",
        score: 65,
        correct_count: 13,
        wrong_count: 5,
        empty_count: 2,
        rank: 10,
        breakdown: [
          {
            test_id: "test-3",
            title: "Section 3",
            subject: "English",
            topic: "IELTS Writing",
            section_type: "writing",
            earned: 10,
            max: 15,
          },
        ],
        pembahasan: [],
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByText("Writing")).toBeInTheDocument();
    expect(screen.getByText("10/15")).toBeInTheDocument();
  });

  it("standard/utbk result renders section titles with raw scores and no band/scaled number (FR-29)", () => {
    resultState = {
      data: {
        state: "result",
        result_config: "score_pembahasan",
        score: 90,
        correct_count: 9,
        wrong_count: 0,
        empty_count: 1,
        rank: 1,
        breakdown: [
          {
            test_id: "test-1",
            title: "Tes Matematika",
            subject: "Matematika",
            topic: "Aljabar",
            earned: 8,
            max: 10,
          },
        ],
        pembahasan: [
          {
            question_id: "q-1",
            body: "Berapa 2+2?",
            format: "mcq",
            your_answer: "4",
            correct_answer: "4",
            is_correct: true,
            explanation: "2+2 sama dengan 4.",
          },
        ],
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByText("Tes Matematika")).toBeInTheDocument();
    expect(screen.getByText("8/10")).toBeInTheDocument();
    // No section_type label (should not appear)
    expect(screen.queryByText("Listening")).not.toBeInTheDocument();
    expect(screen.queryByText("Reading")).not.toBeInTheDocument();
    expect(screen.queryByText("Writing")).not.toBeInTheDocument();
    // Raw score only — no band/IELTS number
    expect(screen.getByText("8/10")).toBeInTheDocument();
  });

  it("renders breakdown and pembahasan for score_pembahasan result (FR-S5-26)", () => {
    resultState = {
      data: {
        state: "result",
        result_config: "score_pembahasan",
        score: 90,
        correct_count: 9,
        wrong_count: 0,
        empty_count: 1,
        rank: 1,
        breakdown: [
          {
            test_id: "test-1",
            title: "Tes Matematika",
            subject: "Matematika",
            topic: "Aljabar",
            earned: 8,
            max: 10,
          },
        ],
        pembahasan: [
          {
            question_id: "q-1",
            body: "Berapa 2+2?",
            format: "mcq",
            your_answer: "4",
            correct_answer: "4",
            is_correct: true,
            explanation: "2+2 sama dengan 4.",
          },
        ],
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(screen.getByText("Berdasarkan Topik")).toBeInTheDocument();
    expect(screen.getByText("Tes Matematika")).toBeInTheDocument();
    expect(screen.getByText("8/10")).toBeInTheDocument();
    expect(screen.getByText("Pembahasan")).toBeInTheDocument();
    expect(screen.getByText(/Berapa 2\+2\?/)).toBeInTheDocument();
  });

  it("renders rich body in pembahasan via RichContent (LaTeX + bold HTML)", async () => {
    resultState = {
      data: {
        state: "result",
        result_config: "score_pembahasan",
        score: 90,
        correct_count: 9,
        wrong_count: 0,
        empty_count: 1,
        rank: 1,
        breakdown: [],
        pembahasan: [
          {
            question_id: "q-rich",
            body: "Hitung \\(x^2\\) dan buat <b>tebal</b>",
            format: "mcq",
            your_answer: "4",
            correct_answer: "4",
            is_correct: true,
            explanation: "Penjelasan polos \\(x^2\\). <b>tidak</b> dirich.",
          },
        ],
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    const richNode = await waitFor(() => {
      const el = document.querySelector("[data-rich-content] .katex");
      if (!el) throw new Error("not yet");
      return el.closest("[data-rich-content]") as HTMLElement;
    });
    expect(richNode).not.toBeNull();
    const b = richNode.querySelector("b");
    expect(b).not.toBeNull();
    expect(b?.textContent).toBe("tebal");
    // Literal LaTeX delimiters are replaced by KaTeX — not visible as text.
    expect(richNode.textContent).not.toContain("\\(");
    // Explanation remains plain text (no RichContent wrapper around it) — literal
    // delimiters and tag text are visible.
    expect(
      screen.getByText(/Penjelasan polos.*tidak.*dirich/)
    ).toBeInTheDocument();
  });

  it("renders a true_false pembahasan row without crashing on the new shape", () => {
    resultState = {
      data: {
        state: "result",
        result_config: "score_pembahasan",
        score: 90,
        correct_count: 9,
        wrong_count: 0,
        empty_count: 1,
        rank: 1,
        breakdown: [],
        pembahasan: [
          {
            question_id: "q-tf",
            body: "Which statements are true?",
            format: "true_false",
            your_answer: '["true","false","true",""]',
            correct_answer: null,
            is_correct: false,
            explanation: "Statement 2 is actually true.",
          },
        ],
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    render(<SessionResultPage />);
    expect(
      screen.getByText(/Which statements are true\?/)
    ).toBeInTheDocument();
    expect(
      screen.getByText('["true","false","true",""]')
    ).toBeInTheDocument();
  });
});
