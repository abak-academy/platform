import { describe, expect, it, vi } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useSearchStudentsAcrossSchools,
  usePresignExamGrantBulkUpload,
  useEnqueueExamGrantBulk,
} from "./admin-exam-grants";

const mockAuthFetch = vi.fn();

vi.mock("@/lib/api", () => ({
  authFetch: (...args: Parameters<typeof mockAuthFetch>) => mockAuthFetch(...args),
}));

it("passes exam context and pagination to cross-school search", async () => {
  mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );

  renderHook(
    () =>
      useSearchStudentsAcrossSchools({
        examId: "exam-1",
        cursor: "cursor-1",
        limit: 20,
        q: "budi",
      }),
    { wrapper },
  );

  await waitFor(() =>
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/exam-grants/students/search?q=budi&exam_id=exam-1&cursor=cursor-1&limit=20",
    ),
  );
});

function wrapperFactory() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("usePresignExamGrantBulkUpload", () => {
  it("posts to /admin/exam-grants/bulk/presign with exam_id, filename and content_type", async () => {
    const presignResp = {
      url: "http://minio.local/exam-grant-bulk/exam-1/uuid-x.csv?sig=abc",
      method: "PUT",
      key: "exam-grant-bulk/exam-1/uuid-x.csv",
    };
    mockAuthFetch.mockResolvedValueOnce(presignResp);

    const { result } = renderHook(() => usePresignExamGrantBulkUpload("exam-1"), {
      wrapper: wrapperFactory(),
    });

    let returned: typeof presignResp | undefined;
    await act(async () => {
      returned = await result.current.mutateAsync({
        filename: "grants.csv",
        contentType: "text/csv",
      });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/exam-grants/bulk/presign?exam_id=exam-1&filename=grants.csv&content_type=text%2Fcsv",
      { method: "POST" },
    );
    expect(returned).toEqual(presignResp);
  });
});

describe("useEnqueueExamGrantBulk", () => {
  it("posts {exam_id, file_key} to /admin/exam-grants/bulk and returns the job", async () => {
    mockAuthFetch.mockResolvedValueOnce({ job_id: "job-42" });

    const { result } = renderHook(() => useEnqueueExamGrantBulk(), {
      wrapper: wrapperFactory(),
    });

    let returned: { job_id: string } | undefined;
    await act(async () => {
      returned = await result.current.mutateAsync({
        examId: "exam-1",
        fileKey: "exam-grant-bulk/exam-1/uuid-x.csv",
      });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exam-grants/bulk", {
      method: "POST",
      body: JSON.stringify({
        exam_id: "exam-1",
        file_key: "exam-grant-bulk/exam-1/uuid-x.csv",
      }),
    });
    expect(returned).toEqual({ job_id: "job-42" });
  });
});
