import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import * as api from "@/lib/api";
import type { Product, ProductGroupingDetail } from "@/lib/types";
import ProductGroupingDetailPage from "./page";

const push = vi.fn();
vi.mock("next/navigation", () => ({ useParams: () => ({ id: "g1" }), useRouter: () => ({ push }) }));
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast }));

const p1: Product = { id: "p1", type: "exam", name: "ESOLIO 2026 SD", price: 150000, phase: "open", stages: ["SD"], grade_min: 1, grade_max: 3, multi_event: true };
const p2: Product = { id: "p2", type: "exam", name: "ESOLIO 2025 SMP", price: 0, phase: "done", stages: [], grade_min: null, grade_max: null };
const p3: Product = { id: "p3", type: "merchandise", name: "Kaos ESOLIO", price: 80000 };

const detail: ProductGroupingDetail = {
  id: "g1",
  name: "ESOLIO",
  description: "Olimpiade",
  status: "published",
  display_order: 0,
  slug: "esolio",
  short_name: "Einstein",
  scope: "international",
  subjects: ["Matematika", "Sains"],
  tone: "teal",
  illustration: "science",
  logo_key: null,
  official_url: "https://esolio.example.test",
  is_visible: false,
  participant_prefix: "ESOL",
  content: {
    id: {
      full_name: "Einstein Olympiad",
      tagline: "Olimpiade sains dunia",
      levels_label: "Kelas 1–12",
      story: ["Paragraf satu", "Paragraf dua"],
      rounds: ["Babak 1", "Babak 2"],
      categories: [{ name: "SD", detail: "Kelas 1–6" }],
      facts: [{ label: "Biaya", value: "Rp150.000" }],
    },
    en: {
      full_name: "Einstein Olympiad EN",
      tagline: "World science olympiad",
      levels_label: "Grades 1–12",
      story: ["Paragraph one"],
      rounds: ["Round 1"],
      categories: [{ name: "Elementary", detail: "Grades 1–6" }],
      facts: [{ label: "Fee", value: "IDR 150,000" }],
    },
  },
  schedule: [
    { id: "s1", label: "Pendaftaran", label_en: "Registration", starts_on: "2026-12-01", ends_on: "2026-12-05" },
    { id: "s2", label: "Final", label_en: "Final round", starts_on: "2027-02-01", ends_on: null },
  ],
  en_complete: true,
  products: [p1, p2, p3],
};

const roundTrip = {
  name: "ESOLIO",
  description: "Olimpiade",
  status: "published",
  short_name: "Einstein",
  scope: "international",
  subjects: ["Matematika", "Sains"],
  tone: "teal",
  illustration: "science",
  logo_key: "",
  official_url: "https://esolio.example.test",
  content: {
    id: {
      full_name: "Einstein Olympiad",
      tagline: "Olimpiade sains dunia",
      levels_label: "Kelas 1–12",
      story: ["Paragraf satu", "Paragraf dua"],
      rounds: ["Babak 1", "Babak 2"],
      categories: [{ name: "SD", detail: "Kelas 1–6" }],
      facts: [{ label: "Biaya", value: "Rp150.000" }],
    },
    en: {
      full_name: "Einstein Olympiad EN",
      tagline: "World science olympiad",
      levels_label: "Grades 1–12",
      story: ["Paragraph one"],
      rounds: ["Round 1"],
      categories: [{ name: "Elementary", detail: "Grades 1–6" }],
      facts: [{ label: "Fee", value: "IDR 150,000" }],
    },
  },
  is_visible: false,
  participant_prefix: "ESOL",
  schedule: [
    { label: "Pendaftaran", label_en: "Registration", starts_on: "2026-12-01", ends_on: "2026-12-05" },
    { label: "Final", label_en: "Final round", starts_on: "2027-02-01", ends_on: null },
  ],
};

type Call = { path: string; method: string; body: unknown };
let calls: Call[];
let current: ProductGroupingDetail;
let patchError: unknown;

