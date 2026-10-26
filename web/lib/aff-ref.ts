export const AFF_REF_COOKIE = "aff_ref";
export const AFF_REF_MAX_AGE_SECONDS = 30 * 24 * 60 * 60;

// days comes from the admin's cookie_days setting; without it the 30-day default applies.
export function setAffRefCookie(code: string, days?: number): void {
  const maxAge = days && days > 0 ? Math.round(days * 24 * 60 * 60) : AFF_REF_MAX_AGE_SECONDS;
  document.cookie = `${AFF_REF_COOKIE}=${encodeURIComponent(code.trim().toUpperCase())}; path=/; max-age=${maxAge}; SameSite=Lax`;
}

export function readAffRefCookie(): string {
  const prefix = `${AFF_REF_COOKIE}=`;
  const entry = document.cookie.split("; ").find((c) => c.startsWith(prefix));
  if (!entry) return "";
  try {
    return decodeURIComponent(entry.slice(prefix.length));
  } catch {
    return "";
  }
}
