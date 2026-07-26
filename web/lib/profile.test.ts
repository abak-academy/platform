import { describe, it, expect } from "vitest";
import { isProfileComplete } from "./profile";
import type { User } from "./types";

function user(overrides: Partial<User> = {}): User {
  return {
    id: "u1",
    name: "Test",
    role: "student",
    ...overrides,
  };
}

describe("isProfileComplete", () => {
  it("true when school_id and grade are both set", () => {
    expect(isProfileComplete(user({ school_id: "s1", grade: 10 }))).toBe(true);
    expect(isProfileComplete(user({ school_id: "s1", grade: "10" as unknown as number }))).toBe(true);
  });

  it("false when school_id is missing", () => {
    expect(isProfileComplete(user({ school_id: undefined, grade: 10 }))).toBe(false);
  });

  it("false when grade is null", () => {
    expect(isProfileComplete(user({ school_id: "s1", grade: undefined }))).toBe(false);
  });

  it("false when grade is empty string", () => {
    expect(isProfileComplete(user({ school_id: "s1", grade: "" as unknown as number }))).toBe(false);
  });

  it("false for null or undefined user", () => {
    expect(isProfileComplete(null)).toBe(false);
    expect(isProfileComplete(undefined)).toBe(false);
  });

  it("true for non-google user (provider not checked here)", () => {
    // isProfileComplete is provider-agnostic; the gate checks auth_provider separately.
    expect(isProfileComplete(user({ school_id: "s1", grade: 10, auth_provider: "password" }))).toBe(true);
  });

  it("true when unlisted_school_name is set and grade is set (no real school_id)", () => {
    expect(isProfileComplete(user({ grade: 10, unlisted_school_name: "SMA Maju Bersama" }))).toBe(true);
  });

  it("false when neither school_id nor unlisted_school_name is set", () => {
    expect(isProfileComplete(user({ grade: 10 }))).toBe(false);
  });

  it("false when unlisted_school_name is empty string", () => {
    expect(isProfileComplete(user({ grade: 10, unlisted_school_name: "" }))).toBe(false);
  });
});