function mockApi(overrides: Partial<ProductGroupingDetail> = {}) {
  calls = [];
  patchError = undefined;
  current = { ...detail, ...overrides };
  vi.spyOn(api, "authFetch").mockImplementation((async (path: string, init?: RequestInit) => {
    const method = init?.method ?? "GET";
    const body = init?.body ? JSON.parse(init.body as string) : undefined;
    calls.push({ path, method, body });
    if (path === "/admin/product-groupings/g1" && method === "GET") return current;
    if (path === "/admin/product-groupings/g1" && method === "PATCH") {
      if (patchError) throw patchError;
      return { ...current, ...body, content: body.content, schedule: body.schedule };
    }
    if (path === "/admin/product-groupings/g1/products") return current;
    if (path === "/admin/product-groupings/g1" && method === "DELETE") return undefined;
    if (path === "/admin/products") return { data: [p1, p2, p3, { id: "p4", type: "book", name: "Buku Latihan", price: 50000 }] };
    if (path.startsWith("/admin/uploads/image")) return { url: "https://upload.example.test/put", method: "PUT", key: "question/logo-abc.png" };
    return {};
  }) as typeof api.authFetch);
}

const patches = () => calls.filter((c) => c.path === "/admin/product-groupings/g1" && c.method === "PATCH");

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <ProductGroupingDetailPage />
    </QueryClientProvider>,
  );
}

async function ready() {
  await screen.findByLabelText("Nama event");
}

const field = (label: string) => screen.getByLabelText(label);
const type = (label: string, value: string) => fireEvent.change(field(label), { target: { value } });
const save = () => fireEvent.click(screen.getByRole("button", { name: "Simpan event" }));
const toggle = () => screen.getByRole("switch", { name: "Tampil di landing page" });

