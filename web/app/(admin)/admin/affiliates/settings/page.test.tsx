import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import SettingsPage from "./page";

const h = vi.hoisted(() => ({
  settings: vi.fn(),
  update: vi.fn(),
  program: vi.fn(),
  overrides: vi.fn(),
  upsert: vi.fn(),
  remove: vi.fn(),
  run: vi.fn(),
  products: vi.fn(),
  replace: vi.fn(),
  search: { value: "" },
  capability: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: h.replace }),
  usePathname: () => "/admin/affiliates/settings",
  useSearchParams: () => new URLSearchParams(h.search.value),
}));
vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useAffiliateOverrideProducts: h.products,
  useAdminAffiliateSettings: h.settings,
  useUpdateAffiliateSettings: () => ({ mutate: h.update, isPending: false }),
  useSetAffiliateProgram: () => ({ mutate: h.program, isPending: false }),
  useAdminAffiliateOverrides: h.overrides,
  useUpsertAffiliateOverride: () => ({ mutate: h.upsert, isPending: false }),
  useDeleteAffiliateOverride: () => ({ mutate: h.remove, isPending: false }),
  useRunAffiliateJob: () => ({ mutate: h.run, isPending: false }),
}));
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: h.capability }));
vi.mock("sonner", () => ({ toast: h.toast }));

const view = {
  program_enabled: false,
  settings: {
    id: 1,
    cookie_days: 30,
    binding_months: 3,
    min_extend_amount: 0,
    digital_wait_days: 7,
    physical_wait_days: 14,
    min_payout: 100000,
    payout_day_of_month: null,
    launch_date: null,
    eval_day: 8,
    daily_registration_cap: null,
  },
  levels: [],
};

async function chooseRuleType(trigger: HTMLElement, option: string) {
  fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false, pointerType: "mouse" });
  fireEvent.click(await screen.findByRole("option", { name: option }));
}

describe("AdminAffiliateSettingsPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => "mockReset" in m && m.mockReset());
    h.toast.success.mockReset();
    h.toast.error.mockReset();
    h.capability.mockReturnValue(true);
    h.settings.mockReturnValue({ isLoading: false, isError: false, data: view });
    h.search.value = "";
    h.overrides.mockReturnValue({
      isLoading: false,
      isError: false,
      data: {
        data: [{ product_id: "p-1", product_name: "Kursus Matematika", product_type: "course", percentage: 12.5, updated_at: "2026-10-01T00:00:00Z" }],
        total: 1,
        page: 1,
        per_page: 10,
      },
    });
    h.products.mockReturnValue({
      isLoading: false,
      data: [{ id: "p-2", name: "Buku Fisika", type: "book", percentage: null }],
    });
  });

  it("is blocked without the capability", () => {
    h.capability.mockReturnValue(false);
    render(<SettingsPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });

  it("shows the program as off and turns it on only after confirming", async () => {
    render(<SettingsPage />);

    expect(screen.getByText("Nonaktif")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Aktifkan program" }));
    expect(h.program).not.toHaveBeenCalled();

    fireEvent.click(await screen.findByRole("button", { name: "Konfirmasi" }));
    expect(h.program).toHaveBeenCalledWith(true, expect.anything());
  });

  it("offers to turn an active program off", () => {
    h.settings.mockReturnValue({ isLoading: false, isError: false, data: { ...view, program_enabled: true } });
    render(<SettingsPage />);

    expect(screen.getByText("Aktif")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Nonaktifkan program" })).toBeInTheDocument();
  });

  it("loads the current values into the form", async () => {
    render(<SettingsPage />);

    expect(await screen.findByLabelText("Masa berlaku cookie")).toHaveValue("30");
    expect(screen.getByLabelText("Penarikan minimum", { selector: "#g-min_payout" })).toHaveValue("100000");
  });

  it("saves an edited value with the untouched ones", async () => {
    render(<SettingsPage />);

    fireEvent.change(await screen.findByLabelText("Masa berlaku cookie"), { target: { value: "45" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan pengaturan" }));

    expect(h.update).toHaveBeenCalledTimes(1);
    const body = h.update.mock.calls[0][0];
    expect(body.cookie_days).toBe(45);
    expect(body.binding_months).toBe(3);
    expect(body.levels).toHaveLength(3);
  });

  it("does not send a non-numeric value", async () => {
    render(<SettingsPage />);

    fireEvent.change(await screen.findByLabelText("Masa berlaku cookie"), { target: { value: "abc" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan pengaturan" }));

    expect(h.update).not.toHaveBeenCalled();
    expect(h.toast.error).toHaveBeenCalledWith("Isian angka tidak valid.");
  });

  it("explains every field under its input", async () => {
    render(<SettingsPage />);

    await screen.findByLabelText("Masa berlaku cookie");
    expect(screen.getAllByText(/Berapa lama browser calon siswa mengingat tautan referral/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/Contoh:/).length).toBeGreaterThan(8);
  });

  it("shows the column guide like the student bulk import", async () => {
    render(<SettingsPage />);

    await screen.findByLabelText("Masa berlaku cookie");
    expect(screen.getByText("Lihat aturan kolom")).toBeInTheDocument();
  });

  it("opens the level tab from the URL and falls back to the global value as placeholder", async () => {
    h.search.value = "tab=levels";
    render(<SettingsPage />);

    const field = await screen.findByLabelText("Penarikan minimum", { selector: "#l-silver-min_payout" });
    expect(field).toHaveAttribute("placeholder", "100000");
    expect(screen.getByText("Gold")).toBeInTheDocument();
  });

  it("lets the admin pick percent or fixed per category on a level and saves the typed rule", async () => {
    h.search.value = "tab=levels";
    render(<SettingsPage />);

    const selects = await screen.findAllByLabelText("Buku Jenis");
    expect(selects).toHaveLength(3);
    await chooseRuleType(selects[0], "Nominal");
    fireEvent.change(document.getElementById("l-bronze-rule-book") as HTMLInputElement, { target: { value: "4500" } });
    fireEvent.click(screen.getAllByRole("button", { name: "Simpan pengaturan" })[0]);

    const body = h.update.mock.calls[0][0];
    const bronze = body.levels.find((l: { level: string }) => l.level === "bronze");
    expect(bronze.commission_rule).toEqual({ book: { type: "fixed", value: 4500 } });
  });

  it("shows no received-days field and a level binding period", async () => {
    h.search.value = "tab=levels";
    render(<SettingsPage />);

    expect(await screen.findByLabelText("Masa ikatan siswa", { selector: "#l-silver-binding_months" })).toHaveAttribute("placeholder", "3");
    expect(screen.queryByLabelText(/Batas konfirmasi barang diterima/)).not.toBeInTheDocument();
  });

  it("switches tabs through the URL", async () => {
    render(<SettingsPage />);
    await screen.findByLabelText("Masa berlaku cookie");

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Override produk" }));

    expect(h.replace).toHaveBeenCalledWith("/admin/affiliates/settings?tab=overrides", { scroll: false });
  });

  it("picks an override product by name instead of typing an id, and validates the percent", async () => {
    h.search.value = "tab=overrides";
    render(<SettingsPage />);

    fireEvent.focus(await screen.findByLabelText("Produk"));
    fireEvent.click(await screen.findByRole("button", { name: /Buku Fisika/ }));
    fireEvent.change(screen.getByLabelText("Nilai komisi"), { target: { value: "150" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan override" }));
    expect(h.upsert).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Nilai komisi"), { target: { value: "7.125" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan override" }));
    expect(h.upsert).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Nilai komisi"), { target: { value: "15" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan override" }));
    expect(h.upsert).toHaveBeenCalledWith({ productId: "p-2", type: "percent", value: 15 }, expect.anything());
  });

  it("saves a fixed rupiah override per unit and refuses cents or an out-of-range percent", async () => {
    h.search.value = "tab=overrides";
    render(<SettingsPage />);

    fireEvent.focus(await screen.findByLabelText("Produk"));
    fireEvent.click(await screen.findByRole("button", { name: /Buku Fisika/ }));
    await chooseRuleType(screen.getByLabelText("Jenis"), "Nominal");

    fireEvent.change(screen.getByLabelText("Nilai komisi"), { target: { value: "4500.5" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan override" }));
    expect(h.upsert).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Nilai komisi"), { target: { value: "25000" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan override" }));
    expect(h.upsert).toHaveBeenCalledWith({ productId: "p-2", type: "fixed", value: 25000 }, expect.anything());
  });

  it("refuses an override without a product", async () => {
    h.search.value = "tab=overrides";
    render(<SettingsPage />);

    fireEvent.change(await screen.findByLabelText("Nilai komisi"), { target: { value: "15" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan override" }));

    expect(h.upsert).not.toHaveBeenCalled();
    expect(h.toast.error).toHaveBeenCalledWith("Pilih produk dan isi nilai komisi yang valid: persen 0–100 (maks 2 desimal) atau nominal rupiah bulat.");
  });

  it("lists overrides with the product name and removes one", async () => {
    h.search.value = "tab=overrides";
    render(<SettingsPage />);

    expect(await screen.findByText("Kursus Matematika")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Hapus" }));

    expect(h.remove).toHaveBeenCalledWith("p-1", expect.anything());
  });

  it("sends a quarter only for the levels job", async () => {
    h.search.value = "tab=jobs";
    render(<SettingsPage />);
    await screen.findByTestId("job-scope");

    fireEvent.click(screen.getByRole("button", { name: "Jalankan" }));
    expect(h.run).toHaveBeenLastCalledWith({ scope: "bindings", quarter: undefined }, expect.anything());

    fireEvent.pointerDown(screen.getByTestId("job-scope"), { button: 0, ctrlKey: false, pointerType: "mouse" });
    fireEvent.click(await screen.findByRole("option", { name: "Level" }));
    expect(screen.getByText(/Menilai ulang level affiliator/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/Kuartal/), { target: { value: "2026-Q3" } });
    fireEvent.click(screen.getByRole("button", { name: "Jalankan" }));
    expect(h.run).toHaveBeenLastCalledWith({ scope: "levels", quarter: "2026-Q3" }, expect.anything());
  });

  it("explains the level rules with a column guide too", async () => {
    h.search.value = "tab=levels";
    render(<SettingsPage />);

    await screen.findByText("Lihat aturan kolom");
    expect(screen.getAllByText(/Penjualan kuartal minimum agar level ini tidak turun/).length).toBeGreaterThan(0);
  });
});
