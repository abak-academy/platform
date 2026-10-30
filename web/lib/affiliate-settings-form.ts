import type {
  AffiliateCommissionRule,
  AffiliateCommissionRuleType,
  AffiliateLevelSettings,
  AffiliateSettings,
  AffiliateSettingsInput,
} from "@/lib/types";

export const COMMISSION_CATEGORIES = ["exam", "course", "book", "merchandise", "medal"] as const;
export const AFFILIATE_LEVELS = ["bronze", "silver", "gold"] as const;

export const GLOBAL_NUMBER_FIELDS = [
  "cookie_days",
  "binding_months",
  "min_extend_amount",
  "digital_wait_days",
  "physical_wait_days",
  "min_payout",
  "eval_day",
] as const;

export const GLOBAL_OPTIONAL_FIELDS = ["payout_day_of_month", "daily_registration_cap"] as const;

export const LEVEL_FIELDS = [
  "binding_months",
  "digital_wait_days",
  "physical_wait_days",
  "min_payout",
  "payout_day_of_month",
  "rise_threshold",
  "hold_threshold",
] as const;

export type GlobalNumberField = (typeof GLOBAL_NUMBER_FIELDS)[number];
export type GlobalOptionalField = (typeof GLOBAL_OPTIONAL_FIELDS)[number];
export type LevelField = (typeof LEVEL_FIELDS)[number];

export interface RuleForm {
  type: AffiliateCommissionRuleType;
  value: string;
}

export interface LevelForm {
  fields: Record<LevelField, string>;
  rules: Record<string, RuleForm>;
}

export interface SettingsForm {
  numbers: Record<GlobalNumberField, string>;
  optional: Record<GlobalOptionalField, string>;
  launchDate: string;
  levels: Record<string, LevelForm>;
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
    const rules: Record<string, RuleForm> = {};
    for (const category of COMMISSION_CATEGORIES) {
      const rule = row?.commission_rule?.[category];
      rules[category] = { type: rule?.type ?? "percent", value: str(rule?.value) };
    }
    levelForms[level] = { fields, rules };
  }

  return {
    numbers,
    optional,
    launchDate: settings.launch_date ? settings.launch_date.slice(0, 10) : "",
    levels: levelForms,
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
    const rules: Record<string, AffiliateCommissionRule> = {};
    for (const category of COMMISSION_CATEGORIES) {
      const n = check(parseOptional(lf.rules[category].value));
      if (n !== null) rules[category] = { type: lf.rules[category].type, value: n };
    }
    return {
      level,
      commission_rule: Object.keys(rules).length > 0 ? rules : null,
      binding_months: check(parseOptional(lf.fields.binding_months)),
      digital_wait_days: check(parseOptional(lf.fields.digital_wait_days)),
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
    launch_date: form.launchDate || null,
    levels,
  };
}