beforeEach(() => {
  push.mockReset();
  toast.success.mockReset();
  toast.error.mockReset();
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("event editor: fields", () => {
  it("round-trips an untouched event: the PATCH body equals what was loaded", async () => {
    mockApi();
    renderPage();
    await ready();

    expect(field("Prefiks nomor peserta")).toHaveValue("ESOL");
    expect(field("Cerita (ID)")).toHaveValue("Paragraf satu\n\nParagraf dua");
    expect(field("Kategori (ID)")).toHaveValue("SD | Kelas 1–6");
    save();

    await waitFor(() => expect(patches()).toHaveLength(1));
    expect(patches()[0].body).toEqual(roundTrip);
  });

  // ~40 getByLabelText calls over the full editor DOM exceed the 5s default under suite load.
  it("sends every edited field from both language tabs, the schedule and the display card", async () => {
    mockApi();
    renderPage();
    await ready();

    type("Nama event", "  ESOLIO Baru ");
    type("Nama pendek", "Einstein 2");
    type("Deskripsi (filter Toko)", "Filter Toko");
    type("Tingkat", "national");
    type("Situs resmi", "https://baru.example.test");

    type("Nama lengkap (ID)", "Nama lengkap baru");
    type("Label jenjang (ID)", "Kelas 4–12");
    type("Tagline (ID)", "Tagline baru");
    type("Cerita (ID)", "A\n\nB\n\n\nC");
    type("Alur babak (ID)", "Satu\n\n Dua ");
    type("Kategori (ID)", "SD | Kelas 1–6\nSMP | Kelas 7–9\nTanpa detail");
    type("Fakta singkat (ID)", "Biaya | Rp1 | x\nTempat | Online");

    fireEvent.click(screen.getByRole("tab", { name: /English/ }));
    type("Nama lengkap (EN)", "Full name new");
    type("Label jenjang (EN)", "Grades 4–12");
    type("Tagline (EN)", "New tagline");
    type("Cerita (EN)", "One\n\nTwo");
    type("Alur babak (EN)", "First\nSecond");
    type("Kategori (EN)", "Elementary | Grades 1–6");
    type("Fakta singkat (EN)", "Fee | IDR 1");

    type("Label jadwal 1", "Daftar");
    type("Label EN jadwal 1", "Sign up");
    type("Mulai jadwal 1", "2026-12-02");
    type("Selesai jadwal 1", "");
    fireEvent.click(screen.getByRole("button", { name: "Tambah jadwal" }));
    type("Label jadwal 3", "Technical meeting");
    type("Label EN jadwal 3", "Technical meeting EN");
    type("Mulai jadwal 3", "2027-01-10");
    type("Selesai jadwal 3", "2027-01-11");
    fireEvent.click(screen.getByRole("button", { name: "Hapus jadwal 2" }));

    type("Warna", "gold");
    type("Ilustrasi", "coding");
    type("Bidang", "Coding, ,Robotik");
    type("Status", "published");
    fireEvent.click(toggle());
    expect(toggle()).toHaveAttribute("aria-checked", "true");
    save();

    await waitFor(() => expect(patches()).toHaveLength(1));
    expect(patches()[0].body).toEqual({
      name: "ESOLIO Baru",
      description: "Filter Toko",
      status: "published",
      short_name: "Einstein 2",
      scope: "national",
      subjects: ["Coding", "Robotik"],
      tone: "gold",
      illustration: "coding",
      logo_key: "",
      official_url: "https://baru.example.test",
      content: {
        id: {
          full_name: "Nama lengkap baru",
          tagline: "Tagline baru",
          levels_label: "Kelas 4–12",
          story: ["A", "B", "C"],
          rounds: ["Satu", "Dua"],
          categories: [
            { name: "SD", detail: "Kelas 1–6" },
            { name: "SMP", detail: "Kelas 7–9" },
            { name: "Tanpa detail", detail: "" },
          ],
          facts: [
            { label: "Biaya", value: "Rp1 | x" },
            { label: "Tempat", value: "Online" },
          ],
        },
        en: {
          full_name: "Full name new",
          tagline: "New tagline",
          levels_label: "Grades 4–12",
          story: ["One", "Two"],
          rounds: ["First", "Second"],
          categories: [{ name: "Elementary", detail: "Grades 1–6" }],
          facts: [{ label: "Fee", value: "IDR 1" }],
        },
      },
      is_visible: true,
      participant_prefix: "ESOL",
      schedule: [
        { label: "Daftar", label_en: "Sign up", starts_on: "2026-12-02", ends_on: null },
        { label: "Technical meeting", label_en: "Technical meeting EN", starts_on: "2027-01-10", ends_on: "2027-01-11" },
      ],
    });
  }, 20_000);

  it("shows the preview chip with the short name, falling back to the name", async () => {
    mockApi();
    renderPage();
    await ready();

    expect(screen.getByTestId("event-preview")).toHaveTextContent("Einstein");
    type("Nama pendek", "");
    expect(screen.getByTestId("event-preview")).toHaveTextContent("ESOLIO");
  });

  it("does not offer the landing preview link before the landing PR merges", async () => {
    mockApi();
    renderPage();
    await ready();
    expect(screen.queryByText(/Lihat di landing/)).not.toBeInTheDocument();
  });
});

describe("event editor: schedule validation", () => {
  it("blocks saving when an end date precedes its start date and names the row", async () => {
    mockApi();
    renderPage();
    await ready();

    type("Selesai jadwal 1", "2026-11-30");

    expect(screen.getByText("Jadwal 1: Tanggal selesai tidak boleh sebelum tanggal mulai.")).toBeInTheDocument();
    save();
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Jadwal baris 1: Tanggal selesai tidak boleh sebelum tanggal mulai."));
    expect(patches()).toHaveLength(0);
  });

  it("blocks a half-filled row but ignores a fully blank one", async () => {
    mockApi();
    renderPage();
    await ready();

    fireEvent.click(screen.getByRole("button", { name: "Tambah jadwal" }));
    save();
    await waitFor(() => expect(patches()).toHaveLength(1));
    expect((patches()[0].body as { schedule: unknown[] }).schedule).toHaveLength(2);

    fireEvent.click(screen.getByRole("button", { name: "Tambah jadwal" }));
    type("Label jadwal 3", "Tanpa tanggal");
    expect(screen.getByText("Jadwal 3: Isi label dan tanggal mulai.")).toBeInTheDocument();
    save();
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Jadwal baris 3: Isi label dan tanggal mulai."));
    expect(patches()).toHaveLength(1);
  });
});

describe("event editor: logo", () => {
  it("uploads through the presigned image endpoint, previews it and saves the key", async () => {
    mockApi();
    const put = vi.fn().mockResolvedValue({ ok: true, status: 200 });
    vi.stubGlobal("fetch", put);
    renderPage();
    await ready();
    expect(screen.queryByAltText("Logo event")).not.toBeInTheDocument();

    const file = new File(["png"], "logo.png", { type: "image/png" });
    fireEvent.change(screen.getByTestId("event-logo-input"), { target: { files: [file] } });

    const preview = await screen.findByAltText("Logo event");
    expect(preview).toHaveAttribute("src", expect.stringContaining("/files/question/logo-abc.png"));
    expect(calls.some((c) => c.path === "/admin/uploads/image?filename=logo.png&content_type=image%2Fpng" && c.method === "POST")).toBe(true);
    expect(put).toHaveBeenCalledWith("https://upload.example.test/put", expect.objectContaining({ method: "PUT", body: file }));

    save();
    await waitFor(() => expect(patches()).toHaveLength(1));
    expect(patches()[0].body).toMatchObject({ logo_key: "question/logo-abc.png" });
  });

  it("removes a stored logo and clears it with an empty logo_key", async () => {
    mockApi({ logo_key: "question/old.png" });
    renderPage();
    await ready();
    expect(screen.getByAltText("Logo event")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Hapus logo" }));
    expect(screen.queryByAltText("Logo event")).not.toBeInTheDocument();
    save();

    await waitFor(() => expect(patches()).toHaveLength(1));
    expect(patches()[0].body).toMatchObject({ logo_key: "" });
  });

  it("keeps the previous logo and reports the failure when the upload is rejected", async () => {
    mockApi();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 403 }));
    renderPage();
    await ready();

    fireEvent.change(screen.getByTestId("event-logo-input"), { target: { files: [new File(["x"], "logo.png", { type: "image/png" })] } });

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Upload gagal: 403"));
    expect(screen.queryByAltText("Logo event")).not.toBeInTheDocument();
  });
});

