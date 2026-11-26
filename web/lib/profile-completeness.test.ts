import { describe, it, expect } from "vitest";
import { isProfileComplete, missingProfileFields } from "./profile-completeness";
import type { User } from "@/lib/types";

const completeProfile: User = {
  id: "u1",
  name: "Test User",
  email: "student@example.com",
  phone: "081200000000",
  school_id: "school-1",
  grade: 12,
  dob: "2008-01-01",
};

describe("missingProfileFields", () => {
  it("reports every field missing when there is no profile yet", () => {
    expect(missingProfileFields(undefined)).toEqual(["name", "email", "phone", "school", "grade", "dob"]);
  });

  it("reports nothing missing once name/email/phone/school/grade/dob are all set", () => {
    expect(missingProfileFields(completeProfile)).toEqual([]);
  });

  it("accepts an unlisted school in place of school_id", () => {
    const profile: User = { ...completeProfile, school_id: undefined, unlisted_school_name: "SMA Luar Daftar" };
    expect(missingProfileFields(profile)).toEqual([]);
  });

  it("reports only the fields that are actually missing", () => {
    const profile: User = { ...completeProfile, grade: undefined, dob: undefined };
    expect(missingProfileFields(profile)).toEqual(["grade", "dob"]);
  });

  it("treats a whitespace-only name, email, or phone as missing", () => {
    const profile: User = { ...completeProfile, name: "   ", email: "  ", phone: " " };
    expect(missingProfileFields(profile)).toEqual(["name", "email", "phone"]);
  });
});

describe("isProfileComplete", () => {
  it("is true once nothing is missing", () => {
    expect(isProfileComplete(completeProfile)).toBe(true);
  });

  it("is false when any field is missing", () => {
    expect(isProfileComplete({ ...completeProfile, phone: undefined })).toBe(false);
  });
});
