import { describe, it, expect, vi, beforeEach, beforeAll } from "vitest";
import { render, screen, within, fireEvent } from "@testing-library/react";
import AdminIndexPage from "./page";

// The page renders the real chart components, and their usePrefersReducedMotion
// hook calls window.matchMedia, which jsdom does not implement. Mirrors the
// stub in components/shell/AppShell.test.tsx.
beforeAll(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  });
});

const replace = vi.fn();
const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace, push }),
}));

let authStore: {
  token: string | null;
  user: { role?: string; name?: string } | null;
} = {
  token: "t",
  user: { role: "super_admin", name: "Super Admin" },
};

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof authStore) => unknown) => selector(authStore),
}));

let meState: {
  data: { role?: string; name?: string } | null;
  isError: boolean;
  isLoading: boolean;
} = { data: null, isError: false, isLoading: false };

vi.mock("@/lib/hooks/auth", async () => {
  const actual = await vi.importActual<typeof import("@/lib/hooks/auth")>(
    "@/lib/hooks/auth"
  );
  return {
    ...actual,
    useMe: ({ enabled }: { enabled?: boolean }) =>
      enabled
        ? meState
        : { data: null, isError: false, isLoading: false },
  };
});

let notifState: {
  items: { id: string; type: string; order_id: string; student_name: string; amount: number; created_at: string; read: boolean }[];
  isLoading: boolean;
  isError: boolean;
} = { items: [], isLoading: false, isError: false };

vi.mock("@/lib/hooks/admin-notifications", () => ({
  useAdminNotifications: () => notifState,
}));

let dashboardState: {
  data: Record<string, unknown> | null;
  isLoading: boolean;
  isError: boolean;
  refetch: ReturnType<typeof vi.fn>;
} = { data: null, isLoading: false, isError: false, refetch: vi.fn() };

vi.mock("@/lib/hooks/admin-dashboard", () => ({
  useAdminDashboard: () => dashboardState,
}));

// Mock only useHasCapability. Do NOT reference useResolvedRole here — that
// rename lands in Plan A, on a different branch. This branch is cut from main,
// where the hook is still useResolvedAdminRole and this page does not use it:
// it resolves role from useAuthStore + useMe, which the preserved harness above
// already mocks. Referencing A's symbol would couple two branches that are
// meant to run in parallel.
vi.mock("@/lib/hooks/use-capability", () => ({
  useHasCapability: () => true,
}));

const sample = {
  period: { from: "2026-07-08", to: "2026-08-06", bucket: "day" },
  kpi: {
    revenue: { value: 48200000, prev: 43000000 },
    order_count: { value: 126, prev: 117 },
    new_students: { value: 84 },
    schools: { value: 23 },
    students_total: { value: 1284 },
    revenue_digital: { value: 30000000, prev: 20000000 },
    revenue_physical: { value: 18200000, prev: 23000000 },
  },
  series: [
    { date: "2026-07-08", revenue: 100, order_count: 1, revenue_digital: 60,
      revenue_physical: 40, new_students: 2, exam_students: 3, buying_students: 1 },
    { date: "2026-07-09", revenue: 200, order_count: 2, revenue_digital: 120,
      revenue_physical: 80, new_students: 1, exam_students: 4, buying_students: 2 },
  ],
  attention: {
    needs_confirm: 7, ready_to_ship: 3, shipment_failed: 2, active_sessions: 14,
    essays_to_grade: 5, essays_exam_id: "11111111-1111-1111-1111-111111111111",
  },
  top_products: [
    { product_id: "p1", name: "Matematika Penalaran UTBK", product_type: "exam",
      is_physical: false, qty_sold: 412, product_revenue: 8200000 },
    { product_id: "p2", name: "Buku Latihan Penalaran", product_type: "book",
      is_physical: true, qty_sold: 206, product_revenue: 5150000 },
  ],
  upcoming_exams: [
    { id: "e1", title: "Try Out UTBK", scheduled_at: "2026-08-12T09:00:00+07:00",
      registrant_count: 412 },
  ],
};


const sampleNotif = {
  id: "n1", type: "order_confirmed", order_id: "ord-9", student_name: "Budi Santoso",
  amount: 150000, created_at: "2026-08-06T10:00:00Z", read: false,
};

const order = (...ids: string[]) => {
  const nodes = ids.map((id) => screen.getByText(id));
  for (let i = 0; i < nodes.length - 1; i++) {
    expect(
      nodes[i].compareDocumentPosition(nodes[i + 1]) & Node.DOCUMENT_POSITION_FOLLOWING,
      `${ids[i]} should come before ${ids[i + 1]}`,
    ).toBeTruthy();
  }
};

