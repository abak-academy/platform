import { describe, it, expect } from "vitest";
import { hasCapability } from "./use-capability";

// Go counterpart: backend/internal/service/rbac_test.go
describe("admin_exam capabilities", () => {
  it.each([
    "questions:*",
    "tests:*",
    "products(exam):*",
    "products(exam):read",
    "products(course):*",
    "sections:*",
    "sessions:*",
    "uploads:write",
    "results:read",
    "students:*",
    "students:read",
    "student-passwords:write",
    "student-scope:all",
    "results-answers:read",
  ])("grants '%s'", (cap) => {
    expect(hasCapability("admin_exam", cap)).toBe(true);
  });

  it.each(["orders:write", "revenue:read", "schools:write", "system:admin", "results-workspace:read"])(
    "withholds '%s'",
    (cap) => {
      expect(hasCapability("admin_exam", cap)).toBe(false);
    },
  );
});

// students:* prefix-matches, so the password capability lives outside that namespace.
describe("student account capabilities", () => {
  it("lets admin_school read students but not set passwords", () => {
    expect(hasCapability("admin_school", "students:read")).toBe(true);
    expect(hasCapability("admin_school", "student-passwords:write")).toBe(false);
    expect(hasCapability("admin_school", "student-scope:all")).toBe(false);
  });

  it("lets super_admin do both", () => {
    expect(hasCapability("super_admin", "students:read")).toBe(true);
    expect(hasCapability("super_admin", "student-passwords:write")).toBe(true);
    expect(hasCapability("super_admin", "student-scope:all")).toBe(true);
  });
});

describe("other roles are unchanged", () => {
  it("super_admin keeps the wildcard", () => {
    expect(hasCapability("super_admin", "anything:at:all")).toBe(true);
  });

  it("admin_store keeps orders and loses revenue", () => {
    expect(hasCapability("admin_store", "orders:*")).toBe(true);
    expect(hasCapability("admin_store", "revenue:read")).toBe(false);
  });

  it("an unknown role gets nothing", () => {
    expect(hasCapability("nonsense", "orders:*")).toBe(false);
    expect(hasCapability(undefined, "orders:*")).toBe(false);
  });
});
