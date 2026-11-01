import { readAffRefCookie } from "./aff-ref";

const MANUAL_REFERRAL_KEY = "abak-referral-code";

// A code the visitor typed counts as manual only when it differs from the
// cookie prefill — otherwise the cookie alone carries the attribution.
export function storeManualReferral(typed: string): void {
  const code = typed.trim().toUpperCase();
  try {
    if (code && code !== readAffRefCookie().toUpperCase()) {
      sessionStorage.setItem(MANUAL_REFERRAL_KEY, code);
    } else {
      sessionStorage.removeItem(MANUAL_REFERRAL_KEY);
    }
  } catch {
    // storage unavailable: attribution falls back to the cookie
  }
}

export interface ReferralFields {
  referral_code?: string;
  referral_code_cookie?: string;
}

export function readReferralFields(): ReferralFields {
  const fields: ReferralFields = {};
  try {
    const manual = sessionStorage.getItem(MANUAL_REFERRAL_KEY);
    if (manual) fields.referral_code = manual;
  } catch {
    // ignore
  }
  const cookie = readAffRefCookie();
  if (cookie) fields.referral_code_cookie = cookie;
  return fields;
}

export function clearManualReferral(): void {
  try {
    sessionStorage.removeItem(MANUAL_REFERRAL_KEY);
  } catch {
    // ignore
  }
}
