import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { PromoModal } from "./PromoModal";
import type { PromoCode } from "@/lib/types";

const mockOnSubmit = vi.fn();
const mockOnOpenChange = vi.fn();
const capability = vi.hoisted(() => ({ value: false }));

vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: () => capability.value }));
vi.mock("@/components/admin/PartnerPicker", () => ({
  PartnerPicker: ({ value, onChange }: { value: string[]; onChange: (ids: string[]) => void }) => (
    <button type="button" onClick={() => onChange([...value, "mitra-1"])}>
      pilih mitra ({value.join(",")})
    </button>
  ),
}));

describe("PromoModal", () => {
  beforeEach(() => {
    mockOnSubmit.mockReset();
    mockOnOpenChange.mockReset();
    capability.value = false;
  });

  it("renders create modal with empty fields", () => {
    render(
      <PromoModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByRole("dialog", { name: /buat kode promo/i })).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/contoh: diskon10/i)).toHaveValue("");
    expect(screen.getByLabelText(/jenis diskon/i)).toHaveValue("percent");
  });

  it("renders edit modal prefilled with promo data", () => {
    const promo: PromoCode = {
      id: "promo-1",
      code: "DISKON10",
      discount_percent: 10,
      max_discount_amount: 50000,
      min_order_amount: 100000,
      max_uses: 100,
      used_count: 3,
      expires_at: "2026-12-31T00:00:00Z",
    };

    render(
      <PromoModal
        open={true}
        onOpenChange={mockOnOpenChange}
        promo={promo}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByRole("dialog", { name: /edit kode promo/i })).toBeInTheDocument();
    expect(screen.getByDisplayValue("DISKON10")).toBeInTheDocument();
    expect(screen.getByDisplayValue("10")).toBeInTheDocument();
    expect(screen.getByDisplayValue("100")).toBeInTheDocument();
  });

  it("submits create input with percent discount", async () => {
    render(
      <PromoModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByPlaceholderText(/contoh: diskon10/i), { target: { value: "DISKON15" } });
    fireEvent.input(screen.getByLabelText(/nilai diskon/i), { target: { value: "15" } });
    fireEvent.input(screen.getByLabelText(/maksimal penggunaan/i), { target: { value: "50" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          code: "DISKON15",
          discount_percent: 15,
          max_uses: 50,
        })
      );
    });
  });

  it("submits create input with fixed discount", async () => {
    render(
      <PromoModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByPlaceholderText(/contoh: diskon10/i), { target: { value: "CASHBACK20K" } });

    const typeSelect = screen.getByLabelText(/jenis diskon/i);
    fireEvent.change(typeSelect, { target: { value: "fixed" } });

    fireEvent.input(screen.getByLabelText(/nilai diskon/i), { target: { value: "20000" } });
    fireEvent.input(screen.getByLabelText(/minimal pembelian/i), { target: { value: "50000" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          code: "CASHBACK20K",
          discount_amount: 20000,
          min_order_amount: 50000,
        })
      );
    });
  });

  it("submits edit input only with allowed fields", async () => {
    const promo: PromoCode = {
      id: "promo-1",
      code: "DISKON10",
      discount_percent: 10,
      max_uses: 100,
      used_count: 3,
    };

    render(
      <PromoModal
        open={true}
        onOpenChange={mockOnOpenChange}
        promo={promo}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/maksimal penggunaan/i), { target: { value: "200" } });
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith({ max_uses: 200, is_public: false, affiliator_only: false });
    });
  });

  // FR-13: is_public round-trips through create and edit. is_public is a
  // plain bool on the wire (not a pointer), so the edit payload must carry
  // it even when turning it off — an omitted key resets it server-side.
  it("round-trips is_public: create with it on, then edit it off", async () => {
    const { rerender } = render(
      <PromoModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByPlaceholderText(/contoh: diskon10/i), { target: { value: "PUBLIKPROMO" } });
    fireEvent.input(screen.getByLabelText(/nilai diskon/i), { target: { value: "10" } });
    fireEvent.click(screen.getByLabelText(/tampilkan di checkout/i));

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ code: "PUBLIKPROMO", is_public: true })
      );
    });

    mockOnSubmit.mockClear();

    const promo: PromoCode = {
      id: "promo-1",
      code: "PUBLIKPROMO",
      discount_percent: 10,
      used_count: 0,
      is_public: true,
    };

    rerender(
      <PromoModal
        open={true}
        onOpenChange={mockOnOpenChange}
        promo={promo}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    const toggle = screen.getByLabelText(/tampilkan di checkout/i);
    expect(toggle).toBeChecked();
    fireEvent.click(toggle);

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith({ is_public: false, affiliator_only: false });
    });
  });

  it("disables save when code or nilai diskon is empty", () => {
    render(
      <PromoModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    const saveButton = screen.getByRole("button", { name: /^simpan$/i });
    expect(saveButton).toBeDisabled();
  });

  describe("Mitra voucher", () => {
    function renderCreate() {
      render(<PromoModal open={true} onOpenChange={mockOnOpenChange} onSubmit={mockOnSubmit} isPending={false} />);
      fireEvent.change(screen.getByPlaceholderText(/contoh: diskon10/i), { target: { value: "MITRA20" } });
      fireEvent.change(screen.getByLabelText(/nilai diskon/i), { target: { value: "20" } });
    }

    it("is hidden from an admin who cannot manage Mitra, and sends nothing about it", () => {
      renderCreate();
      expect(screen.queryByLabelText(/voucher mitra/i)).toBeNull();

      fireEvent.click(screen.getByRole("button", { name: /simpan/i }));
      expect(mockOnSubmit.mock.calls[0][0]).not.toHaveProperty("partner_voucher");
    });

    it("needs a quota unit before it can be saved, with no unit preselected", () => {
      capability.value = true;
      renderCreate();
      fireEvent.click(screen.getByRole("checkbox", { name: /voucher mitra/i }));

      expect(screen.getByLabelText("Kuota dihitung")).toHaveValue("");
      expect(screen.getByRole("button", { name: /simpan/i })).toBeDisabled();

      fireEvent.change(screen.getByLabelText("Kuota dihitung"), { target: { value: "student" } });
      expect(screen.getByRole("button", { name: /simpan/i })).toBeEnabled();
    });

    it("sends the unit and the chosen Mitra, and is never public", () => {
      capability.value = true;
      renderCreate();
      fireEvent.click(screen.getByRole("checkbox", { name: /tampilkan di checkout/i }));
      fireEvent.click(screen.getByRole("checkbox", { name: /voucher mitra/i }));
      fireEvent.change(screen.getByLabelText("Kuota dihitung"), { target: { value: "order" } });
      fireEvent.click(screen.getByRole("button", { name: /pilih mitra/i }));
      fireEvent.click(screen.getByRole("button", { name: /simpan/i }));

      expect(mockOnSubmit.mock.calls[0][0]).toMatchObject({
        is_public: false,
        partner_voucher: { enabled: true, ids: ["mitra-1"], quota_unit: "order" },
      });
    });

    it("cannot be combined with the promo shared by Affiliators", () => {
      capability.value = true;
      renderCreate();
      fireEvent.click(screen.getByRole("checkbox", { name: /khusus affiliator/i }));
      expect(screen.getByRole("checkbox", { name: /voucher mitra/i })).toBeDisabled();
    });

    it("on an edit, switches an existing Mitra voucher off explicitly, and leaves an ordinary promo alone", () => {
      capability.value = true;
      const voucher: PromoCode = { id: "p1", code: "MITRA", discount_percent: 10, used_count: 0, partner: { ids: [], quota_unit: "student" } };
      const { unmount } = render(<PromoModal open={true} onOpenChange={mockOnOpenChange} promo={voucher} onSubmit={mockOnSubmit} isPending={false} />);
      expect(screen.getByRole("checkbox", { name: /voucher mitra/i })).toBeChecked();
      fireEvent.click(screen.getByRole("checkbox", { name: /voucher mitra/i }));
      fireEvent.click(screen.getByRole("button", { name: /simpan/i }));
      expect(mockOnSubmit.mock.calls[0][0]).toMatchObject({ partner_voucher: { enabled: false } });
      unmount();

      mockOnSubmit.mockReset();
      render(<PromoModal open={true} onOpenChange={mockOnOpenChange} promo={{ id: "p2", code: "BIASA", discount_percent: 5, used_count: 0 }} onSubmit={mockOnSubmit} isPending={false} />);
      fireEvent.click(screen.getByRole("button", { name: /simpan/i }));
      expect(mockOnSubmit.mock.calls[0][0]).not.toHaveProperty("partner_voucher");
    });
  });
});
