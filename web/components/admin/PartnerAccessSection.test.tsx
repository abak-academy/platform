import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { PartnerAccessSection } from "./PartnerAccessSection";
import { ApiError } from "@/lib/api";
import type { PartnerAccess } from "@/lib/types";

const hook = vi.hoisted(() => ({
  access: { isLoading: false, isError: false, data: undefined } as { isLoading: boolean; isError: boolean; data?: PartnerAccess },
  save: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/lib/hooks/partner", () => ({
  usePartnerProductAccess: () => hook.access,
  useSetPartnerProductAccess: () => ({ mutate: hook.save, isPending: false }),
}));
vi.mock("@/components/admin/PartnerPicker", () => ({
  PartnerPicker: ({ value, onChange }: { value: string[]; onChange: (ids: string[]) => void }) => (
    <button type="button" onClick={() => onChange([...value, "mitra-1"])}>
      pilih ({value.length})
    </button>
  ),
}));
vi.mock("sonner", () => ({ toast: hook.toast }));

describe("PartnerAccessSection", () => {
  beforeEach(() => {
    hook.save.mockReset();
    hook.toast.success.mockReset();
    hook.toast.error.mockReset();
    hook.access = { isLoading: false, isError: false, data: { scope: "none", ids: [] } };
  });

  it("starts at what the server says, and a new product is closed", () => {
    render(<PartnerAccessSection productId="p1" />);
    expect(screen.getByRole("radio", { name: /tidak tampil/i })).toBeChecked();
    expect(screen.queryByRole("button", { name: /pilih/i })).toBeNull();
  });

  it("opens a product to every Mitra", () => {
    render(<PartnerAccessSection productId="p1" />);
    fireEvent.click(screen.getByRole("radio", { name: /semua mitra/i }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan akses Mitra" }));
    expect(hook.save).toHaveBeenCalledWith({ productId: "p1", access: { scope: "all", ids: [] } }, expect.anything());
  });

  it("will not save 'selected' without a Mitra, then sends the chosen ones", () => {
    render(<PartnerAccessSection productId="p1" />);
    fireEvent.click(screen.getByRole("radio", { name: /mitra terpilih/i }));
    expect(screen.getByRole("button", { name: "Simpan akses Mitra" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: /pilih/i }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan akses Mitra" }));
    expect(hook.save).toHaveBeenCalledWith({ productId: "p1", access: { scope: "selected", ids: ["mitra-1"] } }, expect.anything());
  });

  it("drops the chosen Mitra when the scope is not 'selected'", async () => {
    hook.access = { isLoading: false, isError: false, data: { scope: "selected", ids: ["mitra-9"] } };
    render(<PartnerAccessSection productId="p1" />);
    await waitFor(() => expect(screen.getByRole("radio", { name: /mitra terpilih/i })).toBeChecked());

    fireEvent.click(screen.getByRole("radio", { name: /tidak tampil/i }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan akses Mitra" }));
    expect(hook.save).toHaveBeenCalledWith({ productId: "p1", access: { scope: "none", ids: [] } }, expect.anything());
  });

  it("reports a save that failed", () => {
    hook.save.mockImplementation((_input, opts) => opts.onError(new ApiError("validation_error", "pilih minimal satu Mitra", 400)));
    render(<PartnerAccessSection productId="p1" />);
    fireEvent.click(screen.getByRole("radio", { name: /semua mitra/i }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan akses Mitra" }));
    expect(hook.toast.error).toHaveBeenCalledWith("pilih minimal satu Mitra");
  });

  it("shows loading and error states", () => {
    hook.access = { isLoading: true, isError: false };
    const loading = render(<PartnerAccessSection productId="p1" />);
    expect(screen.getByText("Memuat...")).toBeInTheDocument();
    loading.unmount();

    hook.access = { isLoading: false, isError: true };
    render(<PartnerAccessSection productId="p1" />);
    expect(screen.getByRole("alert")).toHaveTextContent("Gagal memuat akses Mitra.");
  });
});
