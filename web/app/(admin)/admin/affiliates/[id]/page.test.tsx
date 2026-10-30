import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { Suspense } from "react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import DetailPage from "./page";
import { ApiError } from "@/lib/api";

const h = vi.hoisted(() => ({
  detail: vi.fn(),
  capability: vi.fn(),
  status: vi.fn(),
  level: vi.fn(),
  adjust: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useAdminAffiliate: h.detail,
  useChangeAffiliateStatus: () => ({ mutate: h.status, isPending: false }),
  useChangeAffiliateLevel: () => ({ mutate: h.level, isPending: false }),
  useCreateAffiliateAdjustment: () => ({ mutate: h.adjust, isPending: false }),
}));
vi.mock("@/components/affiliate/AdminCommissionsPanel", () => ({
  AdminCommissionsPanel: ({ affiliatorId }: { affiliatorId: string }) => <div>orders-panel-{affiliatorId}</div>,
}));
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: h.capability }));
vi.mock("sonner", () => ({ toast: h.toast }));

const detail = {
  affiliate: { id: "a1", code: "BUDI1234", status: "active", level: "silver" },
  name: "Budi Santoso",
  email: "budi@example.com",
  bindings: {
    active: 2,
    ended: 1,
    data: [
      { binding_id: "b1", name: "Sari W.", status: "active", started_at: "2026-09-01T00:00:00Z", expires_at: null, total_spend: 100000 },
    ],
  },
  commissions: { counts: { paid: 1 }, total_amount: 50000 },
  saldo: 30000,
};

async function renderPage() {
  await act(async () => {
    render(
      <Suspense fallback={null}>
        <DetailPage params={Promise.resolve({ id: "a1" })} />
      </Suspense>,
    );
  });
}

describe("AdminAffiliateDetailPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => "mockReset" in m && m.mockReset());
    h.toast.success.mockReset();
    h.toast.error.mockReset();
    h.capability.mockReturnValue(true);
    h.detail.mockReturnValue({ isLoading: false, isError: false, data: detail });
  });

  it("shows the profile summary and privacy-safe student names", async () => {
    await renderPage();

    expect(screen.getByRole("heading", { name: "Budi Santoso" })).toBeInTheDocument();
    expect(screen.getByText("Sari W.")).toBeInTheDocument();
    expect(screen.getByText("2 / 1")).toBeInTheDocument();
  });

  it("is blocked without the affiliates capability", async () => {
    h.capability.mockReturnValue(false);
    await renderPage();

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });

  it("says so when the affiliate does not exist", async () => {
    h.detail.mockReturnValue({
      isLoading: false,
      isError: true,
      data: undefined,
      error: new ApiError("affiliate_not_found", "nf", 404),
    });
    await renderPage();

    expect(screen.getByText("Affiliator tidak ditemukan.")).toBeInTheDocument();
  });

  it("requires a reason before an adjustment and sends the signed amount", async () => {
    await renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Penyesuaian saldo" }));
    fireEvent.click(await screen.findByRole("radio", { name: "Pengurangan" }));
    fireEvent.change(screen.getByLabelText(/Jumlah/), { target: { value: "-2a0.000" } });
    expect(screen.getByLabelText(/Jumlah/)).toHaveValue("20000");
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));
    expect(h.adjust).not.toHaveBeenCalled();
    expect(h.toast.error).toHaveBeenCalledWith("Alasan wajib diisi");

    fireEvent.change(screen.getByLabelText("Alasan"), { target: { value: "koreksi salah hitung" } });
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));

    await waitFor(() =>
      expect(h.adjust).toHaveBeenCalledWith(
        { id: "a1", category: "correction", amount: -20000, reason: "koreksi salah hitung" },
        expect.anything(),
      ),
    );
  });

  it("suspends with a reason only, because a suspension has no end date", async () => {
    await renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Ubah status" }));
    fireEvent.click(await screen.findByRole("radio", { name: "Ditangguhkan" }));
    expect(screen.queryByLabelText("Lama (hari)")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));

    expect(h.status).not.toHaveBeenCalled();
    expect(h.toast.error).toHaveBeenCalledWith("Alasan wajib diisi");

    fireEvent.change(screen.getByLabelText("Alasan"), { target: { value: "pelanggaran" } });
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));

    expect(h.status).toHaveBeenCalledWith({ id: "a1", status: "suspended", reason: "pelanggaran" }, expect.anything());
  });

  it("changes the level without a lock date", async () => {
    await renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Ubah level" }));
    expect(screen.queryByLabelText(/Kunci/)).toBeNull();
    fireEvent.click(await screen.findByRole("radio", { name: "Gold" }));
    fireEvent.change(screen.getByLabelText("Alasan"), { target: { value: "mitra utama" } });
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));

    expect(h.level).toHaveBeenCalledWith({ id: "a1", level: "gold", reason: "mitra utama" }, expect.anything());
  });

  it("no longer offers to freeze the balance", async () => {
    await renderPage();

    expect(screen.queryByRole("button", { name: "Bekukan saldo" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Buka pembekuan" })).toBeNull();
  });
});
