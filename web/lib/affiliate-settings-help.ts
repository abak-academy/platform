import type { I18nKey } from "@/lib/i18n";
import type { GlobalNumberField, GlobalOptionalField, LevelField } from "@/lib/affiliate-settings-form";

export interface FieldHelp {
  unit?: "days" | "months" | "rupiah" | "percent";
  example: string;
  required: boolean;
}

export type GlobalField = GlobalNumberField | GlobalOptionalField | "launch_date";

export const GLOBAL_HELP: Record<GlobalField, FieldHelp> = {
  cookie_days: { unit: "days", example: "30", required: true },
  binding_months: { unit: "months", example: "12", required: true },
  min_extend_amount: { unit: "rupiah", example: "100000", required: true },
  launch_date: { example: "2026-11-01", required: false },
  digital_wait_days: { unit: "days", example: "7", required: true },
  physical_received_days: { unit: "days", example: "14", required: true },
  physical_wait_days: { unit: "days", example: "7", required: true },
  min_payout: { unit: "rupiah", example: "100000", required: true },
  payout_day_of_month: { example: "10", required: false },
  freeze_max_days: { unit: "days", example: "30", required: true },
  adjustment_threshold: { unit: "rupiah", example: "1000000", required: true },
  soon_end_days: { unit: "days", example: "30", required: true },
  eval_day: { example: "5", required: true },
  silver_threshold: { unit: "rupiah", example: "5000000", required: true },
  gold_threshold: { unit: "rupiah", example: "15000000", required: true },
  daily_registration_cap: { example: "50", required: false },
};

export const LEVEL_HELP: Record<LevelField, FieldHelp> = {
  digital_wait_days: { unit: "days", example: "5", required: false },
  physical_received_days: { unit: "days", example: "10", required: false },
  physical_wait_days: { unit: "days", example: "5", required: false },
  min_payout: { unit: "rupiah", example: "50000", required: false },
  payout_day_of_month: { example: "10", required: false },
  rise_threshold: { unit: "rupiah", example: "5000000", required: false },
  hold_threshold: { unit: "rupiah", example: "2000000", required: false },
};

export const hintKey = (field: string): I18nKey => `admin_aff_hint_${field}` as I18nKey;
export const levelHintKey = (field: string): I18nKey => `admin_aff_lhint_${field}` as I18nKey;
