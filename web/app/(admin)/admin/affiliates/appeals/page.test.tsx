import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AppealsPage from "./page";

const h = vi.hoisted(() => ({ list: vi.fn(), decide: vi.fn(), capability: vi.fn() }));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useAdminAffiliateAppeals: h.list,
  useDecideAffiliateAppeal: () => ({ mutate: h.decide, isPending: false }),
}));
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: h.capability }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const appeal = (id: string, appeal_status: string) => ({
  id,
  sanction: "suspend",
  description: "Pembelian atas akun sendiri",
  appeal_text: "Itu pembelian keluarga",
  appeal_status,
  affiliator_name: "Budi Santoso",
  affiliator_code: "BUDI1234",
});

describe("AdminAffiliateAppealsPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => m.mockReset());
    h.capability.mockReturnValue(true);
    h.list.mockReturnValue({ isLoading: false, isError: false, data: { data: [appeal("v1", "pending"), appeal("v2", "upheld")] } });
  });

  it("shows the appeal text and only decides pending appeals", () => {
    render(<AppealsPage />);

    expect(h.list).toHaveBeenCalledWith("pending");
    expect(screen.getAllByText("Itu pembelian keluarga")).toHaveLength(2);
    expect(screen.getAllByRole("button", { name: "Pertahankan sanksi" })).toHaveLength(1);
  });

  it("maps the two decisions to upheld and rejected", () => {
    render(<AppealsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Pertahankan sanksi" }));
    fireEvent.click(screen.getByRole("button", { name: "Batalkan sanksi" }));

    expect(h.decide).toHaveBeenNthCalledWith(1, { id: "v1", outcome: "upheld" }, expect.anything());
    expect(h.decide).toHaveBeenNthCalledWith(2, { id: "v1", outcome: "rejected" }, expect.anything());
  });
});
