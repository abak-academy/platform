import { describe, it, expect } from "vitest";
import { formToInput, settingsToForm } from "./affiliate-settings-form";
import type { AffiliateLevelSettings, AffiliateSettings } from "@/lib/types";

const settings: AffiliateSettings = {
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
  launch_date: "2026-11-01T00:00:00Z",
  eval_day: 8,
  silver_threshold: 5000000,
  gold_threshold: 15000000,
  hold_thresholds: { silver: 1000000 },
  daily_registration_cap: null,
  updated_by: null,
  updated_at: "2026-10-01T00:00:00Z",
};

const levels: AffiliateLevelSettings[] = [
  {
    level: "silver",
    commission_pct: { book: 7.5 },
    digital_wait_days: null,
    physical_received_days: null,
    physical_wait_days: null,
    min_payout: 50000,
    payout_day_of_month: null,
    rise_threshold: 5000000,
    hold_threshold: 1000000,
  },
];

describe("affiliate settings form", () => {
  it("round-trips the server settings unchanged", () => {
    const input = formToInput(settingsToForm(settings, levels));

    expect(input).not.toBeNull();
    expect(input?.cookie_days).toBe(30);
    expect(input?.payout_day_of_month).toBeNull();
    expect(input?.launch_date).toBe("2026-11-01");
    expect(input?.hold_thresholds).toEqual({ silver: 1000000 });
    const silver = input?.levels.find((l) => l.level === "silver");
    expect(silver?.commission_pct).toEqual({ book: 7.5 });
    expect(silver?.min_payout).toBe(50000);
    const bronze = input?.levels.find((l) => l.level === "bronze");
    expect(bronze?.commission_pct).toBeNull();
    expect(bronze?.min_payout).toBeNull();
  });

  it("clears an emptied optional field to null", () => {
    const form = settingsToForm(settings, levels);
    form.levels.silver.fields.min_payout = "";

    expect(formToInput(form)?.levels.find((l) => l.level === "silver")?.min_payout).toBeNull();
  });

  it("refuses a body with a non-numeric field", () => {
    const form = settingsToForm(settings, levels);
    form.numbers.cookie_days = "abc";

    expect(formToInput(form)).toBeNull();
  });

  it("refuses an emptied required field", () => {
    const form = settingsToForm(settings, levels);
    form.numbers.min_payout = "";

    expect(formToInput(form)).toBeNull();
  });

  it("refuses a non-numeric commission percent", () => {
    const form = settingsToForm(settings, levels);
    form.levels.gold.pct.exam = "10%";

    expect(formToInput(form)).toBeNull();
  });
});
