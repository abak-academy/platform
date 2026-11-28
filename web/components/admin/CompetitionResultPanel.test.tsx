import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import * as api from "@/lib/api";
import { ApiError } from "@/lib/api";
import { CompetitionResultPanel } from "./CompetitionResultPanel";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

type Call = { path: string; method: string; body: unknown };
let calls: Call[];
let saved: unknown;
let previewResult: () => unknown;
let applyResult: () => unknown;
let fetchMock: ReturnType<typeof vi.fn>;

const draft = { id: "c1", exam_id: "e1", source_key: "k", row_count: 3, published_at: null, updated_at: "2026-11-01T03:00:00Z" };
const live = { ...draft, published_at: "2026-11-02T03:00:00Z" };
const URL_BASE = "/admin/exams/e1/competition-result";

beforeEach(() => {
  calls = [];
  saved = null;
  previewResult = () => ({ matched: 2, errors: [], result_exists: false, result_published: false });
  applyResult = () => ({ data: draft });
  fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200 });
  vi.stubGlobal("fetch", fetchMock);
  vi.spyOn(api, "authFetch").mockImplementation((async (path: string, init?: RequestInit) => {
    const method = init?.method ?? "GET";
    calls.push({ path, method, body: init?.body ? JSON.parse(init.body as string) : undefined });
    if (path === URL_BASE && method === "GET") return { data: saved };
    if (path.includes("/presign?")) return { url: "https://minio.test/put", method: "PUT", fields: {}, key: "cr/e1/x.csv" };
    if (path.endsWith("/preview")) return previewResult();
    if (path.endsWith("/apply")) return applyResult();
    if (path.endsWith("/results")) {
      return { data: [{ registration_id: "r1", student_name: "Siswa Uji", participant_no: "ESOL-0001", score: 90, rank: 1, award: "Emas", certificate_url: null }] };
    }
    return {};
  }) as typeof api.authFetch);
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function renderDialog(canWrite = true) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <CompetitionResultPanel examId="e1" canWrite={canWrite} />
    </QueryClientProvider>,
  );
}

function uploadAndPreview() {
  fireEvent.change(screen.getByLabelText("Berkas CSV"), { target: { files: [new File(["x"], "hasil.csv", { type: "text/csv" })] } });
  fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
}

const paths = (m: string) => calls.filter((c) => c.method === m).map((c) => c.path);

