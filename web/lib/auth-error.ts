import { ApiError } from "@/lib/api";
import type { useTranslation } from "@/lib/i18n";

export function loginErrorMessage(err: unknown, t: ReturnType<typeof useTranslation>["t"]): string {
  if (err instanceof ApiError && err.code === "rate_limited") {
    return t("login_rate_limited");
  }
  if (err instanceof ApiError) {
    return err.message;
  }
  return t("login_failed");
}
