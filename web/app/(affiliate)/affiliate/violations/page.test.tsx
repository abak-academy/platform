import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import ViolationsPage from "./page";

const h = vi.hoisted(() => ({
  violations: vi.fn(),
  adjustments: vi.fn(),
  appeal: vi.fn(),
  replace: vi.fn(),
  search: { value: "" },
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: h.replace }),
  usePathname: () => "/affiliate/violations",
  useSearchParams: () => new URLSearchParams(h.search.value),
}));
vi.mock("@/lib/hooks/affiliate", () => ({
  useAffiliateViolations: h.violations,
  useAffiliateAdjustments: h.adjustments,
  useAppealViolation: () => ({ mutate: h.appeal, isPending: false }),
}));
vi.mock("sonner", () => ({ toast: h.toast }));

const page = (data: unknown[]) => ({ isLoading: false, isError: false, data: { data, page: 1, per_page: 20, total: data.length } });
const violation = {
  id: "v1",
  category: "transaction_fraud",
  description: "Pembelian atas akun sendiri",
  sanction: "warning",
  suspension_until: null,
  status: "open",
  decided_at: "2026-10-01T00:00:00Z",
  appeal_status: "none",
};

describe("AffiliateViolationsPage", () => {
  beforeEach(() => {
    h.appeal.mockReset();
    h.replace.mockReset();
    h.toast.error.mockReset();
    h.search.value = "";
    h.violations.mockReturnValue(page([violation]));
    h.adjustments.mockReturnValue(
      page([
        { id: "a1", category: "bonus", amount: 50000, reason: "Bonus kampanye", applied: true, created_at: "2026-10-02T00:00:00Z" },
        { id: "a2", category: "correction", amount: -20000, reason: "Koreksi", applied: false, created_at: "2026-10-03T00:00:00Z" },
      ]),
    );
  });

  it("explains each tab in plain words and starts on sanctions", () => {
    render(<ViolationsPage />);

    expect(screen.getByText(/Sanksi dicatat admin bila ada pelanggaran/)).toBeInTheDocument();
    expect(screen.getByText("Pembelian atas akun sendiri")).toBeInTheDocument();
    expect(screen.getByText("Peringatan")).toBeInTheDocument();
    expect(screen.getByText("Masih berlaku")).toBeInTheDocument();
  });

  it("switches to adjustments through the URL, and lists them with their effective state", () => {
    fireEvent.mouseDown(render(<ViolationsPage />).getByRole("tab", { name: "Penyesuaian saldo" }), { button: 0 });
    expect(h.replace).toHaveBeenCalledWith("/affiliate/violations?tab=adjustments", { scroll: false });
  });

  it("shows adjustments with whether they have taken effect", () => {
    h.search.value = "tab=adjustments";
    render(<ViolationsPage />);

    expect(screen.getByText(/Penyesuaian adalah penambahan atau pengurangan saldo/)).toBeInTheDocument();
    expect(screen.getByText("Bonus kampanye")).toBeInTheDocument();
    expect(screen.getByText("Berlaku")).toBeInTheDocument();
    expect(screen.getByText("Menunggu persetujuan")).toBeInTheDocument();
  });

  it("passes the filters of the active tab to its own query", () => {
    h.search.value = "q=spam&type=warning&status=open&page=2";
    render(<ViolationsPage />);

    expect(h.violations).toHaveBeenCalledWith({ page: 2, per_page: 20, q: "spam", type: "warning", status: "open", from: "", to: "" });
  });

  it("requires appeal text before sending", () => {
    render(<ViolationsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Ajukan banding" }));
    fireEvent.click(screen.getByRole("button", { name: "Kirim banding" }));

    expect(h.appeal).not.toHaveBeenCalled();
    expect(h.toast.error).toHaveBeenCalledWith("Alasan banding wajib diisi");
  });

  it("submits the trimmed appeal for the right sanction", () => {
    render(<ViolationsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Ajukan banding" }));
    fireEvent.change(screen.getByPlaceholderText("Jelaskan alasan bandingmu"), { target: { value: "  saya keberatan " } });
    fireEvent.click(screen.getByRole("button", { name: "Kirim banding" }));

    expect(h.appeal).toHaveBeenCalledWith({ id: "v1", text: "saya keberatan" }, expect.anything());
  });

  it("shows the appeal outcome instead of the button, and no button on a closed sanction", () => {
    h.violations.mockReturnValue(
      page([
        { ...violation, id: "v2", appeal_status: "pending" },
        { ...violation, id: "v3", status: "resolved", appeal_status: "none" },
      ]),
    );
    render(<ViolationsPage />);

    expect(screen.getByText("Menunggu keputusan")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Ajukan banding" })).toBeNull();
  });
});
