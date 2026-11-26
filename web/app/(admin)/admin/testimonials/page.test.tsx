import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "@/lib/api";
import type { AdminTestimonial } from "@/lib/types";
import { useUIStore } from "@/stores/ui";
import TestimonialsPage from "./page";

const toastError = vi.fn();
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: (m: string) => toastError(m) } }));

beforeAll(() => {
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

const stamp = { consent_by: null, consent_by_name: null, consent_at: null, created_at: "2026-11-01T00:00:00Z", updated_at: "2026-11-01T00:00:00Z" };

const full: AdminTestimonial = {
  ...stamp, id: "t1", name: "Peserta Satu", role_label: "SMA Contoh 1", photo_key: "question/a.jpg",
  content: { id: { quote: "Seru sekali", achievements: ["Medali perak"] }, en: { quote: "So fun", achievements: [] } },
  has_consent: true, consent_note: "Formulir WA", is_visible: true, sort_order: 0,
};
const draftNoPhoto: AdminTestimonial = {
  ...stamp, id: "t2", name: "Peserta Dua", role_label: "SMP Contoh 2", photo_key: null,
  content: { id: { quote: "Belajar jadi mudah", achievements: [] }, en: { quote: "Learning is easy", achievements: [] } },
  has_consent: true, consent_note: "Formulir WA", is_visible: false, sort_order: 1,
};
const bare: AdminTestimonial = {
  ...stamp, id: "t3", name: "Peserta Tiga", role_label: "Guru", photo_key: null,
  content: { id: { quote: "Bagus", achievements: [] } },
  has_consent: false, consent_note: null, is_visible: false, sort_order: 2,
};
const noNote: AdminTestimonial = { ...draftNoPhoto, id: "t4", name: "Peserta Empat", photo_key: "question/b.jpg", consent_note: null };
const noEn: AdminTestimonial = { ...draftNoPhoto, id: "t5", name: "Peserta Lima", photo_key: "question/c.jpg", content: { id: { quote: "Halo", achievements: [] } } };

const noId: AdminTestimonial = { ...noEn, id: "t6", name: "Peserta Enam", content: { id: { quote: " ", achievements: [] }, en: { quote: "Hi", achievements: [] } } };

type Call = { path: string; method: string; body: unknown };
let calls: Call[];
let patchError: unknown;
let putFetch: ReturnType<typeof vi.fn>;

function mockApi(list: AdminTestimonial[]) {
  calls = [];
  patchError = undefined;
  vi.spyOn(api, "authFetch").mockImplementation((async (path: string, init?: RequestInit) => {
    const method = init?.method ?? "GET";
    const body = init?.body ? JSON.parse(init.body as string) : undefined;
    calls.push({ path, method, body });
    if (path === "/admin/testimonials" && method === "GET") return { data: list };
    if (path === "/admin/testimonials" && method === "POST") return { ...bare, id: "new1", ...body };
    if (path.startsWith("/admin/testimonials/") && method === "PATCH") {
      if (patchError) throw patchError;
      return { ...list[0], ...body };
    }
    if (path.startsWith("/admin/testimonials/") && method === "DELETE") return undefined;
    if (path.startsWith("/admin/uploads/image")) return { url: "https://upload.example.test/put", method: "PUT", key: "question/new-photo.jpg" };
    return {};
  }) as typeof api.authFetch);
  putFetch = vi.fn().mockResolvedValue({ ok: true, status: 200 });
  vi.stubGlobal("fetch", putFetch);
}

function stubImage(width: number, height: number) {
  vi.stubGlobal("createImageBitmap", vi.fn().mockResolvedValue({ width, height, close: vi.fn() }));
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({ drawImage: vi.fn() } as unknown as CanvasRenderingContext2D);
  vi.spyOn(HTMLCanvasElement.prototype, "toBlob").mockImplementation((cb, type) => cb(new Blob(["jpg"], { type })));
  URL.createObjectURL = vi.fn(() => "blob:preview");
  URL.revokeObjectURL = vi.fn();
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <TestimonialsPage />
    </QueryClientProvider>,
  );
}

const cardNames = () => screen.getAllByRole("article").map((a) => a.getAttribute("aria-label"));
const pick = (group: string, name: string) =>
  fireEvent.click(within(screen.getByRole("group", { name: group })).getByRole("button", { name }));
const upload = (f: File) => userEvent.upload(screen.getByTestId("testimonial-photo-input"), f);
const photoFile = (size = 1000) => {
  const f = new File(["x"], "p.png", { type: "image/png" });
  Object.defineProperty(f, "size", { value: size });
  return f;
};
const writes = () => calls.filter((c) => c.method !== "GET");

beforeEach(() => toastError.mockReset());
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("testimonials i18n", () => {
  afterEach(() => useUIStore.setState({ lang: "id" }));

  it("renders the search placeholder in English", async () => {
    useUIStore.setState({ lang: "en" });
    mockApi([full]);
    renderPage();
    await screen.findByRole("article", { name: "Peserta Satu" });
    expect(screen.getByPlaceholderText("Search name or quote")).toBeInTheDocument();
  });
});

describe("testimonials list", () => {
  it("renders cards with photo, quote, chips and flags", async () => {
    mockApi([full, draftNoPhoto, bare]);
    renderPage();
    await screen.findByRole("article", { name: "Peserta Satu" });
    expect(cardNames()).toEqual(["Peserta Satu", "Peserta Dua", "Peserta Tiga"]);
    const card = within(screen.getByRole("article", { name: "Peserta Satu" }));
    expect(card.getByText("Seru sekali")).toBeInTheDocument();
    expect(card.getByText("Medali perak")).toBeInTheDocument();
    expect(card.getByText("Ada persetujuan")).toBeInTheDocument();
    expect(card.getByText("EN lengkap")).toBeInTheDocument();
    expect(card.getByRole("img", { name: "Peserta Satu" })).toHaveAttribute("src", expect.stringContaining("/files/question/a.jpg"));
    const three = within(screen.getByRole("article", { name: "Peserta Tiga" }));
    expect(three.getByText("Belum ada persetujuan")).toBeInTheDocument();
    expect(three.getByText("EN belum lengkap")).toBeInTheDocument();
  });

  it("searches name and quote, and filters by landing, consent and EN", async () => {
    mockApi([full, draftNoPhoto, bare]);
    renderPage();
    await screen.findByRole("article", { name: "Peserta Satu" });

    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "mudah" } });
    expect(cardNames()).toEqual(["Peserta Dua"]);
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "peserta tiga" } });
    expect(cardNames()).toEqual(["Peserta Tiga"]);
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "" } });

    fireEvent.click(screen.getByRole("button", { name: /^Filter/ }));
    pick("Landing page", "Tampil");
    expect(cardNames()).toEqual(["Peserta Satu"]);
    pick("Landing page", "Draft");
    expect(cardNames()).toEqual(["Peserta Dua", "Peserta Tiga"]);
    pick("Landing page", "Semua");
    pick("Persetujuan", "Belum");
    expect(cardNames()).toEqual(["Peserta Tiga"]);
    pick("Persetujuan", "Ada");
    expect(cardNames()).toEqual(["Peserta Satu", "Peserta Dua"]);
    pick("Persetujuan", "Semua");
    pick("English", "Belum lengkap");
    expect(cardNames()).toEqual(["Peserta Tiga"]);
    pick("English", "Lengkap");
    expect(cardNames()).toEqual(["Peserta Satu", "Peserta Dua"]);
  });
});

