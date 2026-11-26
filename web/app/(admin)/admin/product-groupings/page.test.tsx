import { beforeAll, beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "@/lib/api";
import type { AdminProductGrouping } from "@/lib/types";
import ProductGroupingsPage from "./page";

const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));
const toastError = vi.fn();
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: (m: string) => toastError(m) } }));

beforeAll(() => {
  globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} };
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

const base: Pick<AdminProductGrouping, "subjects" | "tone" | "illustration" | "logo_key" | "official_url" | "participant_prefix" | "description"> = {
  subjects: [],
  tone: "brand",
  illustration: "trophy",
  logo_key: null,
  official_url: null,
  participant_prefix: "ESOL",
  description: "",
};

const events: AdminProductGrouping[] = [
  {
    ...base,
    id: "g1", name: "ESOLIO", slug: "esolio", short_name: null, scope: "international", status: "published", display_order: 0, is_visible: true,
    content: { id: { tagline: "Olimpiade sains", levels_label: "Kelas 1–12" } },
    edition_count: 3, open_edition_count: 2,
    next_milestone: { label: "Babak Regional", starts_on: "2026-12-01", ends_on: "2026-12-05", state: "ongoing" },
  },
  {
    ...base,
    id: "g2", name: "KoSSMI", slug: "kossmi", short_name: null, scope: "national", status: "draft", display_order: 1, is_visible: false,
    content: { id: { tagline: "Sains nasional" } },
    edition_count: 1, open_edition_count: 0,
    next_milestone: { label: "Pendaftaran", starts_on: "2027-01-10", ends_on: null, state: "upcoming" },
  },
  {
    ...base,
    id: "g3", name: "IKLC", slug: "iklc", short_name: "Kangaroo", scope: "national", status: "published", display_order: 2, is_visible: false,
    content: null,
    edition_count: 5, open_edition_count: 1,
    next_milestone: null,
  },
];

let calls: { path: string; init?: RequestInit }[];

function mockApi(list = events) {
  calls = [];
  let current = list;
  vi.spyOn(api, "authFetch").mockImplementation((async (path: string, init?: RequestInit) => {
    calls.push({ path, init });
    if (path === "/admin/product-groupings" && !init) return { data: current };
    if (path === "/admin/product-groupings/order") {
      const ids: string[] = JSON.parse(init?.body as string).grouping_ids;
      current = ids.map((id, i) => ({ ...current.find((g) => g.id === id)!, display_order: i }));
      return {};
    }
    if (path === "/admin/product-groupings" && init?.method === "POST") return { ...events[0], id: "new1" };
    return {};
  }) as typeof api.authFetch);
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <ProductGroupingsPage />
    </QueryClientProvider>,
  );
}

const cardNames = () => screen.getAllByRole("article").map((a) => a.getAttribute("aria-label"));
const pick = async (field: string, name: string) => {
  await userEvent.click(screen.getByRole("combobox", { name: field }));
  await userEvent.click(await screen.findByRole("option", { name }));
};

beforeEach(() => {
  push.mockReset();
  toastError.mockReset();
});
afterEach(() => vi.restoreAllMocks());

