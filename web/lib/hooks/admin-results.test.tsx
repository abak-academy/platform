import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAdminResults,
  useAdminResultDetail,
  exportAdminResults,
  adminResultsKeys,
} from "./admin-results";
import type { AdminResultRow, AdminResultDetail } from "@/lib/types";

const mockAuthFetch = vi.fn();

vi.mock("@/lib/api", () => ({
  authFetch: (...args: Parameters<typeof mockAuthFetch>) =>
    mockAuthFetch(...args),
  API_BASE: "http://localhost:8080/api/v1",
  ApiError: class extends Error {
    code: string;
    status: number;
    constructor(code: string, message: string, status: number) {
      super(message);
      this.code = code;
      this.status = status;
    }
  },
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: {
    getState: () => ({ token: "test-token" }),
  },
}));

describe("admin-results hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("useAdminResults fetches GET /admin/results with exam_id", async () => {
    const rows: AdminResultRow[] = [
      {
        session_id: "s1",
        student_name: "Budi Santoso",
        username: "12345",
        score: 85,
        submitted_at: "2026-01-01T00:00:00Z",
      },
    ];
    const response = { data: rows, next_cursor: undefined };
    mockAuthFetch.mockResolvedValueOnce(response);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminResults({ examId: "exam-1" }), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/results?exam_id=exam-1");
    expect(result.current.data).toEqual(response);
  });

  it("useAdminResults omits q and cursor when unset", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined });

    const { wrapper } = wrapperFactory();
    renderHook(() => useAdminResults({ examId: "exam-1" }), { wrapper });

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/admin/results?exam_id=exam-1",
      ),
    );
  });

  it("useAdminResults passes q, cursor, limit as query params", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined });

    const { wrapper } = wrapperFactory();
    renderHook(
      () =>
        useAdminResults({
          examId: "exam-1",
          q: "budi",
          cursor: "cursor-1",
          limit: 10,
        }),
      { wrapper },
    );

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/admin/results?exam_id=exam-1&q=budi&cursor=cursor-1&limit=10",
      ),
    );
  });

  it("useAdminResults passes school_id when schoolId is provided", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined });

    const { wrapper } = wrapperFactory();
    renderHook(
      () =>
        useAdminResults({
          examId: "exam-1",
          schoolId: "school-1",
        }),
      { wrapper },
    );

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/admin/results?exam_id=exam-1&school_id=school-1",
      ),
    );
  });

  it("useAdminResults omits school_id when not provided", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined });

    const { wrapper } = wrapperFactory();
    renderHook(
      () => useAdminResults({ examId: "exam-1" }),
      { wrapper },
    );

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/admin/results?exam_id=exam-1",
      ),
    );
  });

  it("useAdminResultDetail fetches GET /admin/results/:session_id", async () => {
    const detail: AdminResultDetail = {
      session_id: "s1",
      student_name: "Budi Santoso",
      username: "12345",
      score: 85,
      submitted_at: "2026-01-01T00:00:00Z",
      result_config: "score_only",
      correct_count: 10,
      wrong_count: 2,
      empty_count: 1,
    };
    mockAuthFetch.mockResolvedValueOnce(detail);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminResultDetail("s1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/results/s1");
    expect(result.current.data).toEqual(detail);
  });

  it("useAdminResultDetail is disabled when sessionId is empty", async () => {
    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminResultDetail(""), { wrapper });

    expect(result.current.isFetching).toBe(false);
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("useAdminResultDetail threads school_id when schoolId is provided", async () => {
    mockAuthFetch.mockResolvedValueOnce({} as AdminResultDetail);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(
      () => useAdminResultDetail("s1", "school-1"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/results/s1?school_id=school-1",
    );
  });
});

describe("exportAdminResults", () => {
  beforeEach(() => {
    URL.createObjectURL = vi.fn().mockReturnValue("blob:test-url");
    URL.revokeObjectURL = vi.fn();
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("fetches CSV with Authorization header", async () => {
    const mockBlob = new Blob(["name,nis,score\n"], { type: "text/csv" });
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    await exportAdminResults("exam-1");

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/admin/results/export?exam_id=exam-1",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }),
      }),
    );
    expect(URL.createObjectURL).toHaveBeenCalledWith(mockBlob);
  });

  it("does not throw on non-JSON (blob) success response", async () => {
    const mockBlob = new Blob(["data"], { type: "text/csv" });
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    await expect(exportAdminResults("exam-1")).resolves.toBeUndefined();
  });

  it("exportAdminResults passes school_id param when provided", async () => {
    const mockBlob = new Blob(["name,score\n"], { type: "text/csv" });
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    await exportAdminResults("exam-1", "school-1");

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/admin/results/export?exam_id=exam-1&school_id=school-1",
      expect.anything(),
    );
  });

  it("exportAdminResults passes q param when provided", async () => {
    const mockBlob = new Blob(["name,score\n"], { type: "text/csv" });
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    await exportAdminResults("exam-1", "school-1", "budi");

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/admin/results/export?exam_id=exam-1&school_id=school-1&q=budi",
      expect.anything(),
    );
  });

  it("exportAdminResults throws on failed response", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 403,
      blob: () => Promise.resolve(new Blob(["forbidden"])),
    });

    await expect(exportAdminResults("exam-1")).rejects.toThrow("Export failed (403)");
  });

  it("exportAdminResults omits school_id when not provided", async () => {
    const mockBlob = new Blob(["name,score\n"], { type: "text/csv" });
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    await exportAdminResults("exam-1");

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/admin/results/export?exam_id=exam-1",
      expect.anything(),
    );
  });
});

function wrapperFactory() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
    queryClient,
  };
}
