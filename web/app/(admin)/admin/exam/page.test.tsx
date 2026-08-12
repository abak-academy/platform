import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import ExamDashboardPage from "./page";

const replace = vi.fn();
const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace, push }),
}));

let authStore: {
  token: string | null;
  user: { role?: string; name?: string } | null;
} = {
  token: "t",
  user: { role: "admin_exam", name: "Budi" },
};

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof authStore) => unknown) => selector(authStore),
}));

let meState: {
  data: { role?: string; name?: string } | null;
  isError: boolean;
  isLoading: boolean;
} = { data: null, isError: false, isLoading: false };

vi.mock("@/lib/hooks/auth", async () => {
  const actual = await vi.importActual<typeof import("@/lib/hooks/auth")>(
    "@/lib/hooks/auth"
  );
  return {
    ...actual,
    useMe: ({ enabled }: { enabled?: boolean }) =>
      enabled
        ? meState
        : { data: null, isError: false, isLoading: false },
  };
});

let dashboardState = {
  data: {
    active_sessions: 3,
    upcoming_exams: [
      { id: "e1", title: "TKA Matematika", scheduled_at: "2026-08-20T09:00:00+07:00", registrant_count: 42 },
    ],
    counts: { questions: 1500, tests: 40, exams: 12, courses: 8 },
    recent_violations: [
      {
        session_id: "s1",
        exam_id: "e1",
        exam_title: "TKA Matematika",
        student_name: "Budi",
        violation_type: "tab_switch",
        occurred_at: "2026-08-12T10:00:00+07:00",
      },
    ],
  },
  isLoading: false,
  isError: false,
  refetch: vi.fn(),
};

vi.mock("@/lib/hooks/admin-dashboard", () => ({
  useExamDashboard: () => dashboardState,
}));

describe("admin_exam dashboard", () => {
  beforeEach(() => {
    authStore = { token: "t", user: { role: "admin_exam", name: "Budi" } };
  });

  it("shows the live session count and the upcoming exams", () => {
    render(<ExamDashboardPage />);
    expect(screen.getByTestId("exam-active-sessions")).toHaveTextContent("3");
    expect(screen.getByText("TKA Matematika")).toBeTruthy();
    expect(screen.getByText(/42/)).toBeTruthy();
  });

  it("shows a courses panel — admin_exam manages courses too", () => {
    render(<ExamDashboardPage />);
    expect(screen.getByTestId("exam-courses")).toHaveTextContent("8");
  });

  it("shows the bank and catalogue counts", () => {
    render(<ExamDashboardPage />);
    expect(screen.getByTestId("exam-questions")).toHaveTextContent("1500");
    expect(screen.getByTestId("exam-tests")).toHaveTextContent("40");
    expect(screen.getByTestId("exam-exams")).toHaveTextContent("12");
  });

  it("lists recent violations", () => {
    render(<ExamDashboardPage />);
    // "Budi" is also the signed-in admin's name (hero) — scope to disambiguate.
    const panel = screen.getByText("Pelanggaran terbaru").closest(".md-card-outlined") as HTMLElement;
    expect(within(panel).getByText("Budi")).toBeTruthy();
  });

  it("links every monitor card to the page that owns it", () => {
    render(<ExamDashboardPage />);
    expect(screen.getByTestId("exam-questions-link")).toHaveAttribute("href", "/admin/exam/questions");
    expect(screen.getByTestId("exam-tests-link")).toHaveAttribute("href", "/admin/exam/tests");
    expect(screen.getByTestId("exam-courses-link")).toHaveAttribute("href", "/admin/courses");
  });

  it("renders no chart and no period control — that is the super-admin page's job", () => {
    const { container } = render(<ExamDashboardPage />);
    expect(container.querySelector("svg[role='img']")).toBeNull();
    expect(screen.queryByText(/periode/i)).toBeNull();
  });

  it("refuses a role without sessions capability", () => {
    authStore = { token: "t", user: { role: "admin_store", name: "Siti" } };
    render(<ExamDashboardPage />);
    expect(screen.getByTestId("no-access")).toBeTruthy();
  });

  it("offers a retry when the fetch fails instead of rendering zeros", () => {
    dashboardState = { ...dashboardState, data: undefined as never, isError: true };
    render(<ExamDashboardPage />);
    expect(screen.getByRole("button", { name: /muat ulang/i })).toBeTruthy();
  });
});
