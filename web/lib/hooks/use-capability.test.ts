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
    "question-bundles:*",
    "question-bundles:write",
  ])("grants '%s'", (cap) => {
    expect(hasCapability("admin_exam", cap)).toBe(true);
  });

  it.each(["orders:write", "revenue:read", "schools:write", "students:*"])(
    "withholds '%s'",
    (cap) => {
      expect(hasCapability("admin_exam", cap)).toBe(false);
    },
  );
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