describe("event list", () => {
  it("renders one card per event with tagline, counts and the next milestone with an ongoing badge", async () => {
    mockApi();
    renderPage();

    await screen.findByRole("article", { name: "ESOLIO" });
    expect(cardNames()).toEqual(["ESOLIO", "KoSSMI", "IKLC"]);
    expect(screen.getByText("3 event")).toBeInTheDocument();

    const esolio = within(screen.getByRole("article", { name: "ESOLIO" }));
    expect(esolio.getByText("Olimpiade sains")).toBeInTheDocument();
    expect(esolio.getByText("Internasional · Kelas 1–12")).toBeInTheDocument();
    expect(esolio.getByText("Babak Regional")).toBeInTheDocument();
    expect(esolio.getByText("Berlangsung")).toBeInTheDocument();
    expect(esolio.getByText("edisi").parentElement).toHaveTextContent("3 edisi");
    expect(esolio.getByText("dibuka").parentElement).toHaveTextContent("2 dibuka");
    expect(esolio.getByRole("link", { name: "Kelola" })).toHaveAttribute("href", "/admin/product-groupings/g1");

    const kossmi = within(screen.getByRole("article", { name: "KoSSMI" }));
    expect(kossmi.queryByText("Berlangsung")).not.toBeInTheDocument();
    expect(within(screen.getByRole("article", { name: "IKLC" })).getByText("Jadwal belum diisi")).toBeInTheDocument();
  });

  it("filters by landing, status, scope and schedule and updates the count", async () => {
    mockApi();
    renderPage();
    await screen.findByRole("article", { name: "ESOLIO" });

    fireEvent.click(screen.getByRole("button", { name: /^Filter/ }));

    await pick("Landing page", "Tampil");
    expect(cardNames()).toEqual(["ESOLIO"]);
    expect(screen.getAllByText("1 event").length).toBeGreaterThan(0);
    await pick("Landing page", "Disembunyikan");
    expect(cardNames()).toEqual(["KoSSMI", "IKLC"]);
    await pick("Landing page", "Semua");

    await pick("Status", "Draft");
    expect(cardNames()).toEqual(["KoSSMI"]);
    await pick("Status", "Semua");

    await pick("Tingkat", "Nasional");
    expect(cardNames()).toEqual(["KoSSMI", "IKLC"]);
    await pick("Tingkat", "Semua");

    await pick("Jadwal", "Sedang berlangsung");
    expect(cardNames()).toEqual(["ESOLIO"]);
    await pick("Jadwal", "Akan datang");
    expect(cardNames()).toEqual(["KoSSMI"]);
    await pick("Jadwal", "Belum diisi");
    expect(cardNames()).toEqual(["IKLC"]);
  });

  it("searches the event name and short name", async () => {
    mockApi();
    renderPage();
    await screen.findByRole("article", { name: "ESOLIO" });

    fireEvent.change(screen.getByRole("searchbox", { name: "Cari nama event" }), { target: { value: "kanga" } });
    expect(cardNames()).toEqual(["IKLC"]);
  });

  it("sorts by name and by edition count, and hides reorder controls outside landing order", async () => {
    mockApi();
    renderPage();
    await screen.findByRole("article", { name: "ESOLIO" });
    expect(screen.getAllByRole("button", { name: "Naikkan urutan" })).toHaveLength(3);

    const user = userEvent.setup();
    await user.click(screen.getByRole("combobox", { name: "Urutkan" }));
    await user.click(await screen.findByRole("option", { name: "Edisi terbanyak" }));
    expect(cardNames()).toEqual(["IKLC", "ESOLIO", "KoSSMI"]);
    expect(screen.queryByRole("button", { name: "Naikkan urutan" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("combobox", { name: "Urutkan" }));
    await user.click(await screen.findByRole("option", { name: "Nama A–Z" }));
    expect(cardNames()).toEqual(["ESOLIO", "IKLC", "KoSSMI"]);
  });

  it("reorders with the arrow buttons and sends the full id order", async () => {
    mockApi();
    renderPage();
    await screen.findByRole("article", { name: "ESOLIO" });

    fireEvent.click(within(screen.getByRole("article", { name: "KoSSMI" })).getByRole("button", { name: "Naikkan urutan" }));

    await waitFor(() => expect(calls.some((c) => c.path === "/admin/product-groupings/order")).toBe(true));
    const call = calls.find((c) => c.path === "/admin/product-groupings/order")!;
    expect(call.init?.method).toBe("PATCH");
    expect(JSON.parse(call.init?.body as string)).toEqual({ grouping_ids: ["g2", "g1", "g3"] });
    await waitFor(() => expect(cardNames()).toEqual(["KoSSMI", "ESOLIO", "IKLC"]));
  });

  it("disables moving the first card up and the last card down", async () => {
    mockApi();
    renderPage();
    await screen.findByRole("article", { name: "ESOLIO" });

    expect(within(screen.getByRole("article", { name: "ESOLIO" })).getByRole("button", { name: "Naikkan urutan" })).toBeDisabled();
    expect(within(screen.getByRole("article", { name: "IKLC" })).getByRole("button", { name: "Turunkan urutan" })).toBeDisabled();
  });

  it("toggles landing visibility with a PATCH of is_visible only", async () => {
    mockApi();
    renderPage();
    await screen.findByRole("article", { name: "ESOLIO" });

    fireEvent.click(screen.getByRole("switch", { name: "Tampilkan KoSSMI di landing" }));

    await waitFor(() => expect(calls.some((c) => c.path === "/admin/product-groupings/g2")).toBe(true));
    const call = calls.find((c) => c.path === "/admin/product-groupings/g2")!;
    expect(call.init?.method).toBe("PATCH");
    expect(JSON.parse(call.init?.body as string)).toEqual({ is_visible: true });
  });

  it("names what is missing when the API refuses to show an incomplete event", async () => {
    mockApi();
    renderPage();
    await screen.findByRole("article", { name: "ESOLIO" });
    vi.mocked(api.authFetch).mockImplementation((async (path: string, init?: RequestInit) => {
      if (init?.method === "PATCH") {
        throw new api.ApiError("validation_failed", "event cannot be visible yet", 422, { missing: ["en.tagline", "schedule[1].label_en"] });
      }
      return { data: events };
    }) as typeof api.authFetch);

    fireEvent.click(screen.getByRole("switch", { name: "Tampilkan KoSSMI di landing" }));

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(toastError.mock.calls[0][0]).toContain("Tagline (English)");
    expect(toastError.mock.calls[0][0]).toContain("Label EN jadwal ke-2");
  });

  it("creates an event from the dialog and opens its editor", async () => {
    mockApi();
    renderPage();
    await screen.findByRole("article", { name: "ESOLIO" });

    fireEvent.click(screen.getByRole("button", { name: "Tambah event" }));
    fireEvent.change(screen.getByLabelText("Nama event"), { target: { value: "  HIPPO  " } });
    fireEvent.change(screen.getByLabelText("Nama pendek (opsional)"), { target: { value: "Hip" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(push).toHaveBeenCalledWith("/admin/product-groupings/new1"));
    const post = calls.find((c) => c.init?.method === "POST")!;
    expect(JSON.parse(post.init?.body as string)).toEqual({ name: "HIPPO", short_name: "Hip" });
  });

  it("shows the empty state when there are no events", async () => {
    mockApi([]);
    renderPage();
    expect(await screen.findByText("Belum ada event.")).toBeInTheDocument();
  });
});
