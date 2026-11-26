import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import * as api from "@/lib/api";
import { ApiError } from "@/lib/api";
import { ExternalRoundsDialog } from "./ExternalRoundsDialog";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

type Call = { path: string; method: string; body: unknown };
let calls: Call[];
let rounds: unknown[];
let previewResult: () => unknown;
let fetchMock: ReturnType<typeof vi.fn>;

const draftRound = { id: "r1", exam_id: "e1", label: "Semifinal", label_en: null, row_count: 3, published_at: null, updated_at: "2026-11-01T03:00:00Z" };
const livePast = { ...draftRound, id: "r2", label: "Final", published_at: "2026-11-02T03:00:00Z" };

beforeEach(() => {
  calls = [];
  rounds = [];
  previewResult = () => ({ matched: 2, errors: [], round_exists: false, round_published: false });
  fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200 });
  vi.stubGlobal("fetch", fetchMock);
  vi.spyOn(api, "authFetch").mockImplementation((async (path: string, init?: RequestInit) => {
    const method = init?.method ?? "GET";
    calls.push({ path, method, body: init?.body ? JSON.parse(init.body as string) : undefined });
    if (path === "/admin/exams/e1/external-rounds" && method === "GET") return { data: rounds };
    if (path.includes("/presign")) return { url: "https://minio.test/put", method: "PUT", key: "ext/e1/x.csv" };
    if (path.endsWith("/preview")) return previewResult();
    if (path.endsWith("/apply")) return { data: draftRound };
    return {};
  }) as typeof api.authFetch);
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function renderDialog() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <ExternalRoundsDialog open onOpenChange={() => {}} productName="ESOLIO 2026" examIds={["e1"]} />
    </QueryClientProvider>,
  );
}

async function uploadAndPreview(label = "Semifinal") {
  fireEvent.change(screen.getByLabelText("Label ronde"), { target: { value: label } });
  fireEvent.change(screen.getByLabelText("Berkas CSV"), { target: { files: [new File(["x"], "hasil.csv", { type: "text/csv" })] } });
  fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
}

const paths = (m: string) => calls.filter((c) => c.method === m).map((c) => c.path);

describe("ExternalRoundsDialog", () => {
  it("uploads, previews, applies and publishes with the right requests", async () => {
    renderDialog();
    await uploadAndPreview();
    expect(await screen.findByText(/baris cocok/)).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledWith("https://minio.test/put", expect.objectContaining({ method: "PUT" }));
    expect(calls.find((c) => c.path.endsWith("/preview"))?.body).toEqual({ key: "ext/e1/x.csv", round_label: "Semifinal" });
    expect(calls.find((c) => c.path.includes("/presign"))?.path).toBe("/admin/exams/e1/external-rounds/presign?filename=hasil.csv");

    rounds = [draftRound];
    fireEvent.click(screen.getByRole("button", { name: "Terapkan" }));
    await waitFor(() => expect(calls.find((c) => c.path.endsWith("/apply"))?.body).toEqual({ key: "ext/e1/x.csv", round_label: "Semifinal" }));

    fireEvent.click(await screen.findByRole("button", { name: "Tayangkan" }));
    await waitFor(() => expect(paths("POST")).toContain("/admin/external-rounds/r1/publish"));
  });

  it("lists every error with Indonesian reason and blocks confirm", async () => {
    previewResult = () => ({
      matched: 1,
      errors: [
        { line: 3, participant_no: "ESOL-0009", reason: "unknown_participant" },
        { line: 5, participant_no: "ESOL-0010", reason: "bad_score" },
      ],
      round_exists: false,
      round_published: false,
    });
    renderDialog();
    await uploadAndPreview();
    expect(await screen.findByText("Nomor peserta tidak dikenal")).toBeTruthy();
    expect(screen.getByText("Skor tidak valid")).toBeTruthy();
    expect(screen.getByText("ESOL-0010")).toBeTruthy();
    const confirm = screen.getByRole("button", { name: "Terapkan" }) as HTMLButtonElement;
    expect(confirm.disabled).toBe(true);
    fireEvent.click(confirm);
    expect(paths("POST").some((p) => p.endsWith("/apply"))).toBe(false);
  });

  it("shows the file-level 422 message and offers no confirm", async () => {
    previewResult = () => {
      throw new ApiError("validation_failed", "kolom participant_no tidak ditemukan", 422);
    };
    renderDialog();
    await uploadAndPreview();
    expect((await screen.findByRole("alert")).textContent).toContain("kolom participant_no tidak ditemukan");
    expect(screen.queryByRole("button", { name: "Terapkan" })).toBeNull();
  });

  it("warns when the replaced round is already published", async () => {
    rounds = [livePast];
    previewResult = () => ({ matched: 2, errors: [], round_exists: true, round_published: true });
    renderDialog();
    await screen.findByText("Final");
    await uploadAndPreview("Final");
    expect(await screen.findByText(/Ronde ini sudah tayang/)).toBeTruthy();
    expect((screen.getByRole("button", { name: "Terapkan" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("deletes only after confirmation", async () => {
    rounds = [draftRound];
    renderDialog();
    fireEvent.click(await screen.findByRole("button", { name: "Hapus Semifinal" }));
    expect(paths("DELETE")).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: "Ya, hapus" }));
    await waitFor(() => expect(paths("DELETE")).toEqual(["/admin/external-rounds/r1"]));
  });
});
