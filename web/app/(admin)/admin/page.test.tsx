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

// Shared state for the audit-log panel — unchanged from the previous page.
let auditState: {
  data: { id: number; actor_name?: string | null; actor_id?: string | null; actor_email?: string | null; target_type: string; target_id: string; action: string; created_at: string }[];
  isLoading: boolean;
  isError: boolean;
  refetch: ReturnType<typeof vi.fn>;
} = {
  data: [],
  isLoading: false,
  isError: false,
  refetch: vi.fn(),
};

vi.mock("@/lib/hooks/admin-audit", () => ({
  useAdminAuditLog: () => auditState,
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
  },
  series: [
    { date: "2026-07-08", revenue: 100, order_count: 1, revenue_digital: 60,
      revenue_physical: 40, new_students: 2, exam_students: 3, buying_students: 1 },
    { date: "2026-07-09", revenue: 200, order_count: 2, revenue_digital: 120,
      revenue_physical: 80, new_students: 1, exam_students: 4, buying_students: 2 },
  ],
  attention: { needs_confirm: 7, ready_to_ship: 3, shipment_failed: 2, active_sessions: 14 },
  top_products: [
    { product_id: "p1", name: "Try Out UTBK", product_type: "exam",
      is_physical: false, qty_sold: 412, product_revenue: 8200000 },
  ],
  upcoming_exams: [
    { id: "e1", title: "Try Out UTBK", scheduled_at: "2026-08-12T09:00:00+07:00",
      registrant_count: 412 },
  ],
};

describe("AdminIndexPage — reworked dashboard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    authStore = { token: "t", user: { role: "super_admin", name: "Super Admin" } };
    meState = { data: null, isError: false, isLoading: false };
    auditState = { data: [], isLoading: false, isError: false, refetch: vi.fn() };
    dashboardState = { data: sample, isLoading: false, isError: false, refetch: vi.fn() };
  });

  it("no longer renders the dead exam-session card", () => {
    render(<AdminIndexPage />);
    expect(screen.queryByText(/tidak tersedia/i)).toBeNull();
  });

  it("renders the live active-session count", () => {
    render(<AdminIndexPage />);
    expect(screen.getByText(/14/)).toBeInTheDocument();
  });

  it("renders a delta when prev is present", () => {
    render(<AdminIndexPage />);
    // 48.2M vs 43M = +12%
    expect(screen.getByText(/12%/)).toBeInTheDocument();
  });

  it("renders no delta when prev is absent", () => {
    render(<AdminIndexPage />);
    // Scoped to the KPI row, not the whole page: the students chart's own
    // subtitle ("Siswa baru, ikut ujian, dan belanja") also contains "siswa
    // baru" as a substring, so an unscoped query would find two matches.
    const kpiRow = within(screen.getByTestId("kpi-row"));
    const newStudents = kpiRow.getByText(/siswa baru/i).closest("div")!;
    expect(newStudents.textContent).not.toMatch(/%/);
  });

  it("turns the shipment-failed card red only above zero", () => {
    const { rerender } = render(<AdminIndexPage />);
    expect(screen.getByTestId("attention-shipment-failed").dataset.accent).toBe("error");

    dashboardState = {
      ...dashboardState,
      data: { ...sample, attention: { ...sample.attention, shipment_failed: 0 } },
    };
    rerender(<AdminIndexPage />);
    expect(screen.getByTestId("attention-shipment-failed").dataset.accent).not.toBe("error");
  });

  it("links each attention card to its filtered queue", () => {
    render(<AdminIndexPage />);
    expect(screen.getByTestId("attention-needs-confirm").closest("a"))
      .toHaveAttribute("href", "/admin/orders?status=pending");
    expect(screen.getByTestId("attention-shipment-failed").closest("a"))
      .toHaveAttribute("href", "/admin/orders?queue=shipment_failed");
  });

  // The card's count is attention.ready_to_ship — the ready_to_ship bucket
  // (paid + processing, physical item only). ?status=paid alone is a bigger,
  // wrong set — see admin_order.go's OrderFilter.ReadyToShip.
  it("links the ready-to-ship card to the ready_to_ship queue, not status=paid", () => {
    render(<AdminIndexPage />);
    expect(screen.getByTestId("attention-ready-to-ship").closest("a"))
      .toHaveAttribute("href", "/admin/orders?queue=ready_to_ship");
  });

  it("renders upcoming exams with registrant counts", () => {
    render(<AdminIndexPage />);
    expect(screen.getByText("Try Out UTBK")).toBeInTheDocument();
    expect(screen.getByText(/412/)).toBeInTheDocument();
  });

  it("renders an empty state when no exams are scheduled", () => {
    dashboardState = { ...dashboardState, data: { ...sample, upcoming_exams: [] } };
    render(<AdminIndexPage />);
    expect(screen.getByText(/tidak ada ujian terjadwal/i)).toBeInTheDocument();
  });

  it("still renders the quick actions", () => {
    render(<AdminIndexPage />);
    expect(screen.getByText(/akses cepat/i)).toBeInTheDocument();
  });

  it("shows skeletons while loading", () => {
    dashboardState = { data: null, isLoading: true, isError: false, refetch: vi.fn() };
    const { container } = render(<AdminIndexPage />);
    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
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
