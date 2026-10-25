import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import SettingsPage from "./page";

const h = vi.hoisted(() => ({
  settings: vi.fn(),
  update: vi.fn(),
  overrides: vi.fn(),
  upsert: vi.fn(),
  remove: vi.fn(),
  run: vi.fn(),
  capability: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useAdminAffiliateSettings: h.settings,
  useUpdateAffiliateSettings: () => ({ mutate: h.update, isPending: false }),
  useAdminAffiliateOverrides: h.overrides,
  useUpsertAffiliateOverride: () => ({ mutate: h.upsert, isPending: false }),
  useDeleteAffiliateOverride: () => ({ mutate: h.remove, isPending: false }),
  useRunAffiliateJob: () => ({ mutate: h.run, isPending: false }),
}));
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: h.capability }));
vi.mock("sonner", () => ({ toast: h.toast }));

const view = {
  settings: {
    id: 1,
    cookie_days: 30,
    binding_months: 3,
    min_extend_amount: 0,
    digital_wait_days: 7,
    physical_received_days: 7,
    physical_wait_days: 7,
    min_payout: 100000,
    payout_day_of_month: null,
    adjustment_threshold: 1000000,
    adjustment_threshold_enabled: false,
    freeze_max_days: 14,
    soon_end_days: 30,
    launch_date: null,
    eval_day: 8,
    silver_threshold: 5000000,
    gold_threshold: 15000000,
    hold_thresholds: null,
    daily_registration_cap: null,
  },
  levels: [],
};

describe("AdminAffiliateSettingsPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => "mockReset" in m && m.mockReset());
    h.toast.success.mockReset();
    h.toast.error.mockReset();
    h.capability.mockReturnValue(true);
    h.settings.mockReturnValue({ isLoading: false, isError: false, data: view });
    h.overrides.mockReturnValue({ data: [{ product_id: "p-1", percentage: 12.5 }] });
  });

  it("is blocked without the capability", () => {
    h.capability.mockReturnValue(false);
    render(<SettingsPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });

  it("loads the current values into the form", async () => {
    render(<SettingsPage />);

    expect(await screen.findByLabelText("Masa berlaku cookie (hari)")).toHaveValue("30");
    expect(screen.getByLabelText("Penarikan minimum", { selector: "#g-min_payout" })).toHaveValue("100000");
  });

  it("saves an edited value with the untouched ones", async () => {
    render(<SettingsPage />);

    fireEvent.change(await screen.findByLabelText("Masa berlaku cookie (hari)"), { target: { value: "45" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan pengaturan" }));

    expect(h.update).toHaveBeenCalledTimes(1);
    const body = h.update.mock.calls[0][0];
    expect(body.cookie_days).toBe(45);
    expect(body.binding_months).toBe(3);
    expect(body.levels).toHaveLength(3);
  });

  it("does not send a non-numeric value", async () => {
    render(<SettingsPage />);

    fireEvent.change(await screen.findByLabelText("Masa berlaku cookie (hari)"), { target: { value: "abc" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan pengaturan" }));

    expect(h.update).not.toHaveBeenCalled();
    expect(h.toast.error).toHaveBeenCalledWith("Isian angka tidak valid.");
  });

  it("validates an override percent before sending", async () => {
    render(<SettingsPage />);

    fireEvent.change(await screen.findByLabelText("ID produk"), { target: { value: "p-2" } });
    fireEvent.change(screen.getByLabelText("Persen"), { target: { value: "150" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan override" }));
    expect(h.upsert).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Persen"), { target: { value: "15" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan override" }));
    expect(h.upsert).toHaveBeenCalledWith({ productId: "p-2", percentage: 15 }, expect.anything());
  });

  it("removes an override", async () => {
    render(<SettingsPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Hapus" }));

    expect(h.remove).toHaveBeenCalledWith("p-1", expect.anything());
  });

  it("sends a quarter only for the levels job", async () => {
    render(<SettingsPage />);
    await screen.findByLabelText("Cakupan");

    fireEvent.click(screen.getByRole("button", { name: "Jalankan" }));
    expect(h.run).toHaveBeenLastCalledWith({ scope: "bindings", quarter: undefined }, expect.anything());

    fireEvent.change(screen.getByLabelText("Cakupan"), { target: { value: "levels" } });
    fireEvent.change(screen.getByLabelText(/Kuartal/), { target: { value: "2026-Q3" } });
    fireEvent.click(screen.getByRole("button", { name: "Jalankan" }));
    expect(h.run).toHaveBeenLastCalledWith({ scope: "levels", quarter: "2026-Q3" }, expect.anything());
  });
});