describe("event editor: visibility", () => {
  const incompleteEn = {
    content: { ...detail.content, en: { ...detail.content!.en!, tagline: "" } },
  };

  it("shows a ! badge on the English tab while incomplete and none when complete", async () => {
    mockApi(incompleteEn);
    const first = renderPage();
    await ready();
    expect(within(screen.getByRole("tab", { name: /English/ })).getByText("!")).toBeInTheDocument();
    first.unmount();

    mockApi();
    renderPage();
    await ready();
    expect(within(screen.getByRole("tab", { name: /English/ })).queryByText("!")).not.toBeInTheDocument();
  });

  it("refuses to turn visibility on while English is incomplete and switches to the English tab", async () => {
    mockApi(incompleteEn);
    renderPage();
    await ready();
    expect(field("Tagline (ID)")).toBeInTheDocument();

    fireEvent.click(toggle());

    expect(toggle()).toHaveAttribute("aria-checked", "false");
    expect(screen.getByRole("tab", { name: /English/ })).toHaveAttribute("aria-selected", "true");
    expect(field("Tagline (EN)")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("Wajib diisi sebelum event bisa tampil");
    expect(patches()).toHaveLength(0);

    type("Tagline (EN)", "Filled in");
    fireEvent.click(toggle());
    expect(toggle()).toHaveAttribute("aria-checked", "true");
  });

  it("treats a schedule row without an English label as incomplete", async () => {
    mockApi();
    renderPage();
    await ready();

    type("Label EN jadwal 2", "");
    fireEvent.click(toggle());

    expect(toggle()).toHaveAttribute("aria-checked", "false");
    expect(screen.getByRole("tab", { name: /English/ })).toHaveAttribute("aria-selected", "true");
  });

  it("switches visibility off when status goes back to draft", async () => {
    mockApi({ is_visible: true });
    renderPage();
    await ready();
    expect(toggle()).toHaveAttribute("aria-checked", "true");

    type("Status", "draft");
    expect(toggle()).toHaveAttribute("aria-checked", "false");
    save();

    await waitFor(() => expect(patches()).toHaveLength(1));
    expect(patches()[0].body).toMatchObject({ status: "draft", is_visible: false });
  });

  it("lists what the API says is missing, in plain words, and opens the English tab for English gaps", async () => {
    mockApi();
    renderPage();
    await ready();
    patchError = new api.ApiError("validation_failed", "event cannot be visible yet, missing: status, en.tagline, schedule[1].label_en", 422, {
      missing: ["status", "en.tagline", "schedule[1].label_en"],
    });

    fireEvent.click(toggle());
    save();

    expect(await screen.findByText(/Belum lengkap: Status Published, Tagline \(English\), Label EN jadwal ke-2\./)).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /English/ })).toHaveAttribute("aria-selected", "true");
  });
});

