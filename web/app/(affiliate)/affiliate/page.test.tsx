import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AffiliateDashboardPage from "./page";
import { ApiError } from "@/lib/api";
import type { AffiliateProfile } from "@/lib/types";

const meState = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const refetch = vi.hoisted(() => vi.fn());
const statsCall = vi.hoisted(() => vi.fn());

vi.mock("@/lib/hooks/auth", () => ({ useMe: () => ({ data: { name: "Budi Santoso" } }) }));
vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateMe: () => meState.value,
  useAffiliateStats: (range: unknown) => ({
    ...(statsCall(range), {}),
    data: {
      registrations: 3,
      registrations_by_source: {},
      total_students: 3,
      active_bindings: 2,
      new_students: 3,
      commission_earned: 90000,
    },
  }),
  useAffiliatePromos: () => ({ data: { data: [] } }),
  useAffiliateLedger: () => ({ data: { saldo: 150000, data: [], page: 1, per_page: 1, total: 0 } }),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const profile: AffiliateProfile = {
  code: "BUDI10",
  referral_link: "/r/BUDI10",
  level: "silver",
  status: "active",
  warn_quarters: 0,
  bank_name: null,
  bank_account_number: null,
  bank_account_holder: null,
  terms_version: "2026-10",
  terms_accepted_at: "2026-10-01T00:00:00Z",
  level_progress: { quarter: "2026-Q4", net_revenue: 1200000, next_level: "gold", rise_threshold: 15000000 },
};

describe("AffiliateDashboardPage", () => {
  beforeEach(() => {
    refetch.mockClear();
  });

  it("shows the code, link, balance and stats for an active affiliator", () => {
    meState.value = { isLoading: false, isError: false, data: profile, refetch };
    render(<AffiliateDashboardPage />);

    expect(screen.getByText("Halo, Budi Santoso")).toBeInTheDocument();
    expect(screen.getAllByText("BUDI10").length).toBeGreaterThan(0);
    expect(screen.getByText(/90\.000/)).toBeInTheDocument();
    expect(screen.getByText(/Kurang .*13\.800\.000.* lagi untuk naik ke Gold/)).toBeInTheDocument();
    expect(screen.getByText(`${window.location.origin}/r/BUDI10`)).toBeInTheDocument();
    expect(screen.getByText(/150\.000/)).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("asks for the last 30 days first and re-queries when the period changes", () => {
    meState.value = { isLoading: false, isError: false, data: profile, refetch };
    render(<AffiliateDashboardPage />);

    expect(statsCall.mock.calls[0][0]).toEqual({ from: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/) });
    fireEvent.click(screen.getByRole("button", { name: "7 hari" }));
    expect(statsCall.mock.lastCall?.[0]).toEqual({ from: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/) });
    expect(statsCall.mock.lastCall?.[0]).not.toEqual(statsCall.mock.calls[0][0]);
  });

  it("explains a pending application and hides the link and copy action", () => {
    meState.value = {
      isLoading: false,
      isError: false,
      data: { ...profile, status: "pending_approval", code: "", referral_link: "" },
      refetch,
    };
    render(<AffiliateDashboardPage />);

    expect(screen.getByRole("status")).toHaveTextContent(/sedang ditinjau/i);
    expect(screen.getAllByText("Belum ada kode").length).toBeGreaterThan(0);
    expect(screen.queryByRole("button", { name: /salin tautan/i })).toBeNull();
  });

  it("shows the rejection reason", () => {
    meState.value = {
      isLoading: false,
      isError: false,
      data: { ...profile, status: "rejected", code: "", referral_link: "", rejection_reason: "data tidak lengkap" },
      refetch,
    };
    render(<AffiliateDashboardPage />);

    expect(screen.getByRole("status")).toHaveTextContent("Alasan: data tidak lengkap");
  });

  it("says the program is unavailable on a 404", () => {
    meState.value = {
      isLoading: false,
      isError: true,
      error: new ApiError("affiliate_not_found", "not found", 404),
      data: undefined,
      refetch,
    };
    render(<AffiliateDashboardPage />);

    expect(screen.getByText(/belum tersedia untuk akunmu/i)).toBeInTheDocument();
  });

  it("offers a retry on other errors", () => {
    meState.value = { isLoading: false, isError: true, error: new Error("boom"), data: undefined, refetch };
    render(<AffiliateDashboardPage />);

    fireEvent.click(screen.getByRole("button", { name: /coba lagi/i }));
    expect(refetch).toHaveBeenCalled();
  });

  it("shows a loading state", () => {
    meState.value = { isLoading: true, isError: false, data: undefined, refetch };
    render(<AffiliateDashboardPage />);

    expect(screen.getByRole("status", { name: "Memuat…" })).toBeInTheDocument();
  });
});
