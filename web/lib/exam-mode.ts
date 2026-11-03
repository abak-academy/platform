import type { I18nKey } from "@/lib/i18n";

const MODE_KEYS: Record<string, I18nKey> = {
  standard: "exam_packages_modal_mode_standard",
  utbk: "exam_packages_modal_mode_utbk",
  ielts: "exam_packages_modal_mode_ielts",
};

export function examModeLabel(mode: string | null | undefined, t: (key: I18nKey) => string): string {
  const key = MODE_KEYS[mode || "standard"];
  return key ? t(key) : (mode ?? "").toUpperCase();
}
