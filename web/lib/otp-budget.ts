// Shared sessionStorage state for the OTP challenge flow (login → /otp).
// The resend budget is anchored to the wall clock (abak-resend-at) so any
// page that adopts it — login, register, the OTP page itself — resumes the
// countdown from the remaining time instead of restarting from the raw
// stored value.
const RESPEND_AFTER_KEY = "abak-resend-after";
const RESPEND_AT_KEY = "abak-resend-at";

export function adoptResendBudget(budgetSec: number): void {
  if (typeof window === "undefined") return;
  const budget = Math.max(0, Math.ceil(budgetSec) || 0);
  sessionStorage.setItem(RESPEND_AFTER_KEY, String(budget));
  sessionStorage.setItem(RESPEND_AT_KEY, String(Date.now()));
}

export function clearResendBudget(): void {
  if (typeof window === "undefined") return;
  sessionStorage.removeItem(RESPEND_AFTER_KEY);
  sessionStorage.removeItem(RESPEND_AT_KEY);
}

// Remaining seconds until the next send is allowed. Absent budget → 60 (the
// default floor the pages have always shown); an expired anchor → 0.
export function readResendBudget(): number {
  if (typeof window === "undefined") return 60;
  const raw = sessionStorage.getItem(RESPEND_AFTER_KEY);
  const stored = raw === null ? 60 : Math.max(0, Math.ceil(Number(raw) || 0));
  const at = Number(sessionStorage.getItem(RESPEND_AT_KEY) || 0);
  if (at <= 0) return stored;
  return Math.max(0, stored - Math.floor((Date.now() - at) / 1000));
}
