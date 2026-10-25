import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import StudentsPage from "./students/page";
import CommissionsPage from "./commissions/page";
import LedgerPage from "./ledger/page";

const hooks = vi.hoisted(() => ({
  bindings: vi.fn(),
  commissions: vi.fn(),
  ledger: vi.fn(),
}));

vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateBindings: hooks.bindings,
  useAffiliateCommissions: hooks.commissions,
  useAffiliateLedger: hooks.ledger,
}));

function ok<T>(data: T[], extra: Record<string, unknown> = {}) {
  return { isLoading: false, isError: false, data: { data, page: 1, per_page: 20, total: data.length, ...extra } };
}

describe("affiliate list pages", () => {
  beforeEach(() => {
    hooks.bindings.mockReset();
    hooks.commissions.mockReset();
    hooks.ledger.mockReset();
  });

  it("lists referred students by display name only", () => {
    hooks.bindings.mockReturnValue(
      ok([
        {
          binding_id: "b1",
          name: "Budi S.",
          registered_at: "2026-09-01T00:00:00Z",
          status: "active",
          source: "link",
          started_at: "2026-09-02T00:00:00Z",
          ended_at: "2026-12-02T00:00:00Z",
          total_spend: 250000,
        },
      ]),
    );
    render(<StudentsPage />);

    expect(screen.getByText("Budi S.")).toBeInTheDocument();
    expect(screen.getByText("Tautan referral")).toBeInTheDocument();
    expect(screen.getByText("Aktif")).toBeInTheDocument();
  });

  it("filters commissions by status and returns to the first page", () => {
    hooks.commissions.mockReturnValue(
      ok([
        {
          id: "c1",
          product_type: "book",
          base_amount: 100000,
          percentage: 10,
          amount: 10000,
          status: "pending",
          wait_until: null,
          created_at: "2026-10-01T00:00:00Z",
        },
      ]),
    );
    render(<CommissionsPage />);

    expect(screen.getByText("Buku")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Filter status"), { target: { value: "held" } });
    expect(hooks.commissions).toHaveBeenLastCalledWith({ page: 1, status: "held" });
  });

  it("shows the balance and colours deductions on the ledger", () => {
    hooks.ledger.mockReturnValue(
      ok(
        [
          { id: "l1", entry_type: "commission", amount: 10000, note: null, created_at: "2026-10-01T00:00:00Z" },
          { id: "l2", entry_type: "refund_deduction", amount: -10000, note: "refund", created_at: "2026-10-02T00:00:00Z" },
        ],
        { saldo: 0 },
      ),
    );
    render(<LedgerPage />);

    expect(screen.getByText(/Saldo saat ini/)).toBeInTheDocument();
    expect(screen.getByText("Pengurang refund")).toBeInTheDocument();
  });
});
