import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import SchoolDashboardPage from "./page";
import type { SchoolDashboard } from "@/lib/types";

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
  user: { role: "admin_school", name: "Siti" },
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

let dashboardState: {
  data: SchoolDashboard;
  isLoading: boolean;
  isError: boolean;
  refetch: () => void;
} = {
  data: {
    counts: { students: 320, new_students_month: 14 },
    orderable_exam_count: 5,
    latest_bulk_order: {
      id: "o1",
      status: "paid",
      total: 1500000,
      participant_count: 30,
      placed_at: "2026-08-10T09:00:00+07:00",
    },
    recent_results: [
      {
        session_id: "s1",
        student_name: "Andi",
        exam_title: "TKA Matematika",
        score: 88.5,
        submitted_at: "2026-08-11T11:00:00+07:00",
      },
    ],
  },
  isLoading: false,
  isError: false,
  refetch: vi.fn(),
};

vi.mock("@/lib/hooks/admin-dashboard", () => ({
  useSchoolDashboard: () => dashboardState,
}));

describe("admin_school dashboard", () => {
  beforeEach(() => {
    authStore = { token: "t", user: { role: "admin_school", name: "Siti" } };
  });

  it("shows the student head-count and this month's intake", () => {
    render(<SchoolDashboardPage />);
    expect(screen.getByTestId("school-students")).toHaveTextContent("320");
    expect(screen.getByTestId("school-new-students")).toHaveTextContent("14");
  });

  it("shows how many exams can be ordered", () => {
    render(<SchoolDashboardPage />);
    expect(screen.getByTestId("school-orderable-exams")).toHaveTextContent("5");
  });

  it("shows the latest bulk order with its participant count", () => {
    render(<SchoolDashboardPage />);
    const panel = screen.getByTestId("school-latest-bulk-order");
    expect(panel).toHaveTextContent("30");
  });

  it("shows an empty state, not a fake order, when none was ever placed", () => {
    dashboardState = { ...dashboardState, data: { ...dashboardState.data, latest_bulk_order: null } };
    render(<SchoolDashboardPage />);
    expect(screen.getByTestId("school-latest-bulk-order")).toHaveTextContent(/belum ada/i);
  });

  it("lists recent results and leaves an ungraded score blank rather than zero", () => {
    dashboardState = {
      ...dashboardState,
      data: {
        ...dashboardState.data,
        recent_results: [
          { session_id: "s2", student_name: "Budi", exam_title: "TKA IPA", score: null, submitted_at: "2026-08-11T11:00:00+07:00" },
        ],
      },
    };
    render(<SchoolDashboardPage />);
    // A submitted-but-ungraded essay session must not read as a score of 0.
    expect(screen.getByTestId("school-result-s2")).not.toHaveTextContent("0");
  });

  it("shows no orders panel and no session panel — neither is in this role's scope", () => {
    render(<SchoolDashboardPage />);
    expect(screen.queryByTestId("school-order-queue")).toBeNull();
    expect(screen.queryByTestId("school-active-sessions")).toBeNull();
  });

  it("refuses a role without students capability", () => {
    authStore = { token: "t", user: { role: "admin_store", name: "Budi" } };
    render(<SchoolDashboardPage />);
    expect(screen.getByTestId("no-access")).toBeTruthy();
  });
});