describe("visibility switch on a card", () => {
  it.each([
    [bare, "Centang persetujuan tertulis sebelum tampil di landing"],
    [noNote, "Isi catatan lokasi persetujuan sebelum tampil di landing"],
    [noId, "Isi kutipan Indonesia sebelum tampil di landing"],
    [noEn, "Isi kutipan English sebelum tampil di landing"],
    [{ ...draftNoPhoto }, "Unggah foto peserta sebelum tampil di landing"],
  ])("blocks %# client-side naming the missing part", async (item, message) => {
    mockApi([item]);
    renderPage();
    fireEvent.click(await screen.findByRole("switch", { name: `Tampilkan ${item.name} di landing` }));
    expect(toastError).toHaveBeenCalledWith(expect.stringContaining(message));
    expect(writes()).toHaveLength(0);
  });

  it("names every missing part at once", async () => {
    mockApi([bare]);
    renderPage();
    fireEvent.click(await screen.findByRole("switch"));
    const msg = toastError.mock.calls[0][0] as string;
    for (const part of ["persetujuan tertulis", "catatan lokasi", "kutipan English", "foto peserta"]) expect(msg).toContain(part);
  });

  it("patches is_visible when nothing is missing and surfaces a server 422 missing list", async () => {
    mockApi([{ ...draftNoPhoto, photo_key: "question/z.jpg" }]);
    renderPage();
    const sw = await screen.findByRole("switch");
    fireEvent.click(sw);
    await waitFor(() => expect(writes()).toHaveLength(1));
    expect(writes()[0]).toEqual({ path: "/admin/testimonials/t2", method: "PATCH", body: { is_visible: true } });

    patchError = new api.ApiError("validation_failed", "gagal", 422, { missing: ["photo_key"] });
    fireEvent.click(sw);
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("Unggah foto peserta sebelum tampil di landing"));
  });
});

