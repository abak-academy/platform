import { describe, it, expect } from "vitest";
import { isProfileComplete, missingExamBiodataFields } from "./profile";
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

// missingExamBiodataFields mirrors the backend's exam-registration biodata gate
// (school + grade + dob), which is stricter than isProfileComplete (school +
// grade only, used for the Google-onboarding gate). It exists to warn a
// student on the exam page before they hit the checkout 422.
describe("missingExamBiodataFields", () => {
  it("returns empty when school, grade and dob are all set", () => {
    expect(
      missingExamBiodataFields(user({ school_id: "s1", grade: 10, dob: "2008-01-01" })),
    ).toEqual([]);
  });

  it("names only grade when school and dob are already set", () => {
    expect(
      missingExamBiodataFields(user({ school_id: "s1", dob: "2008-01-01" })),
    ).toEqual(["grade"]);
  });

  it("names only dob when school and grade are already set", () => {
    expect(
      missingExamBiodataFields(user({ school_id: "s1", grade: 10 })),
    ).toEqual(["dob"]);
  });

  it("accepts unlisted_school_name in place of school_id", () => {
    expect(
      missingExamBiodataFields(
        user({ unlisted_school_name: "SMA Test", grade: 10, dob: "2008-01-01" }),
      ),
    ).toEqual([]);
  });

  it("names all three fields for a user with nothing set", () => {
    expect(missingExamBiodataFields(user())).toEqual(["school", "grade", "dob"]);
  });

  it("treats null/undefined user as all fields missing", () => {
    expect(missingExamBiodataFields(null)).toEqual(["school", "grade", "dob"]);
    expect(missingExamBiodataFields(undefined)).toEqual(["school", "grade", "dob"]);
  });
});