describe("AdminIndexPage — v2 dashboard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
    authStore = { token: "t", user: { role: "super_admin", name: "Rizky Saputra" } };
    meState = { data: null, isError: false, isLoading: false };
    notifState = { items: [sampleNotif], isLoading: false, isError: false };
    dashboardState = { data: sample, isLoading: false, isError: false, refetch: vi.fn() };
  });

  it("renders the sections in the v2 order", () => {
    render(<AdminIndexPage />);
    expect(screen.getByTestId("attention-row")).toBeInTheDocument();
    const attn = screen.getByTestId("attention-row");
    const kpi = screen.getByTestId("kpi-row");
    expect(attn.compareDocumentPosition(screen.getByText("Periode")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getByText("Periode").compareDocumentPosition(kpi) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(kpi.compareDocumentPosition(screen.getByText("Tren pendapatan & pesanan")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    order(
      "Tren pendapatan & pesanan", "Aktivitas siswa", "Digital vs Fisik", "Ujian mendatang",
      "Aktivitas pembelian", "Produk terlaris", "Akses Cepat",
    );
  });

  it("greets the admin by first name by Jakarta time of day", () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    const cases: [string, string][] = [
      ["2026-08-12T03:59:00Z", "Selamat pagi, Rizky"], // 10:59 WIB
      ["2026-08-12T04:00:00Z", "Selamat siang, Rizky"], // 11:00
      ["2026-08-12T07:59:00Z", "Selamat siang, Rizky"], // 14:59
      ["2026-08-12T08:00:00Z", "Selamat sore, Rizky"], // 15:00
      ["2026-08-12T10:59:00Z", "Selamat sore, Rizky"], // 17:59
      ["2026-08-12T11:00:00Z", "Selamat malam, Rizky"], // 18:00
    ];
    for (const [iso, text] of cases) {
      vi.setSystemTime(new Date(iso));
      const { unmount } = render(<AdminIndexPage />);
      expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(text);
      unmount();
    }
  });

  it("shows the Jakarta date with the platform-overview suffix", () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-08-12T18:30:00Z")); // 13 Aug 01:30 WIB
    render(<AdminIndexPage />);
    expect(screen.getByText(/13 Agustus 2026 · ringkasan seluruh platform/)).toBeInTheDocument();
  });

  it("puts a Buat ujian button in the header linking to the exam packages", () => {
    render(<AdminIndexPage />);
    const header = screen.getByRole("heading", { level: 1 }).closest("header")!;
    expect(within(header).getByRole("link", { name: /buat ujian/i })).toHaveAttribute("href", "/admin/exam/packages");
  });

  it("renders the attention row: confirm, ship, essays, then failed only above zero", () => {
    render(<AdminIndexPage />);
    const row = within(screen.getByTestId("attention-row"));
    expect(row.getByTestId("attention-needs-confirm")).toHaveTextContent("7");
    expect(row.getByTestId("attention-ready-to-ship")).toHaveTextContent("3");
    expect(row.getByTestId("attention-essays")).toHaveTextContent("5");
    expect(row.getByTestId("attention-shipment-failed")).toHaveTextContent("2");
    expect(row.getAllByTestId(/^attention-/).map((n) => n.dataset.testid)).toEqual([
      "attention-needs-confirm", "attention-ready-to-ship", "attention-essays", "attention-shipment-failed",
    ]);
  });

  it("omits the shipment-failed card at zero", () => {
    dashboardState = { ...dashboardState, data: { ...sample, attention: { ...sample.attention, shipment_failed: 0 } } };
    render(<AdminIndexPage />);
    expect(screen.queryByTestId("attention-shipment-failed")).toBeNull();
  });

  it("keeps the order queue links of the existing attention cards", () => {
    render(<AdminIndexPage />);
    expect(screen.getByTestId("attention-needs-confirm")).toHaveAttribute("href", "/admin/orders?status=pending");
    // ready_to_ship is the paid+processing physical bucket; ?status=paid alone is a bigger, wrong set.
    expect(screen.getByTestId("attention-ready-to-ship")).toHaveAttribute("href", "/admin/orders?queue=ready_to_ship");
    expect(screen.getByTestId("attention-shipment-failed")).toHaveAttribute("href", "/admin/orders?queue=shipment_failed");
  });

  it("links the essay card to the grading tab of the oldest ungraded exam", () => {
    render(<AdminIndexPage />);
    expect(screen.getByTestId("attention-essays")).toHaveAttribute(
      "href",
      "/admin/exam/packages/11111111-1111-1111-1111-111111111111?tab=grading",
    );
  });

  it("shows 0 essays with no link when nothing awaits grading", () => {
    dashboardState = {
      ...dashboardState,
      data: { ...sample, attention: { ...sample.attention, essays_to_grade: 0, essays_exam_id: null } },
    };
    render(<AdminIndexPage />);
    const card = screen.getByTestId("attention-essays");
    expect(card).toHaveTextContent("0");
    expect(card.closest("a")).toBeNull();
  });

  it("shows the sessions chip linking to the monitor only when sessions are running", () => {
    const { unmount } = render(<AdminIndexPage />);
    expect(screen.getByRole("link", { name: "14 sesi berlangsung" })).toHaveAttribute("href", "/admin/exam/monitor");
    unmount();

    dashboardState = { ...dashboardState, data: { ...sample, attention: { ...sample.attention, active_sessions: 0 } } };
    render(<AdminIndexPage />);
    expect(screen.queryByText(/sesi berlangsung/)).toBeNull();
  });

  it("renders a delta with the previous-period label when prev is present", () => {
    render(<AdminIndexPage />);
    // 48.2M vs 43M = +12%
    const kpiRow = within(screen.getByTestId("kpi-row"));
    expect(kpiRow.getByText(/\+12%/)).toBeInTheDocument();
    expect(kpiRow.getAllByText(/vs periode sebelumnya/).length).toBe(2);
  });

  it("renders no delta when prev is absent", () => {
    render(<AdminIndexPage />);
    const kpiRow = within(screen.getByTestId("kpi-row"));
    const newStudents = kpiRow.getByText(/siswa baru/i).closest("div")!;
    expect(newStudents.textContent).not.toMatch(/%/);
  });

  it("renders the revenue mix card instead of the stacked bars", () => {
    render(<AdminIndexPage />);
    expect(screen.getByTestId("mix-digital-share")).toHaveTextContent("62%");
    expect(screen.getByTestId("mix-physical-share")).toHaveTextContent("38%");
  });

  it("links upcoming exams to their package with date and registrants", () => {
    render(<AdminIndexPage />);
    const link = screen.getByRole("link", { name: /Try Out UTBK/ });
    expect(link).toHaveAttribute("href", "/admin/exam/packages/e1");
    expect(link).toHaveTextContent("412");
    expect(link).toHaveTextContent("12 Agu");
  });

  it("renders an empty state when no exams are scheduled", () => {
    dashboardState = { ...dashboardState, data: { ...sample, upcoming_exams: [] } };
    render(<AdminIndexPage />);
    expect(screen.getByText(/tidak ada ujian terjadwal/i)).toBeInTheDocument();
  });

  it("renders the purchase feed with a Live chip", () => {
    render(<AdminIndexPage />);
    const card = screen.getByText("Aktivitas pembelian").closest("[data-slot=card]") as HTMLElement;
    expect(within(card).getByText("Live")).toHaveAttribute("data-variant", "live");
    expect(within(card).getByText(/Budi Santoso/)).toBeInTheDocument();
    expect(card).toHaveTextContent("150.000");
    expect(screen.queryByText("Log Aktivitas")).toBeNull();
  });

  it("shows a failure line in the feed when notifications fail, without breaking the page", () => {
    notifState = { items: [], isLoading: false, isError: true };
    render(<AdminIndexPage />);
    expect(screen.getByText("Gagal memuat notifikasi.")).toBeInTheDocument();
    expect(screen.getByTestId("kpi-row")).toBeInTheDocument();
  });

  it("ranks top products with quantity, revenue and a bar relative to the best seller", () => {
    render(<AdminIndexPage />);
    const rows = screen.getAllByTestId("top-product");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("1");
    expect(rows[0]).toHaveTextContent("Matematika Penalaran UTBK");
    expect(rows[0]).toHaveTextContent("412");
    expect(rows[0]).toHaveTextContent("8.200.000");
    expect(within(rows[0]).getByTestId("top-product-bar")).toHaveStyle({ width: "100%" });
    expect(within(rows[1]).getByTestId("top-product-bar")).toHaveStyle({ width: "50%" });
  });

  it("links the top-products header to the sales summary", () => {
    render(<AdminIndexPage />);
    expect(screen.getByRole("link", { name: "Ringkasan penjualan" })).toHaveAttribute("href", "/admin/store");
  });

  it("renders the D8 quick actions", () => {
    render(<AdminIndexPage />);
    const card = screen.getByText("Akses Cepat").closest("[data-slot=card]") as HTMLElement;
    const hrefs = within(card).getAllByRole("link").map((a) => [a.textContent, a.getAttribute("href")]);
    expect(hrefs).toEqual([
      ["Tambah Produk", "/admin/products"],
      ["Buat ujian", "/admin/exam/packages"],
      ["Kirim pengumuman", "/admin/notifications"],
      ["Buat promo", "/admin/promos"],
    ]);
  });

  it("shows skeletons while loading", () => {
    dashboardState = { data: null, isLoading: true, isError: false, refetch: vi.fn() };
    const { container } = render(<AdminIndexPage />);
    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
    expect(screen.queryByTestId("attention-row")).toBeNull();
  });

  it("shows a failure message and no zeroed KPI values when the dashboard fails to load", () => {
    dashboardState = { data: null, isLoading: false, isError: true, refetch: vi.fn() };
    render(<AdminIndexPage />);
    expect(screen.getByText("Gagal memuat dashboard.")).toBeInTheDocument();
    expect(screen.queryByTestId("kpi-row")).toBeNull();
    expect(screen.queryByText("Rp0")).toBeNull();
  });

  it("retries the dashboard query when the reload button is clicked on failure", () => {
    const refetch = vi.fn();
    dashboardState = { data: null, isLoading: false, isError: true, refetch };
    render(<AdminIndexPage />);
    fireEvent.click(screen.getByRole("button", { name: /muat ulang/i }));
    expect(refetch).toHaveBeenCalled();
  });
});
