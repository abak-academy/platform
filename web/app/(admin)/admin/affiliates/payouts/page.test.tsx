import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import PayoutsPage from "./page";

const h = vi.hoisted(() => ({
  list: vi.fn(),
  process: vi.fn(),
  decide: vi.fn(),
  download: vi.fn(),
  presign: vi.fn(),
  capability: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useAdminAffiliatePayouts: h.list,
  useProcessAffiliatePayout: () => ({ mutate: h.process, isPending: false }),
  useDecideAffiliatePayout: () => ({ mutate: h.decide, isPending: false }),
  downloadAffiliatePayoutsCsv: h.download,
}));
vi.mock("@/lib/hooks/students", () => ({
  usePresignUpload: () => ({ mutateAsync: h.presign, isPending: false }),
}));
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: h.capability }));
vi.mock("sonner", () => ({ toast: h.toast }));

const payout = (id: string, status: string) => ({
  id,
  affiliator_name: "Budi Santoso",
  affiliator_code: "BUDI1234",
  amount: 150000,
  status,
  bank_name: "BCA",
  bank_account_number: "1234567890",
  bank_account_holder: "Budi Santoso",
  requested_at: "2026-10-05T00:00:00Z",
});

describe("AdminAffiliatePayoutsPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => "mockReset" in m && m.mockReset());
    h.toast.success.mockReset();
    h.toast.error.mockReset();
    h.capability.mockReturnValue(true);
    h.list.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { data: [payout("p1", "requested"), payout("p2", "paid")], total: 2 },
    });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 200 })));
  });

  afterEach(() => vi.unstubAllGlobals());

  it("only offers actions on undecided payouts", () => {
    render(<PayoutsPage />);

    expect(screen.getAllByRole("button", { name: "Tandai dibayar" })).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: "Proses" })).toHaveLength(1);
  });

  it("moves a requested payout into processing", () => {
    render(<PayoutsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Proses" }));

    expect(h.process).toHaveBeenCalledWith("p1", expect.anything());
  });

  it("cannot mark paid until a proof is uploaded, then sends the proof key", async () => {
    h.presign.mockResolvedValue({ url: "http://upload.test/x", key: "payment_proof/x.png" });
    render(<PayoutsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Tandai dibayar" }));
    const confirm = await screen.findByRole("button", { name: "Konfirmasi" });
    expect(confirm).toBeDisabled();

    const file = new File(["x"], "bukti.png", { type: "image/png" });
    fireEvent.change(screen.getByTestId("payout-proof-input"), { target: { files: [file] } });
    await screen.findByText("Bukti terunggah");
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));

    expect(h.decide).toHaveBeenCalledWith(
      { id: "p1", decision: "paid", proof_url: "payment_proof/x.png" },
      expect.anything(),
    );
  });

  it("rejects with a reason", async () => {
    render(<PayoutsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Tolak" }));
    fireEvent.change(await screen.findByRole("textbox"), { target: { value: "rekening salah" } });
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));

    await waitFor(() =>
      expect(h.decide).toHaveBeenCalledWith(
        { id: "p1", decision: "reject", reason: "rekening salah" },
        expect.anything(),
      ),
    );
  });

  it("downloads the transfer CSV and reports a failure", async () => {
    h.download.mockRejectedValueOnce(new Error("boom"));
    render(<PayoutsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Unduh CSV transfer" }));

    await waitFor(() => expect(h.toast.error).toHaveBeenCalledWith("Gagal mengunduh CSV."));
    expect(h.download).toHaveBeenCalled();
  });
});
