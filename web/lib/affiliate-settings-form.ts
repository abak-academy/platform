import type { AffiliateLevelSettings, AffiliateSettings, AffiliateSettingsInput } from "@/lib/types";

export const COMMISSION_CATEGORIES = ["exam", "course", "book", "merchandise", "medal"] as const;
export const AFFILIATE_LEVELS = ["bronze", "silver", "gold"] as const;

export const GLOBAL_NUMBER_FIELDS = [
  "cookie_days",
  "binding_months",
  "min_extend_amount",
  "digital_wait_days",
  "physical_received_days",
  "physical_wait_days",
  "min_payout",
  "adjustment_threshold",
  "freeze_max_days",
  "soon_end_days",
  "eval_day",
  "silver_threshold",
  "gold_threshold",
] as const;

// Stored and round-tripped, but not editable until a feature reads it
// (soon_end_days would drive an "ending soon" notice; notifications are out of phase 1).
export const HIDDEN_NUMBER_FIELDS: readonly GlobalNumberField[] = ["soon_end_days"];

export const GLOBAL_OPTIONAL_FIELDS = ["payout_day_of_month", "daily_registration_cap"] as const;

export const LEVEL_FIELDS = [
  "digital_wait_days",
  "physical_received_days",
  "physical_wait_days",
  "min_payout",
  "payout_day_of_month",
  "rise_threshold",
  "hold_threshold",
] as const;

export type GlobalNumberField = (typeof GLOBAL_NUMBER_FIELDS)[number];
export type GlobalOptionalField = (typeof GLOBAL_OPTIONAL_FIELDS)[number];
export type LevelField = (typeof LEVEL_FIELDS)[number];

export interface LevelForm {
  fields: Record<LevelField, string>;
  pct: Record<string, string>;
}

export interface SettingsForm {
  numbers: Record<GlobalNumberField, string>;
  optional: Record<GlobalOptionalField, string>;
  adjustmentThresholdEnabled: boolean;
  launchDate: string;
  levels: Record<string, LevelForm>;
  holdThresholds: Record<string, number> | null;
}

const str = (n: number | null | undefined) => (n === null || n === undefined ? "" : String(n));

export function settingsToForm(settings: AffiliateSettings, levels: AffiliateLevelSettings[]): SettingsForm {
  const numbers = {} as Record<GlobalNumberField, string>;
  for (const key of GLOBAL_NUMBER_FIELDS) numbers[key] = str(settings[key]);
  const optional = {} as Record<GlobalOptionalField, string>;
  for (const key of GLOBAL_OPTIONAL_FIELDS) optional[key] = str(settings[key]);

  const levelForms: Record<string, LevelForm> = {};
  for (const level of AFFILIATE_LEVELS) {
    const row = levels.find((l) => l.level === level);
    const fields = {} as Record<LevelField, string>;
    for (const key of LEVEL_FIELDS) fields[key] = str(row?.[key]);
    const pct: Record<string, string> = {};
    for (const category of COMMISSION_CATEGORIES) pct[category] = str(row?.commission_pct?.[category]);
    levelForms[level] = { fields, pct };
  }

  return {
    numbers,
    optional,
    adjustmentThresholdEnabled: settings.adjustment_threshold_enabled,
    launchDate: settings.launch_date ? settings.launch_date.slice(0, 10) : "",
    levels: levelForms,
    holdThresholds: settings.hold_thresholds,
  };
}

function parse(value: string): number {
  return value.trim() === "" ? NaN : Number(value);
}

function parseOptional(value: string): number | null {
  return value.trim() === "" ? null : Number(value);
}

// Returns null when any numeric field does not parse, so the page can refuse to
// send a body the server would reject as a type error rather than a range error.
export function formToInput(form: SettingsForm): AffiliateSettingsInput | null {
  let valid = true;
  const check = (n: number | null) => {
    if (n !== null && !Number.isFinite(n)) valid = false;
    return n;
  };

  const numbers = {} as Record<GlobalNumberField, number>;
  for (const key of GLOBAL_NUMBER_FIELDS) numbers[key] = check(parse(form.numbers[key])) as number;

  const levels: AffiliateLevelSettings[] = AFFILIATE_LEVELS.map((level) => {
    const lf = form.levels[level];
    const pct: Record<string, number> = {};
    for (const category of COMMISSION_CATEGORIES) {
      const n = check(parseOptional(lf.pct[category]));
      if (n !== null) pct[category] = n;
    }
    return {
      level,
      commission_pct: Object.keys(pct).length > 0 ? pct : null,
      digital_wait_days: check(parseOptional(lf.fields.digital_wait_days)),
      physical_received_days: check(parseOptional(lf.fields.physical_received_days)),
      physical_wait_days: check(parseOptional(lf.fields.physical_wait_days)),
      min_payout: check(parseOptional(lf.fields.min_payout)),
      payout_day_of_month: check(parseOptional(lf.fields.payout_day_of_month)),
      rise_threshold: check(parseOptional(lf.fields.rise_threshold)),
      hold_threshold: check(parseOptional(lf.fields.hold_threshold)),
    };
  });

  const payoutDay = check(parseOptional(form.optional.payout_day_of_month));
  const cap = check(parseOptional(form.optional.daily_registration_cap));
  if (!valid) return null;

  return {
    ...numbers,
    payout_day_of_month: payoutDay,
    daily_registration_cap: cap,
    adjustment_threshold_enabled: form.adjustmentThresholdEnabled,
    launch_date: form.launchDate || null,
    hold_thresholds: form.holdThresholds,
    levels,
  };
}
