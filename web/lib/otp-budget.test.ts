import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { adoptResendBudget, clearResendBudget, readResendBudget } from "./otp-budget";

describe("otp-budget", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it("returns the default floor when no budget was adopted", () => {
    expect(readResendBudget()).toBe(60);
  });

  it("resumes the countdown from the remaining seconds", () => {
    adoptResendBudget(120);
    // Pretend 45 seconds elapsed since adoption.
    const at = Number(sessionStorage.getItem("abak-resend-at"));
    sessionStorage.setItem("abak-resend-at", String(at - 45_000));
    expect(readResendBudget()).toBe(75);
  });

  it("clamps to zero once the anchored budget fully elapsed", () => {
    adoptResendBudget(30);
    const at = Number(sessionStorage.getItem("abak-resend-at"));
    sessionStorage.setItem("abak-resend-at", String(at - 120_000));
    expect(readResendBudget()).toBe(0);
  });

  it("clears both keys", () => {
    adoptResendBudget(45);
    clearResendBudget();
    expect(sessionStorage.getItem("abak-resend-after")).toBeNull();
    expect(sessionStorage.getItem("abak-resend-at")).toBeNull();
  });
});
