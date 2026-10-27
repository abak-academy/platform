import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { toast } from "sonner";
import { AffiliatorBulkPanel } from "./AffiliatorBulkPanel";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const presign = vi.fn();
const enqueue = vi.fn();
const putFile = vi.fn();
vi.mock("@/lib/hooks/admin-affiliators", () => ({
  adminAffiliatorsKeys: { all: ["admin", "affiliators"] },
  usePresignAffiliatorBulkUpload: () => ({ mutateAsync: presign, isPending: false }),
  useEnqueueAffiliatorBulkImport: () => ({ mutateAsync: enqueue, isPending: false }),
  putFileToPresignedURL: (...a: unknown[]) => putFile(...a),
}));

let job: { data?: { status: string; progress: number; result_url: string | null; error: string | null } };
// Like the real hook, there is no status until a job id exists.
vi.mock("@/lib/hooks/jobs", () => ({ useJobStatus: (id: string | null) => (id ? job : {}) }));

let downloaded: { name: string; content: string } | null = null;
vi.mock("@/lib/bulk-import-format", async () => {
  const actual = await vi.importActual<typeof import("@/lib/bulk-import-format")>("@/lib/bulk-import-format");
  return { ...actual, downloadTextFile: (name: string, content: string) => (downloaded = { name, content }) };
});

function renderPanel() {
  const qc = new QueryClient();
  const invalidate = vi.spyOn(qc, "invalidateQueries");
  render(<QueryClientProvider client={qc}><AffiliatorBulkPanel /></QueryClientProvider>);
  return invalidate;
}

function pick(name = "affiliators.csv") {
  const file = new File(["name,email,school_code\n"], name, { type: "text/csv" });
  fireEvent.change(screen.getByLabelText("Pilih berkas CSV"), { target: { files: [file] } });
  return file;
}

describe("AffiliatorBulkPanel", () => {
  beforeEach(() => {
    job = {};
    downloaded = null;
    presign.mockReset().mockResolvedValue({ url: "https://storage.test/put", key: "affiliator-bulk/k.csv", method: "PUT" });
    enqueue.mockReset().mockResolvedValue({ job_id: "job-1" });
    putFile.mockReset().mockResolvedValue(undefined);
    vi.mocked(toast.error).mockReset();
  });

  it("downloads a template whose header carries the name, email, password and school columns", () => {
    renderPanel();

    fireEvent.click(screen.getAllByRole("button", { name: "Unduh template" })[0]);

    expect(downloaded?.name).toBe("bulk_affiliator_template.csv");
    expect(downloaded?.content.split("\n")[0]).toBe("name,email,password,school_npsn,school_code");
  });

  it("downloads a guide and lists the columns in the format table", () => {
    renderPanel();

    fireEvent.click(screen.getByRole("button", { name: /Unduh panduan/ }));
    expect(downloaded?.name).toBe("bulk_affiliator_guide.txt");
    expect(downloaded?.content).toContain("school_npsn");
    expect(screen.getByText("school_code")).toBeInTheDocument();
  });

  it("cannot upload before a file is chosen", () => {
    renderPanel();

    expect(screen.getByRole("button", { name: "Unggah" })).toBeDisabled();
  });

  it("uploads through a presigned URL and starts the job with the stored key", async () => {
    renderPanel();
    const file = pick();
    fireEvent.click(screen.getByRole("button", { name: "Unggah" }));

    await waitFor(() => expect(enqueue).toHaveBeenCalledWith({ fileKey: "affiliator-bulk/k.csv" }));
    expect(presign).toHaveBeenCalledWith({ filename: "affiliators.csv", contentType: "text/csv" });
    expect(putFile).toHaveBeenCalledWith("https://storage.test/put", file, "text/csv");
    expect(screen.getByLabelText("Pilih berkas CSV")).toBeDisabled();
  });

  it("stops and says so when the upload itself fails, without starting a job", async () => {
    putFile.mockRejectedValue(new Error("storage down"));
    renderPanel();
    pick();
    fireEvent.click(screen.getByRole("button", { name: "Unggah" }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("storage down"));
    expect(enqueue).not.toHaveBeenCalled();
  });

  it("reports a server refusal such as a wrong header", async () => {
    enqueue.mockRejectedValue(new Error("csv missing required name/email and school_npsn or school_code header"));
    renderPanel();
    pick();
    fireEvent.click(screen.getByRole("button", { name: "Unggah" }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("csv missing required name/email and school_npsn or school_code header"));
  });

  it("shows progress while the job runs", async () => {
    renderPanel();
    pick();
    job = { data: { status: "running", progress: 40, result_url: null, error: null } };
    fireEvent.click(screen.getByRole("button", { name: "Unggah" }));

    expect(await screen.findByText("Memproses… 40%")).toBeInTheDocument();
  });

  it("offers the result file when the job succeeds and refreshes the affiliator lists once", async () => {
    job = { data: { status: "succeeded", progress: 100, result_url: "https://storage.test/result.csv", error: null } };
    const invalidate = renderPanel();
    pick();
    fireEvent.click(screen.getByRole("button", { name: "Unggah" }));

    expect(await screen.findByText("Impor selesai.")).toBeInTheDocument();
    const link = screen.getByRole("link", { name: /Unduh hasil impor/ });
    expect(link).toHaveAttribute("href", "https://storage.test/result.csv");
    expect(screen.getByText(/kata sandi sementara/)).toBeInTheDocument();
    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: ["admin", "affiliators"] }));
    expect(invalidate.mock.calls.filter((c) => JSON.stringify(c[0]).includes("affiliators"))).toHaveLength(1);
  });

  it("still offers the report when every row failed, because that is when it is needed", async () => {
    job = { data: { status: "failed", progress: 100, result_url: "https://storage.test/result.csv", error: "affiliator_bulk job j: all 2 rows failed" } };
    renderPanel();
    pick();
    fireEvent.click(screen.getByRole("button", { name: "Unggah" }));

    expect(await screen.findByText("Impor gagal.")).toBeInTheDocument();
    expect(screen.getByText("affiliator_bulk job j: all 2 rows failed")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Unduh hasil impor/ })).toBeInTheDocument();
  });
});
