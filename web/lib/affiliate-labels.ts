import { DICT, type I18nKey } from "@/lib/i18n";

// Server enum codes map to aff_v_<code>; an unknown code shows as-is rather
// than an empty cell when the server adds a value before the web does.
export function affLabel(t: (key: I18nKey) => string, code: string): string {
  const key = `aff_v_${code}`;
  return key in DICT.id ? t(key as I18nKey) : code;
}

export function formatAffDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  // A bare YYYY-MM-DD is a calendar day, not an instant: read it as that day
  // locally so it never slips to the day before.
  const day = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  const date = day ? new Date(Number(day[1]), Number(day[2]) - 1, Number(day[3])) : new Date(iso);
  return date.toLocaleDateString("id-ID", { day: "numeric", month: "short", year: "numeric" });
}