describe("editor", () => {
  async function openCreate() {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Tambah testimoni" }));
    return screen.findByRole("dialog");
  }

  it("creates a testimonial with the typed fields", async () => {
    mockApi([]);
    const dialog = await openCreate();
    const d = within(dialog);
    fireEvent.change(d.getByLabelText("Nama"), { target: { value: "Peserta Baru" } });
    fireEvent.change(d.getByLabelText("Sekolah / peran"), { target: { value: "SMA Baru" } });
    fireEvent.change(d.getByLabelText("Kutipan (Indonesia)"), { target: { value: "Mantap" } });
    fireEvent.change(d.getByLabelText("Kutipan (English)"), { target: { value: "Great" } });
    fireEvent.change(d.getByLabelText("Pencapaian (Indonesia)"), { target: { value: "Juara 1\n\nJuara 2" } });
    fireEvent.click(d.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(writes()).toHaveLength(1));
    expect(writes()[0]).toEqual({
      path: "/admin/testimonials",
      method: "POST",
      body: {
        name: "Peserta Baru", role_label: "SMA Baru",
        content: { id: { quote: "Mantap", achievements: ["Juara 1", "Juara 2"] }, en: { quote: "Great", achievements: [] } },
        has_consent: false, consent_note: null, is_visible: false, sort_order: 0,
      },
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("rejects a 4th achievement before any request", async () => {
    mockApi([]);
    const d = within(await openCreate());
    fireEvent.change(d.getByLabelText("Nama"), { target: { value: "A" } });
    fireEvent.change(d.getByLabelText("Sekolah / peran"), { target: { value: "B" } });
    fireEvent.change(d.getByLabelText("Pencapaian (English)"), { target: { value: "1\n2\n3\n4" } });
    fireEvent.click(d.getByRole("button", { name: "Simpan" }));
    expect(await d.findByRole("alert")).toHaveTextContent("Maksimal 3 pencapaian per bahasa");
    expect(writes()).toHaveLength(0);
  });

  it("edits an existing testimonial with a PATCH", async () => {
    mockApi([full]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const d = within(await screen.findByRole("dialog"));
    expect(d.getByLabelText("Nama")).toHaveValue("Peserta Satu");
    fireEvent.change(d.getByLabelText("Nama"), { target: { value: "Nama Baru" } });
    fireEvent.click(d.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(writes()).toHaveLength(1));
    expect(writes()[0].path).toBe("/admin/testimonials/t1");
    expect(writes()[0].method).toBe("PATCH");
    expect(writes()[0].body).toEqual({ name: "Nama Baru" });
  });

  it("sends no request when an edit changes nothing", async () => {
    mockApi([noEn]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const d = within(await screen.findByRole("dialog"));
    fireEvent.click(d.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(writes()).toHaveLength(0);
  });

  it("blocks Save for a visible testimonial whose English quote was cleared", async () => {
    mockApi([full]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const d = within(await screen.findByRole("dialog"));
    fireEvent.change(d.getByLabelText("Kutipan (English)"), { target: { value: "" } });
    fireEvent.click(d.getByRole("button", { name: "Simpan" }));
    expect(await d.findByRole("alert")).toHaveTextContent("Isi kutipan English sebelum tampil di landing");
    expect(writes()).toHaveLength(0);
  });

  it("blocks Save for a visible testimonial whose Indonesian quote was cleared", async () => {
    mockApi([full]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const d = within(await screen.findByRole("dialog"));
    fireEvent.change(d.getByLabelText("Kutipan (Indonesia)"), { target: { value: "" } });
    fireEvent.click(d.getByRole("button", { name: "Simpan" }));
    expect(await d.findByRole("alert")).toHaveTextContent("Isi kutipan Indonesia sebelum tampil di landing");
    expect(writes()).toHaveLength(0);
  });

  it("names a non-integer sort order instead of sending it", async () => {
    mockApi([full]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const d = within(await screen.findByRole("dialog"));
    fireEvent.change(d.getByLabelText("Urutan"), { target: { value: "1.5" } });
    fireEvent.click(d.getByRole("button", { name: "Simpan" }));
    expect(await d.findByRole("alert")).toHaveTextContent("Urutan harus bilangan bulat");
    expect(writes()).toHaveLength(0);
  });

  it("revokes a replaced preview and the last one when the dialog closes", async () => {
    mockApi([draftNoPhoto]);
    stubImage(3000, 2000);
    let n = 0;
    URL.createObjectURL = vi.fn(() => `blob:p${++n}`);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const d = within(await screen.findByRole("dialog"));
    await upload(photoFile());
    await d.findByAltText("Pratinjau foto peserta");
    await upload(photoFile());
    await waitFor(() => expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:p1"));
    fireEvent.click(d.getByRole("button", { name: "Batal" }));
    await waitFor(() => expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:p2"));
  });

  it("blocks the editor switch with the photo message", async () => {
    mockApi([]);
    const d = within(await openCreate());
    fireEvent.click(d.getByRole("switch", { name: "Tampil di landing page" }));
    expect(d.getByRole("alert")).toHaveTextContent("Unggah foto peserta sebelum tampil di landing");
    expect(d.getByRole("switch", { name: "Tampil di landing page" })).toHaveAttribute("aria-checked", "false");
    expect(writes()).toHaveLength(0);
  });

  it("unchecking consent in the editor hides a visible testimonial", async () => {
    mockApi([full]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const d = within(await screen.findByRole("dialog"));
    expect(d.getByRole("switch", { name: "Tampil di landing page" })).toHaveAttribute("aria-checked", "true");
    fireEvent.click(d.getByRole("checkbox", { name: "Ada persetujuan tertulis" }));
    expect(d.getByRole("switch", { name: "Tampil di landing page" })).toHaveAttribute("aria-checked", "false");
  });

  it("refuses a file over 5 MB and one under 1200 x 1500 with their messages", async () => {
    mockApi([]);
    stubImage(1000, 1000);
    const d = within(await openCreate());
    await upload(photoFile(5 * 1024 * 1024 + 1));
    expect(await d.findByRole("alert")).toHaveTextContent("Foto lebih dari 5 MB");
    await upload(photoFile());
    await waitFor(() => expect(d.getByRole("alert")).toHaveTextContent("Foto minimal 1200 × 1500 px"));
    expect(d.queryByAltText("Pratinjau foto peserta")).not.toBeInTheDocument();
  });

  it("previews a picked photo but uploads only on Save, before the PATCH that sets photo_key", async () => {
    mockApi([draftNoPhoto]);
    stubImage(3000, 2000);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const d = within(await screen.findByRole("dialog"));
    await upload(photoFile());
    expect(await d.findByAltText("Pratinjau foto peserta")).toHaveAttribute("src", "blob:preview");
    expect(putFetch).not.toHaveBeenCalled();
    expect(calls.some((c) => c.path.startsWith("/admin/uploads/image"))).toBe(false);

    fireEvent.click(d.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(writes().some((c) => c.method === "PATCH")).toBe(true));
    const order = calls.map((c) => (c.path.startsWith("/admin/uploads/image") ? "presign" : c.method));
    expect(order.indexOf("presign")).toBeLessThan(order.indexOf("PATCH"));
    expect(calls.find((c) => c.path.startsWith("/admin/uploads/image"))!.path).toContain("content_type=image%2Fjpeg");
    expect(putFetch).toHaveBeenCalledWith("https://upload.example.test/put", expect.objectContaining({ method: "PUT" }));
    const sent = putFetch.mock.calls[0][1].body as Blob;
    expect(sent.type).toBe("image/jpeg");
    expect(calls.find((c) => c.method === "PATCH")!.body).toMatchObject({ photo_key: "question/new-photo.jpg" });
  });

  it("removing a photo sends photo_key null and hides the testimonial", async () => {
    mockApi([full]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const d = within(await screen.findByRole("dialog"));
    fireEvent.click(d.getByRole("button", { name: "Hapus foto" }));
    fireEvent.click(d.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(writes()).toHaveLength(1));
    expect(writes()[0].body).toMatchObject({ photo_key: null, is_visible: false });
  });

  it("shows the server 422 missing list in the dialog", async () => {
    mockApi([full]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const d = within(await screen.findByRole("dialog"));
    patchError = new api.ApiError("validation_failed", "gagal", 422, { missing: ["consent_note"] });
    fireEvent.change(d.getByLabelText("Nama"), { target: { value: "Nama Baru" } });
    fireEvent.click(d.getByRole("button", { name: "Simpan" }));
    expect(await d.findByRole("alert")).toHaveTextContent("Isi catatan lokasi persetujuan sebelum tampil di landing");
  });
});

describe("delete", () => {
  it("asks for confirmation, then deletes", async () => {
    mockApi([full, draftNoPhoto]);
    renderPage();
    const card = within(await screen.findByRole("article", { name: "Peserta Dua" }));
    fireEvent.click(card.getByRole("button", { name: "Hapus" }));
    const d = within(await screen.findByRole("dialog"));
    expect(writes()).toHaveLength(0);
    fireEvent.click(d.getByRole("button", { name: "Hapus" }));
    await waitFor(() => expect(writes()).toHaveLength(1));
    expect(writes()[0]).toEqual({ path: "/admin/testimonials/t2", method: "DELETE", body: undefined });
  });

  it("cancel leaves it", async () => {
    mockApi([full]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Hapus" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Batal" }));
    expect(writes()).toHaveLength(0);
  });
});
