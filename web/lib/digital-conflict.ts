import { ApiError } from "@/lib/api";
import type { I18nKey } from "@/lib/i18n";

type Translate = (key: I18nKey) => string;

const CONFLICT_KEYS: Record<string, Record<string, I18nKey>> = {
  owned: { course: "digital_conflict_owned_course", exam: "digital_conflict_owned_exam" },
  claimed: { course: "digital_conflict_claimed_course", exam: "digital_conflict_claimed_exam" },
  duplicate_in_cart: {
    course: "digital_conflict_duplicate_course",
    exam: "digital_conflict_duplicate_exam",
  },
};

function conflictsOf(err: ApiError): { kind?: unknown; reason?: unknown }[] {
  const details = err.body?.details as { conflicts?: unknown } | undefined;
  return Array.isArray(details?.conflicts) ? details.conflicts : [];
}

export function digitalConflictMessage(err: unknown, t: Translate): string | null {
  if (!(err instanceof ApiError)) return null;
  if (err.code === "digital_product_unavailable") return t("digital_product_unavailable");
  if (err.code !== "digital_target_conflict") return null;

  const keys = new Set<I18nKey>();
  for (const c of conflictsOf(err)) {
    const key = CONFLICT_KEYS[String(c?.reason)]?.[String(c?.kind)];
    if (key) keys.add(key);
  }
  if (keys.size === 0) return t("digital_conflict_generic");
  return [...keys].map(t).join(" ");
}
