import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import ViolationsPage from "./page";

const h = vi.hoisted(() => ({
  violations: vi.fn(),
  adjustments: vi.fn(),
  appeal: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateViolations: h.violations,
  useAffiliateAdjustments: h.adjustments,
  useAppealViolation: () => ({ mutate: h.appeal, isPending: false }),
}));

vi.mock("sonner", () => ({ toast: h.toast }));

function page<T>(data: T[]) {
  return { isLoading: false, isError: false, data: { data, page: 1, per_page: 20, total: data.length } };
}

const violation = {
  id: "v1",
  category: "self_purchase",
  description: "Pembelian atas akun sendiri",
  sanction: "warning",
  suspension_until: null,
  decided_at: "2026-10-01T00:00:00Z",
  appeal_status: "none",
};

describe("AffiliateViolationsPage", () => {
  beforeEach(() => {
    h.appeal.mockReset();
    h.toast.error.mockReset();
    h.adjustments.mockReturnValue(
      page([
        { id: "a1", category: "bonus", amount: 50000, reason: "Bonus kampanye", applied: true, created_at: "2026-10-02T00:00:00Z" },
        { id: "a2", category: "correction", amount: -20000, reason: "Koreksi", applied: false, created_at: "2026-10-03T00:00:00Z" },
      ]),
    );
  });

  it("lists sanctions and adjustments with their effective state", () => {
    h.violations.mockReturnValue(page([violation]));
    render(<ViolationsPage />);

    expect(screen.getByText("Peringatan")).toBeInTheDocument();
    expect(screen.getByText("Bonus kampanye")).toBeInTheDocument();
    expect(screen.getByText("Berlaku")).toBeInTheDocument();
    expect(screen.getByText("Menunggu persetujuan")).toBeInTheDocument();
  });

  it("requires appeal text before sending", () => {
    h.violations.mockReturnValue(page([violation]));
    render(<ViolationsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Ajukan banding" }));
    fireEvent.click(screen.getByRole("button", { name: "Kirim banding" }));

    expect(h.appeal).not.toHaveBeenCalled();
    expect(h.toast.error).toHaveBeenCalledWith("Alasan banding wajib diisi");
  });

  it("submits the trimmed appeal for the right violation", () => {
    h.violations.mockReturnValue(page([violation]));
    render(<ViolationsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Ajukan banding" }));
    fireEvent.change(screen.getByPlaceholderText("Jelaskan alasan bandingmu"), { target: { value: "  saya keberatan " } });
    fireEvent.click(screen.getByRole("button", { name: "Kirim banding" }));

    expect(h.appeal).toHaveBeenCalledWith({ id: "v1", text: "saya keberatan" }, expect.anything());
  });

  it("shows the appeal status instead of the button once an appeal exists", () => {
    h.violations.mockReturnValue(page([{ ...violation, appeal_status: "pending" }]));
    render(<ViolationsPage />);

    expect(screen.queryByRole("button", { name: "Ajukan banding" })).toBeNull();
    expect(screen.getAllByText("Menunggu").length).toBeGreaterThan(0);
  });
});
