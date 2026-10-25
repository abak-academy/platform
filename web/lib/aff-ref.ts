export const AFF_REF_COOKIE = "aff_ref";
export const AFF_REF_MAX_AGE_SECONDS = 30 * 24 * 60 * 60;

export function setAffRefCookie(code: string): void {
  document.cookie = `${AFF_REF_COOKIE}=${encodeURIComponent(code)}; path=/; max-age=${AFF_REF_MAX_AGE_SECONDS}; SameSite=Lax`;
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