describe("CompetitionResultPanel", () => {
  it("renders the saved result from the exam id alone", async () => {
    saved = draft;
    renderDialog();
    expect(await screen.findByRole("heading", { name: "Impor hasil kompetisi" })).toBeTruthy();
    expect(screen.getByText(/tidak mengubah skor di platform/)).toBeTruthy();
    expect(await screen.findByText(/3 baris/)).toBeTruthy();
    expect(screen.getByText("Draf")).toBeTruthy();
    expect(calls[0].path).toBe(URL_BASE);
  });

  it("shows the empty state when nothing is saved", async () => {
    renderDialog();
    expect(await screen.findByText("Belum ada hasil kompetisi.")).toBeTruthy();
  });

  it("uploads, previews and applies with the right requests", async () => {
    renderDialog();
    uploadAndPreview();
    expect(await screen.findByText(/baris cocok/)).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledWith("https://minio.test/put", expect.objectContaining({ method: "PUT" }));
    const presignCall = calls.find((c) => c.path.includes("/presign?"));
    expect(presignCall?.path).toContain("?filename=hasil.csv");
    expect(presignCall?.body).toBeUndefined();
    expect(calls.find((c) => c.path.endsWith("/preview"))?.body).toEqual({ key: "cr/e1/x.csv" });

    fireEvent.click(screen.getByRole("button", { name: "Terapkan" }));
    await waitFor(() => expect(calls.find((c) => c.path.endsWith("/apply"))?.body).toEqual({ key: "cr/e1/x.csv" }));
  });

  it("publishes a draft", async () => {
    saved = draft;
    renderDialog();
    fireEvent.click(await screen.findByRole("button", { name: "Tayangkan" }));
    await waitFor(() => expect(paths("POST")).toContain(`${URL_BASE}/publish`));
  });

  it("unpublishes a live result", async () => {
    saved = live;
    renderDialog();
    fireEvent.click(await screen.findByRole("button", { name: "Tarik" }));
    await waitFor(() => expect(paths("POST")).toContain(`${URL_BASE}/unpublish`));
  });

  it("lists the result rows on Lihat", async () => {
    saved = draft;
    renderDialog();
    fireEvent.click(await screen.findByRole("button", { name: "Lihat" }));
    expect(await screen.findByText("Siswa Uji")).toBeTruthy();
    expect(paths("GET")).toContain(`${URL_BASE}/results`);
  });

  it("lists every error with Indonesian reason and blocks confirm", async () => {
    previewResult = () => ({
      matched: 1,
      errors: [
        { line: 3, participant_no: "ESOL-0009", reason: "unknown_participant" },
        { line: 5, participant_no: "ESOL-0010", reason: "bad_score" },
      ],
      result_exists: false,
      result_published: false,
    });
    renderDialog();
    uploadAndPreview();
    expect(await screen.findByText("Nomor peserta tidak dikenal")).toBeTruthy();
    expect(screen.getByText("Skor tidak valid")).toBeTruthy();
    expect(screen.getByText("ESOL-0010")).toBeTruthy();
    const confirm = screen.getByRole("button", { name: "Terapkan" }) as HTMLButtonElement;
    expect(confirm.disabled).toBe(true);
    fireEvent.click(confirm);
    expect(paths("POST").some((p) => p.endsWith("/apply"))).toBe(false);
  });

  it("shows a localized message for a file-level 422 and offers no confirm", async () => {
    previewResult = () => {
      throw new ApiError("validation_failed", 'validation failed: missing_column "participant_no"', 422);
    };
    renderDialog();
    uploadAndPreview();
    expect((await screen.findByRole("alert")).textContent).toBe("CSV wajib punya kolom participant_no dan score.");
    expect(screen.queryByRole("button", { name: "Terapkan" })).toBeNull();
  });

  it("falls back to a localized generic message for an unknown error", async () => {
    previewResult = () => {
      throw new ApiError("boom", "raw english", 500);
    };
    renderDialog();
    uploadAndPreview();
    expect((await screen.findByRole("alert")).textContent).toBe("Terjadi kesalahan. Coba lagi.");
  });

  it("shows the row errors and disables Terapkan when apply returns 422", async () => {
    applyResult = () => {
      throw new ApiError("validation_failed", "file has errors", 422, {
        code: "validation_failed",
        message: "file has errors",
        matched: 1,
        errors: [{ line: 4, participant_no: "ESOL-0042", reason: "unknown_participant" }],
      });
    };
    renderDialog();
    uploadAndPreview();
    await screen.findByText(/baris cocok/);
    fireEvent.click(screen.getByRole("button", { name: "Terapkan" }));
    expect(await screen.findByText("ESOL-0042")).toBeTruthy();
    expect(screen.getByText("Nomor peserta tidak dikenal")).toBeTruthy();
    expect((screen.getByRole("button", { name: "Terapkan" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("warns when the replaced result is already published", async () => {
    saved = live;
    previewResult = () => ({ matched: 2, errors: [], result_exists: true, result_published: true });
    renderDialog();
    await screen.findByText("Tayang");
    uploadAndPreview();
    expect(await screen.findByText(/Hasil ini sudah tayang/)).toBeTruthy();
    expect((screen.getByRole("button", { name: "Terapkan" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("deletes only after confirmation", async () => {
    saved = draft;
    renderDialog();
    fireEvent.click(await screen.findByRole("button", { name: "Hapus" }));
    expect(paths("DELETE")).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: "Ya, hapus" }));
    await waitFor(() => expect(paths("DELETE")).toEqual([URL_BASE]));
  });

  it("shows only the read-only saved result to a non-writer", async () => {
    saved = live;
    renderDialog(false);
    expect(await screen.findByText(/3 baris/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Lihat" })).toBeTruthy();
    for (const name of ["Tayangkan", "Tarik", "Hapus", "Pratinjau"]) expect(screen.queryByRole("button", { name })).toBeNull();
    expect(screen.queryByLabelText("Berkas CSV")).toBeNull();
  });

  it("renders nothing when the result GET is forbidden", async () => {
    vi.spyOn(api, "authFetch").mockRejectedValue(new ApiError("forbidden", "no", 403));
    renderDialog();
    await waitFor(() => expect(screen.queryByRole("heading", { name: "Impor hasil kompetisi" })).toBeNull());
  });
});
