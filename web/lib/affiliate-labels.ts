import { DICT, type I18nKey } from "@/lib/i18n";

// Server enum codes map to aff_v_<code>; an unknown code shows as-is rather
// than an empty cell when the server adds a value before the web does.
export function affLabel(t: (key: I18nKey) => string, code: string): string {
  const key = `aff_v_${code}`;
  return key in DICT.id ? t(key as I18nKey) : code;
}

export function formatAffDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("id-ID", { day: "numeric", month: "short", year: "numeric" });
}
