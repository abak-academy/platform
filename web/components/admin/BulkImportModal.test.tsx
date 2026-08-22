import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BulkImportModal } from "./BulkImportModal";

// ── Mutable mock state for the three bulk-upload hooks ──

const presignMutateAsync = vi.fn();
const enqueueMutateAsync = vi.fn();
const putFile = vi.fn();

const jobStatusState: {
  data: { id: string; type: string; status: string; progress: number; result_url: string | null; error: string | null; created_at: string; updated_at: string } | null;
} = {
  data: null,
};

let pollTick = 0;

vi.mock("@/lib/hooks/admin-students-bulk", () => ({
  usePresignStudentBulkUpload: () => ({
    mutateAsync: presignMutateAsync,
    isPending: false,
  }),
  putFileToPresignedURL: (...args: Parameters<typeof putFile>) => putFile(...args),
  useEnqueueStudentBulkImport: () => ({
    mutateAsync: enqueueMutateAsync,
    isPending: false,
  }),
}));

vi.mock("@/lib/hooks/jobs", () => ({
  useJobStatus: () => {
    void pollTick;
    return {
      data: jobStatusState.data,
      isLoading: false,
      isError: false,
      error: null,
    };
  },
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({
    lang: "id",
    t: (key: string) => key,
  }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

let lastDownloadedFilename: string | null = null;
let lastDownloadedCSV: string | null = null;
let lastCapturedBlob: Blob | null = null;
const originalCreateElement = document.createElement.bind(document);

function wrapperFactory() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("BulkImportModal", () => {
  beforeEach(() => {
    presignMutateAsync.mockReset();
    enqueueMutateAsync.mockReset();
    putFile.mockReset();
    lastDownloadedFilename = null;
    lastDownloadedCSV = null;
    lastCapturedBlob = null;
    jobStatusState.data = null;
    pollTick = 0;

    document.createElement = ((tag: string) => {
      const el = originalCreateElement(tag);
      if (tag === "a") {
        (el as HTMLAnchorElement).click = vi.fn(function (this: HTMLAnchorElement) {
          lastDownloadedFilename = (this as HTMLAnchorElement).download;
          if (lastCapturedBlob) {
            lastCapturedBlob.text().then((t) => {
              lastDownloadedCSV = t;
            });
          }
        });
      }
      return el;
    }) as typeof document.createElement;

    if (!(URL.createObjectURL as any).__mocked) {
      URL.createObjectURL = vi.fn().mockImplementation((blob: Blob) => {
        lastCapturedBlob = blob;
        return "blob:mock" as unknown as string;
      }) as typeof URL.createObjectURL;
      (URL.createObjectURL as any).__mocked = true;
    }
  });

  it("renders nothing when closed", () => {
    render(<BulkImportModal open={false} onOpenChange={vi.fn()} />, {
      wrapper: wrapperFactory(),
    });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders the dialog title and a Download Template button when open", () => {
    render(<BulkImportModal open={true} onOpenChange={vi.fn()} />, {
      wrapper: wrapperFactory(),
    });
    expect(screen.getByText("bulk_register_title")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /bulk_register_download_template/i }),
    ).toBeInTheDocument();
  });

	it("clicking Download Template produces a CSV with the exact header and two example rows, firing no network request", async () => {
    render(<BulkImportModal open={true} onOpenChange={vi.fn()} />, {
      wrapper: wrapperFactory(),
    });

    const downloadBtn = screen.getByRole("button", { name: /bulk_register_download_template/i });
    fireEvent.click(downloadBtn);

    await waitFor(() => expect(lastDownloadedFilename).not.toBeNull());
    await waitFor(() => expect(lastDownloadedCSV).not.toBeNull());

    expect(lastDownloadedFilename).toBe("bulk_register_template.csv");
    expect(lastDownloadedCSV).toBe(
      "name,school,jenjang,email,dob,gender,grade,target_exam,alamat_domisili,provinsi,kota,kecamatan,kode_pos\n" +
        'Budi Santoso,SMAN 1 Jakarta,SMA,budi@example.com,2008-05-14,male,11,UTBK,"Jl. Melati No. 3, RT 04",JAWA BARAT,KOTA BANDUNG,COBLONG,40132\n' +
        "Siti Aminah,SMAN 1 Jakarta,SMA,,,,,,,,,,\n",
    );

    expect(presignMutateAsync).not.toHaveBeenCalled();
    expect(putFile).not.toHaveBeenCalled();
    expect(enqueueMutateAsync).not.toHaveBeenCalled();
  });

  it("shows the field-rule table and downloads a guide txt", async () => {
    render(<BulkImportModal open={true} onOpenChange={vi.fn()} />, {
      wrapper: wrapperFactory(),
    });

    expect(screen.getByText("bulk_format_show")).toBeInTheDocument();
    expect(screen.getByText("jenjang")).toBeInTheDocument();
    expect(screen.getByText("bulk_format_student_jenjang")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /bulk_format_download_guide/i }));

    await waitFor(() => expect(lastDownloadedFilename).toBe("bulk_register_guide.txt"));
    await waitFor(() => expect(lastDownloadedCSV).not.toBeNull());
    expect(lastDownloadedCSV).toContain("bulk_format_student_guide_title");
    expect(lastDownloadedCSV).toContain("bulk_format_student_school");
  });

  it("uploading a valid CSV runs presign -> PUT -> enqueue in order", async () => {
    presignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/k?sig=xyz",
      method: "PUT",
      key: "student-bulk/school-1/uuid.csv",
    });
    enqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-1" });
    putFile.mockResolvedValueOnce(undefined);

    render(<BulkImportModal open={true} onOpenChange={vi.fn()} />, {
      wrapper: wrapperFactory(),
    });

    const fileInput = screen.getByLabelText(/choose_file|file/i) as HTMLInputElement;
    const file = new File(["name,school,jenjang,provinsi,kota,kecamatan,kode_pos,email\nBudi,SMAN 1,sma,JB,Bandung,Coblong,40132,budi@example.com"], "students.csv", { type: "text/csv" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    const submitBtn = screen.getByRole("button", { name: /upload|import|submit|start/i });
    fireEvent.click(submitBtn);

    await waitFor(() => expect(presignMutateAsync).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(putFile).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(enqueueMutateAsync).toHaveBeenCalledTimes(1));

    expect(presignMutateAsync.mock.invocationCallOrder[0]).toBeLessThan(
      putFile.mock.invocationCallOrder[0],
    );
    expect(putFile.mock.invocationCallOrder[0]).toBeLessThan(
      enqueueMutateAsync.mock.invocationCallOrder[0],
    );

    expect(enqueueMutateAsync).toHaveBeenCalledWith({
      fileKey: "student-bulk/school-1/uuid.csv",
    });
  });

  it("shows terminal error when the job fails", async () => {
    presignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/k?sig=xyz",
      method: "PUT",
      key: "k1",
    });
    enqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-bad" });
    putFile.mockResolvedValueOnce(undefined);

    const { rerender } = render(
      <BulkImportModal open={true} onOpenChange={vi.fn()} />,
      { wrapper: wrapperFactory() },
    );

    const fileInput = screen.getByLabelText(/choose_file|file/i) as HTMLInputElement;
    const file = new File(["x"], "x.csv", { type: "text/csv" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    const submitBtn = screen.getByRole("button", { name: /upload|import|submit|start/i });
    fireEvent.click(submitBtn);

    await waitFor(() => expect(enqueueMutateAsync).toHaveBeenCalled());

    jobStatusState.data = {
      id: "job-bad",
      type: "student_bulk",
      status: "failed",
      progress: 0,
      result_url: null,
      error: "unresolvable school name 'SMAN 999'",
      created_at: "2026-07-18T00:00:00Z",
      updated_at: "2026-07-18T00:01:00Z",
    };
    pollTick++;
    rerender(<BulkImportModal open={true} onOpenChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText(/unresolvable school name/i)).toBeInTheDocument();
    });
  });

  it("shows a download link to the result URL on terminal success", async () => {
    presignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/k?sig=xyz",
      method: "PUT",
      key: "k2",
    });
    enqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-ok" });
    putFile.mockResolvedValueOnce(undefined);

    const { rerender } = render(
      <BulkImportModal open={true} onOpenChange={vi.fn()} />,
      { wrapper: wrapperFactory() },
    );

    const fileInput = screen.getByLabelText(/choose_file|file/i) as HTMLInputElement;
    const file = new File(["x"], "x.csv", { type: "text/csv" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    const submitBtn = screen.getByRole("button", { name: /upload|import|submit|start/i });
    fireEvent.click(submitBtn);

    await waitFor(() => expect(enqueueMutateAsync).toHaveBeenCalled());

    jobStatusState.data = {
      id: "job-ok",
      type: "student_bulk",
      status: "succeeded",
      progress: 100,
      result_url: "http://minio.local/result.csv?sig=abc",
      error: null,
      created_at: "2026-07-18T00:00:00Z",
      updated_at: "2026-07-18T00:02:00Z",
    };
    pollTick++;
    rerender(<BulkImportModal open={true} onOpenChange={vi.fn()} />);

    await waitFor(() => {
      const link = screen.getByRole("link");
      expect((link as HTMLAnchorElement).href).toBe(
        "http://minio.local/result.csv?sig=abc",
      );
    });
  });

  // The worker marks a job "failed" when every row fails, but it still uploads
  // and stores the per-row report — which is exactly when the operator needs it.
  it("shows the download link on a failed job that still produced a result URL", async () => {
    presignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/k?sig=xyz",
      method: "PUT",
      key: "k2",
    });
    enqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-allfail" });
    putFile.mockResolvedValueOnce(undefined);

    const { rerender } = render(
      <BulkImportModal open={true} onOpenChange={vi.fn()} />,
      { wrapper: wrapperFactory() },
    );

    const fileInput = screen.getByLabelText(/choose_file|file/i) as HTMLInputElement;
    const file = new File(["x"], "x.csv", { type: "text/csv" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    const submitBtn = screen.getByRole("button", { name: /upload|import|submit|start/i });
    fireEvent.click(submitBtn);

    await waitFor(() => expect(enqueueMutateAsync).toHaveBeenCalled());

    jobStatusState.data = {
      id: "job-allfail",
      type: "student_bulk",
      status: "failed",
      progress: 100,
      result_url: "http://minio.local/allfail.csv?sig=def",
      error: "student_bulk job job-allfail: all 3 rows failed",
      created_at: "2026-07-18T00:00:00Z",
      updated_at: "2026-07-18T00:02:00Z",
    };
    pollTick++;
    rerender(<BulkImportModal open={true} onOpenChange={vi.fn()} />);

    await waitFor(() => {
      const link = screen.getByRole("link");
      expect((link as HTMLAnchorElement).href).toBe(
        "http://minio.local/allfail.csv?sig=def",
      );
    });
    // The generic failure message must still be shown alongside the report.
    expect(screen.getByText(/all 3 rows failed/)).toBeInTheDocument();
  });

  it("does not import any direct student-CSV service path (only HTTP hooks)", async () => {
    const fs = await import("fs");
    const path = await import("path");
    const src = fs.readFileSync(path.join(__dirname, "BulkImportModal.tsx"), "utf8");
    expect(src).not.toMatch(/ParseStudentBulkCSV/);
    expect(src).not.toMatch(/ProcessStudentBulkRows/);
  });
});
