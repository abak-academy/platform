import { describe, it, expect, beforeEach } from "vitest";
import { clearManualReferral, readReferralFields, storeManualReferral } from "./referral";

describe("referral fields", () => {
  beforeEach(() => {
    sessionStorage.clear();
    document.cookie = "aff_ref=; path=/; max-age=0";
  });

  it("sends the cookie alone when the typed code matches the prefill", () => {
    document.cookie = "aff_ref=BUDI10; path=/";
    storeManualReferral("budi10");

    expect(readReferralFields()).toEqual({ referral_code_cookie: "BUDI10" });
  });

  it("sends a differing typed code as manual next to the cookie", () => {
    document.cookie = "aff_ref=BUDI10; path=/";
    storeManualReferral(" sari22 ");

    expect(readReferralFields()).toEqual({ referral_code: "SARI22", referral_code_cookie: "BUDI10" });
  });

  it("sends nothing when there is no code and no cookie", () => {
    storeManualReferral("");

    expect(readReferralFields()).toEqual({});
  });

  it("forgets the manual code once cleared", () => {
    storeManualReferral("SARI22");
    clearManualReferral();

    expect(readReferralFields()).toEqual({});
  });
});
