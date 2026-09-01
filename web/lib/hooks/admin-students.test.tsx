import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAdminStudents,
  useRegisterStudent,
  useChangeStudentStatus,
  useReissueStudentCredentials,
  useSetStudentPassword,
  adminStudentsKeys,
} from "./admin-students";
import type {
  AdminStudent,
  StudentRegistrationInput,
  StudentRegistrationResult,
  StudentCredentials,
} from "@/lib/types";

const mockAuthFetch = vi.fn();

vi.mock("@/lib/api", () => ({
  authFetch: (...args: Parameters<typeof mockAuthFetch>) =>
    mockAuthFetch(...args),
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

describe("admin-students hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("useAdminStudents fetches GET /admin/students and returns paginated response", async () => {
    const students: AdminStudent[] = [
      {
        id: "st1",
        name: "Budi Santoso",
        username: "budi",
        jenjang: "SMA",
        status: "active",
        created_at: "2026-01-01T00:00:00Z",
      },
    ];
    const response = { data: students, next_cursor: undefined };
    mockAuthFetch.mockResolvedValueOnce(response);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminStudents(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/students");
    expect(result.current.data).toEqual(response);
  });

  it("useAdminStudents passes status, q, cursor, limit as query params", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined });

    const { wrapper } = wrapperFactory();
    renderHook(
      () =>
        useAdminStudents({
          status: "active",
          q: "budi",
          cursor: "cursor-1",
          limit: 10,
        }),
      { wrapper },
    );

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/admin/students?status=active&q=budi&cursor=cursor-1&limit=10",
      ),
    );
  });

  it("useAdminStudents passes picker eligibility and grade filters", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined });

    const { wrapper } = wrapperFactory();
    renderHook(
      () =>
        useAdminStudents({
          examId: "exam-1",
          jenjang: "SMA",
          grade: "11",
          limit: 20,
        }),
      { wrapper },
    );

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/admin/students?limit=20&exam_id=exam-1&jenjang=SMA&grade=11",
      ),
    );
  });

  it("useAdminStudents passes school_id when schoolId is provided", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined });

    const { wrapper } = wrapperFactory();
    renderHook(
      () =>
        useAdminStudents({
          schoolId: "school-1",
        }),
      { wrapper },
    );

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/admin/students?school_id=school-1",
      ),
    );
  });

  it("useAdminStudents omits school_id when schoolId is undefined", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined });

    const { wrapper } = wrapperFactory();
    renderHook(() => useAdminStudents(), { wrapper });

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith("/admin/students"),
    );
  });

  it("useRegisterStudent posts to /admin/students, threads school_id, and invalidates list", async () => {
    const input: StudentRegistrationInput = {
      name: "Siti Aisyah",
      jenjang: "SMA",
      email: "siti@test.com",
    };
    const result: StudentRegistrationResult = {
      id: "st2",
      name: "Siti Aisyah",
      username: "siti",
      jenjang: "SMA",
      status: "active",
      created_at: "2026-02-01T00:00:00Z",
      temp_password: "temp123",
    };
    mockAuthFetch.mockResolvedValueOnce(result);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result: hookResult } = renderHook(() => useRegisterStudent(), {
      wrapper,
    });

    await act(async () => {
      await hookResult.current.mutateAsync({ input, schoolId: "school-1" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/students?school_id=school-1", {
      method: "POST",
      body: JSON.stringify(input),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminStudentsKeys.all });
  });

  it("useRegisterStudent omits school_id when schoolId is undefined", async () => {
    const input: StudentRegistrationInput = { name: "X", jenjang: "SMA" };
    mockAuthFetch.mockResolvedValueOnce({} as StudentRegistrationResult);

    const { wrapper } = wrapperFactory();
    const { result: hookResult } = renderHook(() => useRegisterStudent(), {
      wrapper,
    });

    await act(async () => {
      await hookResult.current.mutateAsync({ input });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/students", {
      method: "POST",
      body: JSON.stringify(input),
    });
  });

  it("useChangeStudentStatus patches /admin/students/:id, threads school_id, and invalidates list", async () => {
    mockAuthFetch.mockResolvedValueOnce({ message: "status updated" });

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useChangeStudentStatus(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        id: "st1",
        status: "deactivated",
        schoolId: "school-1",
      });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/students/st1?school_id=school-1", {
      method: "PATCH",
      body: JSON.stringify({ status: "deactivated" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminStudentsKeys.all });
  });

  it("useReissueStudentCredentials fetches /admin/students/:id/credentials, threads school_id, and invalidates list", async () => {
    const creds: StudentCredentials = {
      username: "budi",
      temp_password: "newPass789",
    };
    mockAuthFetch.mockResolvedValueOnce(creds);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useReissueStudentCredentials(), {
      wrapper,
    });

    await act(async () => {
      await result.current.mutateAsync({ id: "st1", schoolId: "school-1" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/students/st1/credentials?school_id=school-1",
    );
    expect(spy).toHaveBeenCalledWith({ queryKey: adminStudentsKeys.all });
  });

  it("useReissueStudentCredentials omits school_id when schoolId is undefined", async () => {
    mockAuthFetch.mockResolvedValueOnce({} as StudentCredentials);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useReissueStudentCredentials(), {
      wrapper,
    });

    await act(async () => {
      await result.current.mutateAsync({ id: "st1" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/students/st1/credentials",
    );
  });

  it("useSetStudentPassword patches only new_password and invalidates list", async () => {
    mockAuthFetch.mockResolvedValueOnce({ message: "password updated" });

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useSetStudentPassword(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        id: "st1",
        newPassword: "chosenPass123",
      });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/students/st1/password", {
      method: "PATCH",
      body: JSON.stringify({ new_password: "chosenPass123" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminStudentsKeys.all });
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
