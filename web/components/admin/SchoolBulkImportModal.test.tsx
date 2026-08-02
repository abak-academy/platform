import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SchoolBulkImportModal } from "./SchoolBulkImportModal";

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

vi.mock("@/lib/hooks/admin-schools-bulk", () => ({
  usePresignSchoolBulkUpload: () => ({
    mutateAsync: presignMutateAsync,
    isPending: false,
  }),
  putFileToPresignedURL: (...args: Parameters<typeof putFile>) => putFile(...args),
  useEnqueueSchoolBulkImport: () => ({
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

describe("SchoolBulkImportModal", () => {
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
    render(<SchoolBulkImportModal open={false} onOpenChange={vi.fn()} />, {
      wrapper: wrapperFactory(),
    });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders the dialog title and a Download Template button when open", () => {
    render(<SchoolBulkImportModal open={true} onOpenChange={vi.fn()} />, {
      wrapper: wrapperFactory(),
    });
    expect(screen.getByText("bulk_school_title")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /bulk_school_download_template/i }),
    ).toBeInTheDocument();
  });

  it("clicking Download Template produces the exact D-3 header with the D-4 pipe-encoded example row, firing no network request", async () => {
    render(<SchoolBulkImportModal open={true} onOpenChange={vi.fn()} />, {
      wrapper: wrapperFactory(),
    });

    const downloadBtn = screen.getByRole("button", { name: /bulk_school_download_template/i });
    fireEvent.click(downloadBtn);

    await waitFor(() => expect(lastDownloadedFilename).not.toBeNull());
    await waitFor(() => expect(lastDownloadedCSV).not.toBeNull());

    expect(lastDownloadedFilename).toBe("bulk_school_template.csv");

    const lines = (lastDownloadedCSV ?? "").split(/\r?\n/).filter(Boolean);
    expect(lines.length).toBe(2);
    // Exact header string per D-3 — this is the same string the backend
    // ParseSchoolBulkCSV parser requires (service/school_bulk.go).
    expect(lines[0]).toBe("name,code,npsn,school_types,alamat");

    const exampleCells = lines[1].split(",");
    // school_types is the 4th column; D-4 requires pipe encoding in-cell.
    expect(exampleCells[3]).toContain("|");
    expect(exampleCells[3]).not.toContain(",");

    // FR-31: the whole file, byte for byte. The identical literal is
    // frontendSchoolBulkTemplateCSV in
    // backend/internal/service/school_bulk_test.go, where it is fed through the
    // real ParseSchoolBulkCSV — changing the template here without changing it
    // there fails that test.
    expect(lastDownloadedCSV).toBe(
      "name,code,npsn,school_types,alamat\n" +
        "SMAN 1 Jakarta,SMAN1JKT,20100001,sma|smk,Jl. Sudirman No. 1\n",
    );

    expect(presignMutateAsync).not.toHaveBeenCalled();
    expect(putFile).not.toHaveBeenCalled();
    expect(enqueueMutateAsync).not.toHaveBeenCalled();
  });

  it("uploading a valid CSV runs presign -> PUT -> enqueue in order with the presigned key", async () => {
    presignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/k?sig=xyz",
      method: "PUT",
      key: "school-bulk/uuid.csv",
    });
    enqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-1" });
    putFile.mockResolvedValueOnce(undefined);

    render(<SchoolBulkImportModal open={true} onOpenChange={vi.fn()} />, {
      wrapper: wrapperFactory(),
    });

    const fileInput = screen.getByLabelText(/choose_file|file/i) as HTMLInputElement;
    const file = new File(
      ["name,code,npsn,school_types,alamat\nSMAN 1 Jakarta,SMAN1JKT,20100001,sma|smk,Jl. Sudirman No. 1"],
      "schools.csv",
      { type: "text/csv" },
    );
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
      fileKey: "school-bulk/uuid.csv",
    });
  });

  it("shows progress rendering while the job is in progress", async () => {
    presignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/k?sig=xyz",
      method: "PUT",
      key: "school-bulk/k1.csv",
    });
    enqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-progress" });
    putFile.mockResolvedValueOnce(undefined);

    const { rerender } = render(
      <SchoolBulkImportModal open={true} onOpenChange={vi.fn()} />,
      { wrapper: wrapperFactory() },
    );

    const fileInput = screen.getByLabelText(/choose_file|file/i) as HTMLInputElement;
    const file = new File(["x"], "x.csv", { type: "text/csv" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    const submitBtn = screen.getByRole("button", { name: /upload|import|submit|start/i });
    fireEvent.click(submitBtn);

    await waitFor(() => expect(enqueueMutateAsync).toHaveBeenCalled());

    jobStatusState.data = {
      id: "job-progress",
      type: "school_bulk",
      status: "processing",
      progress: 40,
      result_url: null,
      error: null,
      created_at: "2026-08-02T00:00:00Z",
      updated_at: "2026-08-02T00:01:00Z",
    };
    pollTick++;
    rerender(<SchoolBulkImportModal open={true} onOpenChange={vi.fn()} />);

    await waitFor(() => {
      const bar = document.querySelector("div.bg-primary.transition-all") as HTMLElement | null;
      expect(bar).not.toBeNull();
      expect(bar?.style.width).toBe("40%");
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
      <SchoolBulkImportModal open={true} onOpenChange={vi.fn()} />,
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
      type: "school_bulk",
      status: "failed",
      progress: 0,
      result_url: null,
      error: "school code already taken",
      created_at: "2026-08-02T00:00:00Z",
      updated_at: "2026-08-02T00:01:00Z",
    };
    pollTick++;
    rerender(<SchoolBulkImportModal open={true} onOpenChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText(/school code already taken/i)).toBeInTheDocument();
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
      <SchoolBulkImportModal open={true} onOpenChange={vi.fn()} />,
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
      type: "school_bulk",
      status: "succeeded",
      progress: 100,
      result_url: "http://minio.local/result.csv?sig=abc",
      error: null,
      created_at: "2026-08-02T00:00:00Z",
      updated_at: "2026-08-02T00:02:00Z",
    };
    pollTick++;
    rerender(<SchoolBulkImportModal open={true} onOpenChange={vi.fn()} />);

    await waitFor(() => {
      const link = screen.getByRole("link");
      expect((link as HTMLAnchorElement).href).toBe(
        "http://minio.local/result.csv?sig=abc",
      );
    });
  });

  it("calls onImportSuccess exactly once when the job reaches succeeded", async () => {
    presignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/k?sig=xyz",
      method: "PUT",
      key: "k3",
    });
    enqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-ok2" });
    putFile.mockResolvedValueOnce(undefined);

    const onImportSuccess = vi.fn();
    const { rerender } = render(
      <SchoolBulkImportModal open={true} onOpenChange={vi.fn()} onImportSuccess={onImportSuccess} />,
      { wrapper: wrapperFactory() },
    );

    const fileInput = screen.getByLabelText(/choose_file|file/i) as HTMLInputElement;
    const file = new File(["x"], "x.csv", { type: "text/csv" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    const submitBtn = screen.getByRole("button", { name: /upload|import|submit|start/i });
    fireEvent.click(submitBtn);

    await waitFor(() => expect(enqueueMutateAsync).toHaveBeenCalled());
    expect(onImportSuccess).not.toHaveBeenCalled();

    jobStatusState.data = {
      id: "job-ok2",
      type: "school_bulk",
      status: "succeeded",
      progress: 100,
      result_url: "http://minio.local/result.csv?sig=abc",
      error: null,
      created_at: "2026-08-02T00:00:00Z",
      updated_at: "2026-08-02T00:02:00Z",
    };
    pollTick++;
    rerender(
      <SchoolBulkImportModal open={true} onOpenChange={vi.fn()} onImportSuccess={onImportSuccess} />,
    );

    await waitFor(() => expect(onImportSuccess).toHaveBeenCalledTimes(1));

    // A subsequent rerender with the same terminal state must not re-fire it.
    pollTick++;
    rerender(
      <SchoolBulkImportModal open={true} onOpenChange={vi.fn()} onImportSuccess={onImportSuccess} />,
    );
    expect(onImportSuccess).toHaveBeenCalledTimes(1);
  });

  it("does not import any direct school-CSV service path (only HTTP hooks)", async () => {
    const fs = await import("fs");
    const path = await import("path");
    const src = fs.readFileSync(path.join(__dirname, "SchoolBulkImportModal.tsx"), "utf8");
    expect(src).not.toMatch(/ParseSchoolBulkCSV/);
    expect(src).not.toMatch(/ProcessSchoolBulkRows/);
  });
});