describe("event editor: participant prefix", () => {
  it("uppercases, strips non-letters and caps at six letters, and sends the cleaned prefix", async () => {
    mockApi({ participant_prefix: null });
    renderPage();
    await ready();

    type("Prefiks nomor peserta", "es-o1lioXY");
    expect(field("Prefiks nomor peserta")).toHaveValue("ESOLIO");
    save();

    await waitFor(() => expect(patches()).toHaveLength(1));
    expect(patches()[0].body).toMatchObject({ participant_prefix: "ESOLIO" });
  });

  it("shows the hint text from the spec", async () => {
    mockApi();
    renderPage();
    await ready();
    expect(
      screen.getByText("Dipakai di depan nomor peserta, mis. ESOL-251012-0007-000123. Mengubahnya mengubah nomor di kartu yang dibuat berikutnya."),
    ).toBeInTheDocument();
  });

  it("shows the duplicate-prefix 409 on the prefix field", async () => {
    mockApi();
    renderPage();
    await ready();
    patchError = new api.ApiError("grouping_prefix_taken", "Prefiks ESOL sudah dipakai event lain", 409);

    save();

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Prefiks ESOL sudah dipakai event lain");
    expect(field("Prefiks nomor peserta")).toHaveAttribute("aria-invalid", "true");
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("shows the API format 422 on the prefix field and clears it on the next keystroke", async () => {
    mockApi();
    renderPage();
    await ready();
    patchError = new api.ApiError("validation_failed", "validation failed: participant_prefix must be 2 to 6 uppercase letters", 422);
    type("Prefiks nomor peserta", "E");

    save();

    expect(await screen.findByRole("alert")).toHaveTextContent("validation failed: participant_prefix must be 2 to 6 uppercase letters");
    type("Prefiks nomor peserta", "ES");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("points at the prefix field when the API lists it as missing for visibility", async () => {
    mockApi({ participant_prefix: null });
    renderPage();
    await ready();
    patchError = new api.ApiError("validation_failed", "event cannot be visible yet, missing: participant_prefix", 422, { missing: ["participant_prefix"] });

    fireEvent.click(toggle());
    save();

    await waitFor(() => expect(screen.getAllByRole("alert").some((a) => a.textContent === "Prefiks nomor peserta wajib diisi.")).toBe(true));
  });

  it("shows the missing panel and opens the English tab when the prefix and an English field are both missing", async () => {
    mockApi({ participant_prefix: null });
    renderPage();
    await ready();
    patchError = new api.ApiError("validation_failed", "event cannot be visible yet, missing: participant_prefix, en.tagline", 422, {
      missing: ["participant_prefix", "en.tagline"],
    });

    fireEvent.click(toggle());
    save();

    expect(await screen.findByText(/Belum lengkap: .*Tagline \(English\)/)).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /English/ })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByText(/event cannot be visible yet/)).not.toBeInTheDocument();
  });
});

describe("event editor: editions and products", () => {
  it("lists exam products as editions with phase and two-event badges, and other products below", async () => {
    mockApi();
    renderPage();
    await ready();

    const editions = within(screen.getByRole("list", { name: "Edisi" }));
    const first = within(editions.getByText("ESOLIO 2026 SD").closest("li")!);
    expect(first.getByText("Dibuka")).toBeInTheDocument();
    expect(first.getByText("ada di dua event")).toBeInTheDocument();
    const second = within(editions.getByText("ESOLIO 2025 SMP").closest("li")!);
    expect(second.getByText("Selesai")).toBeInTheDocument();
    expect(second.queryByText("ada di dua event")).not.toBeInTheDocument();

    const other = within(screen.getByRole("list", { name: "Produk lain" }));
    expect(other.getByText("Kaos ESOLIO")).toBeInTheDocument();
    expect(other.queryByText("SD")).not.toBeInTheDocument();
  });

  it("saves an edition's level with a product PATCH and nothing else", async () => {
    mockApi();
    renderPage();
    await ready();

    const level = within(screen.getByRole("group", { name: "Level ESOLIO 2026 SD" }));
    expect(level.getByRole("button", { name: "SD" })).toHaveAttribute("aria-pressed", "true");
    expect(level.getByLabelText("Kelas dari")).toHaveValue(1);

    fireEvent.click(level.getByRole("button", { name: "SMP" }));
    fireEvent.change(level.getByLabelText("Kelas sampai"), { target: { value: "8" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan level ESOLIO 2026 SD" }));

    await waitFor(() => expect(calls.some((c) => c.path === "/admin/products/p1")).toBe(true));
    const call = calls.find((c) => c.path === "/admin/products/p1")!;
    expect(call.method).toBe("PATCH");
    expect(call.body).toEqual({ stages: ["SD", "SMP"], grade_min: 1, grade_max: 8 });
    expect(patches()).toHaveLength(0);
  });

  it("clears an edition's level with null grades and blocks a reversed range", async () => {
    mockApi();
    renderPage();
    await ready();

    const level = within(screen.getByRole("group", { name: "Level ESOLIO 2026 SD" }));
    fireEvent.change(level.getByLabelText("Kelas sampai"), { target: { value: "0" } });
    expect(screen.getByRole("button", { name: "Simpan level ESOLIO 2026 SD" })).toBeDisabled();

    fireEvent.click(level.getByRole("button", { name: "SD" }));
    fireEvent.change(level.getByLabelText("Kelas dari"), { target: { value: "" } });
    fireEvent.change(level.getByLabelText("Kelas sampai"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan level ESOLIO 2026 SD" }));

    await waitFor(() => expect(calls.some((c) => c.path === "/admin/products/p1")).toBe(true));
    expect(calls.find((c) => c.path === "/admin/products/p1")!.body).toEqual({ stages: [], grade_min: null, grade_max: null });
  });

  it("reorders editions with the arrows and saves the member order with a PUT", async () => {
    mockApi();
    renderPage();
    await ready();

    const editions = within(screen.getByRole("list", { name: "Edisi" }));
    fireEvent.click(editions.getAllByRole("button", { name: "Naikkan urutan" })[1]);
    fireEvent.click(screen.getByRole("button", { name: "Simpan perubahan" }));

    await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.path).toBe("/admin/product-groupings/g1/products");
    expect(put.body).toEqual({ product_ids: ["p2", "p1", "p3"] });
  });

  it("offers a drag handle and both arrows on every row of both lists", async () => {
    mockApi();
    renderPage();
    await ready();

    const editions = within(screen.getByRole("list", { name: "Edisi" }));
    const other = within(screen.getByRole("list", { name: "Produk lain" }));
    expect(editions.getAllByRole("button", { name: "Seret untuk mengurutkan" })).toHaveLength(2);
    expect(editions.getAllByRole("button", { name: "Naikkan urutan" })).toHaveLength(2);
    expect(other.getAllByRole("button", { name: "Seret untuk mengurutkan" })).toHaveLength(1);
    expect(other.getAllByRole("button", { name: "Turunkan urutan" })).toHaveLength(1);
  });

  it("disables moving the first item up and the last item down in each list", async () => {
    mockApi();
    renderPage();
    await ready();

    for (const name of ["Edisi", "Produk lain"]) {
      const list = within(screen.getByRole("list", { name }));
      expect(list.getAllByRole("button", { name: "Naikkan urutan" })[0]).toBeDisabled();
      expect(list.getAllByRole("button", { name: "Turunkan urutan" }).at(-1)).toBeDisabled();
    }
    const editions = within(screen.getByRole("list", { name: "Edisi" }));
    expect(editions.getAllByRole("button", { name: "Naikkan urutan" })[1]).toBeEnabled();
    expect(editions.getAllByRole("button", { name: "Turunkan urutan" })[0]).toBeEnabled();
  });

  it("adds a product from the search and removes another before saving", async () => {
    mockApi();
    renderPage();
    await ready();
    await screen.findByText("Kaos ESOLIO");

    fireEvent.change(screen.getByLabelText("Cari produk untuk ditambahkan"), { target: { value: "buku" } });
    fireEvent.click(await screen.findByRole("button", { name: /Buku Latihan/ }));
    expect(within(screen.getByRole("list", { name: "Produk lain" })).getByText("Buku Latihan")).toBeInTheDocument();

    const kaos = screen.getByText("Kaos ESOLIO").closest("li")!;
    fireEvent.click(within(kaos).getByRole("button", { name: "Keluarkan dari event" }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan perubahan" }));

    await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
    expect(calls.find((c) => c.method === "PUT")!.body).toEqual({ product_ids: ["p1", "p2", "p4"] });
  });

  it("keeps unsaved identity edits when the member list is saved", async () => {
    mockApi();
    renderPage();
    await ready();

    type("Nama event", "Draft nama");
    fireEvent.click(screen.getByRole("button", { name: "Simpan perubahan" }));

    await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
    await waitFor(() => expect(toast.success).toHaveBeenCalled());
    expect(field("Nama event")).toHaveValue("Draft nama");
  });
});

describe("event editor: delete", () => {
  it("offers delete only when the event has no products", async () => {
    mockApi();
    const first = renderPage();
    await ready();
    expect(screen.queryByRole("button", { name: "Hapus event" })).not.toBeInTheDocument();
    first.unmount();

    mockApi({ products: [] });
    renderPage();
    await ready();
    expect(screen.getByRole("button", { name: "Hapus event" })).toBeInTheDocument();
  });

  it("deletes after confirmation and returns to the list", async () => {
    mockApi({ products: [] });
    vi.stubGlobal("confirm", vi.fn(() => true));
    renderPage();
    await ready();

    fireEvent.click(screen.getByRole("button", { name: "Hapus event" }));

    await waitFor(() => expect(push).toHaveBeenCalledWith("/admin/product-groupings"));
    expect(calls.some((c) => c.method === "DELETE" && c.path === "/admin/product-groupings/g1")).toBe(true);
  });

  it("does nothing when the confirmation is declined", async () => {
    mockApi({ products: [] });
    vi.stubGlobal("confirm", vi.fn(() => false));
    renderPage();
    await ready();

    fireEvent.click(screen.getByRole("button", { name: "Hapus event" }));

    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    expect(push).not.toHaveBeenCalled();
  });
});
