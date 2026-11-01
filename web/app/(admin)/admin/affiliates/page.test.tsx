import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AdminAffiliatesPage from "./page";

const h = vi.hoisted(() => ({
  list: vi.fn(),
  decide: vi.fn(),
  download: vi.fn(),
  capability: vi.fn(),
  replace: vi.fn(),
  search: { value: "" },
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: h.replace }),
  usePathname: () => "/admin/affiliates",
  useSearchParams: () => new URLSearchParams(h.search.value),
}));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useAdminAffiliates: h.list,
  useDecideAffiliateApproval: () => ({ mutate: h.decide, isPending: false }),
  downloadAffiliateCsv: h.download,
  useCreateAffiliator: () => ({ mutate: vi.fn(), isPending: false }),
  useBulkCreateAffiliators: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/hooks/use-capability", () => ({
  useHasCapability: h.capability,
}));

vi.mock("sonner", () => ({ toast: h.toast }));

const pending = {
  id: "a1",
  name: "Budi Santoso",
  email: "budi@example.com",
  code: "",
  status: "pending_approval",
  level: "bronze",
  created_at: "2026-10-01T00:00:00Z",
};
const active = { ...pending, id: "a2", name: "Sari", code: "SARI2222", status: "active" };

describe("AdminAffiliatesPage", () => {
  beforeEach(() => {
    h.decide.mockReset();
    h.search.value = "";
    h.toast.success.mockReset();
    h.toast.error.mockReset();
    h.capability.mockReturnValue(true);
    h.list.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { data: [pending, active], page: 1, per_page: 20, total: 2 },
    });
  });

  it("is blocked without the affiliates capability", () => {
    h.capability.mockReturnValue(false);
    render(<AdminAffiliatesPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });

  it("offers approve and reject only on pending applications", () => {
    render(<AdminAffiliatesPage />);

    expect(screen.getAllByRole("button", { name: "Setujui" })).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: "Tolak" })).toHaveLength(1);
    expect(screen.getAllByRole("link", { name: "Detail" })).toHaveLength(2);
  });

  it("downloads the affiliator list with the current filters", () => {
    h.search.value = "q=budi&status=active&type=gold";
    h.download.mockResolvedValue(undefined);
    render(<AdminAffiliatesPage />);

    fireEvent.click(screen.getByRole("button", { name: "Unduh CSV" }));

    expect(h.download).toHaveBeenCalledWith("affiliators", { q: "budi", status: "active", type: "gold" });
  });

  it("lets an admin reopen a rejected application", () => {
    h.list.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { data: [{ ...pending, id: "a3", status: "rejected" }], page: 1, per_page: 20, total: 1 },
    });
    render(<AdminAffiliatesPage />);

    fireEvent.click(screen.getByRole("button", { name: "Buka kembali" }));

    expect(h.decide).toHaveBeenCalledWith({ id: "a3", decision: "reopen" }, expect.anything());
  });

  it("opens the add-affiliator dialog with single and CSV modes", async () => {
    render(<AdminAffiliatesPage />);

    fireEvent.click(screen.getByRole("button", { name: "Tambah affiliator" }));

    expect(await screen.findByLabelText("Nomor telepon")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Unggah CSV" })).toBeInTheDocument();
  });

  it("approves without a reason", () => {
    render(<AdminAffiliatesPage />);

    fireEvent.click(screen.getByRole("button", { name: "Setujui" }));

    expect(h.decide).toHaveBeenCalledWith({ id: "a1", decision: "approve" }, expect.anything());
  });

  it("will not reject without a reason, then rejects with it", async () => {
    render(<AdminAffiliatesPage />);

    fireEvent.click(screen.getByRole("button", { name: "Tolak" }));
    fireEvent.click(await screen.findByRole("button", { name: "Konfirmasi" }));
    expect(h.decide).not.toHaveBeenCalled();
    expect(screen.getByText("Alasan wajib diisi")).toBeInTheDocument();

    fireEvent.change(screen.getByRole("textbox"), { target: { value: "data tidak lengkap" } });
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));

    await waitFor(() =>
      expect(h.decide).toHaveBeenCalledWith(
        { id: "a1", decision: "reject", reason: "data tidak lengkap" },
        expect.anything(),
      ),
    );
  });

  it("sends the URL search, filters and page size to the query", () => {
    h.search.value = "q=budi&status=active&type=gold&page=2&per_page=50";
    render(<AdminAffiliatesPage />);

    expect(h.list).toHaveBeenLastCalledWith({ q: "budi", status: "active", level: "gold", page: 2, per_page: 50 });
  });
});
