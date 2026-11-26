export function safeNext(next: string | null | undefined, role: string | undefined): string | null {
  if (role !== "student" || !next || !next.startsWith("/") || next.startsWith("//")) return null;
  if (next.startsWith("/\\") || /[\u0000-\u001f\u007f]/.test(next)) return null;
  try {
    const u = new URL(next, "http://x");
    return u.origin === "http://x" ? next : null;
  } catch {
    return null;
  }
}
