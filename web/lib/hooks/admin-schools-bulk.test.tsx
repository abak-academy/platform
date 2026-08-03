import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  usePresignSchoolBulkUpload,
  useEnqueueSchoolBulkImport,
} from "./admin-schools-bulk";

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

describe("admin-schools-bulk hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
    vi.restoreAllMocks();
  });

  describe("usePresignSchoolBulkUpload", () => {
    it("posts to /admin/schools/bulk/presign with filename and content_type", async () => {
      const presignResp = {
        url: "http://minio.local/school-bulk/uuid-x.csv?sig=abc",
        method: "PUT",
        key: "school-bulk/uuid-x.csv",
      };
      mockAuthFetch.mockResolvedValueOnce(presignResp);

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => usePresignSchoolBulkUpload(), {
        wrapper,
      });

      let returned: { url: string; method: string; key: string } | undefined;
      await act(async () => {
        returned = await result.current.mutateAsync({
          filename: "schools.csv",
          contentType: "text/csv",
        });
      });

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/admin/schools/bulk/presign?filename=schools.csv&content_type=text%2Fcsv",
        { method: "POST" },
      );
      expect(returned).toEqual(presignResp);
    });
  });

  describe("useEnqueueSchoolBulkImport", () => {
    it("posts {file_key} to /admin/schools/bulk and returns job_id", async () => {
      mockAuthFetch.mockResolvedValueOnce({ job_id: "job-42" });

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useEnqueueSchoolBulkImport(), {
        wrapper,
      });

      let returned: { job_id: string } | undefined;
      await act(async () => {
        returned = await result.current.mutateAsync({
          fileKey: "school-bulk/uuid-x.csv",
        });
      });

      expect(mockAuthFetch).toHaveBeenCalledWith("/admin/schools/bulk", {
        method: "POST",
        body: JSON.stringify({ file_key: "school-bulk/uuid-x.csv" }),
      });
      expect(returned).toEqual({ job_id: "job-42" });
    });
  });
});
