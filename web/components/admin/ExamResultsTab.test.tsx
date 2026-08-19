import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { ExamResultsTab } from "./ExamResultsTab";
import type { AdminResultRow, AdminResultDetail, School } from "@/lib/types";

const mockExport = vi.fn();

let resultsState = {
  data: null as { data: AdminResultRow[]; next_cursor?: string } | null,
  isLoading: false,
  isFetching: false,
  isError: false,
  error: null as Error | null,
};

let detailState = {
  data: null as AdminResultDetail | null,
  isLoading: false,
  isFetching: false,
  isError: false,
  error: null as Error | null,
};

let authStore: {
  token: string | null;
  user: { role?: string; school_id?: string; name?: string } | null;
} = {
  token: "t",
  user: { role: "admin_school", school_id: "s1" },
};

let schoolsState = {
  data: null as { data: School[]; next_cursor?: string } | null,
  isLoading: false,
  isError: false,
};

const mockUseAdminResults = vi.fn();

vi.mock("@/lib/hooks/admin-results", () => ({
  // Spread a fresh object each call so query.data changes reference like real
  // react-query does when the query key changes — the component's accumulate
  // effect keys off that reference to refill after a filter reset.
  useAdminResults: (opts: unknown) => {
    mockUseAdminResults(opts);
    return {
      ...resultsState,
      data: resultsState.data ? { ...resultsState.data } : null,
    };
  },
  useAdminResultDetail: () => ({ ...detailState }),
  exportAdminResults: (...args: Parameters<typeof mockExport>) => mockExport(...args),
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof authStore) => unknown) => selector(authStore),
}));

vi.mock("@/lib/hooks/admin-schools", () => ({
  useSchoolOptions: () => schoolsState,
}));

const sampleResultRows: AdminResultRow[] = [
  {
    session_id: "sess-1",
    student_name: "Budi Santoso",
    school_name: "SMAN 1 Jakarta",
    username: "12345",
    score: 85,
    submitted_at: "2026-01-15T00:00:00Z",
  },
  {
    session_id: "sess-2",
    student_name: "Siti Aisyah",
    school_name: "SMAN 2 Bandung",
    username: "67890",
    score: 92,
    submitted_at: "2026-02-20T00:00:00Z",
  },
];

const scorePembahasanDetail: AdminResultDetail = {
  session_id: "sess-1",
  student_name: "Budi Santoso",
  username: "12345",
  score: 85,
  submitted_at: "2026-01-15T00:00:00Z",
  result_config: "score_pembahasan",
  correct_count: 10,
  wrong_count: 2,
  empty_count: 1,
  breakdown: [
    { test_id: "t1", title: "Aljabar", subject: "Matematika", topic: "Aljabar", earned: 8, max: 10 },
  ],
};

describe("ExamResultsTab", () => {
  beforeEach(() => {
    authStore = { token: "t", user: { role: "admin_school", school_id: "s1" } };
    schoolsState = {
      data: { data: [{ id: "s1", name: "SMAN 1 Jakarta" }, { id: "s2", name: "SMAN 2 Bandung" }], next_cursor: undefined },
      isLoading: false,
      isError: false,
    };
    resultsState = {
      data: { data: sampleResultRows, next_cursor: undefined },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };
    detailState = { data: null, isLoading: false, isFetching: false, isError: false, error: null };
    mockExport.mockReset();
    mockUseAdminResults.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders rows with student name, per-row school, score and submitted date", async () => {
    render(<ExamResultsTab examId="exam-1" />);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
      expect(screen.getByText("Siti Aisyah")).toBeInTheDocument();
    });

    // Two rows from different schools must show their own school_name, not a
    // single picked/own school name repeated for every row (FB defect).
    expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument();
    expect(screen.getByText("SMAN 2 Bandung")).toBeInTheDocument();
    expect(screen.getByText("85")).toBeInTheDocument();
    expect(screen.getByText("92")).toBeInTheDocument();
  });

  it("clicking a row opens the detail dialog with the per-topic breakdown", async () => {
    detailState = {
      data: scorePembahasanDetail,
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    };

    render(<ExamResultsTab examId="exam-1" />);

    const row = await screen.findByText("Budi Santoso");
    fireEvent.click(row);

    await waitFor(() => {
      expect(screen.getByText("Aljabar")).toBeInTheDocument();
      expect(screen.getByText("8/10")).toBeInTheDocument();
    });
  });

  it("fetches and renders results for super_admin with no school picked (defaults to all schools)", async () => {
    authStore = { token: "t", user: { role: "super_admin" } };

    render(<ExamResultsTab examId="exam-1" />);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
      expect(screen.getByText("Siti Aisyah")).toBeInTheDocument();
    });
    expect(screen.getByRole("combobox")).toBeInTheDocument();

    // No school_id filter is sent when nothing has been picked.
    const lastCall = mockUseAdminResults.mock.calls.at(-1)?.[0];
    expect(lastCall.schoolId).toBeUndefined();
  });

  it("narrows results by school_id once a super_admin picks one from the optional filter", async () => {
    authStore = { token: "t", user: { role: "super_admin" } };

    render(<ExamResultsTab examId="exam-1" />);
    await screen.findByText("Budi Santoso");

    const selectTrigger = screen.getByRole("combobox");
    fireEvent.click(selectTrigger);

    // Table rows already show "SMAN 2 Bandung" as a per-row school_name, so
    // disambiguate the dropdown item by its option role.
    const schoolOption = await screen.findByRole("option", { name: "SMAN 2 Bandung" });
    fireEvent.click(schoolOption);

    await waitFor(() => {
      const lastCall = mockUseAdminResults.mock.calls.at(-1)?.[0];
      expect(lastCall.schoolId).toBe("s2");
    });
  });
});
