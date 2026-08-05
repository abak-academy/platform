import { describe, it, expect } from "vitest";
import { redirectForRole, adminHomeForRole } from "./auth-redirect";
import type { UserRole } from "./nav-config";

describe("redirectForRole", () => {
  it("returns '/' for student", () => {
    expect(redirectForRole("student")).toBe("/");
  });

  it("returns '/admin' for every admin role", () => {
    const adminRoles: UserRole[] = [
      "admin_store",
      "admin_exam",
      "admin_school",
      "super_admin",
    ];
    for (const role of adminRoles) {
      expect(redirectForRole(role)).toBe("/admin");
    }
  });

  it("defaults to '/' for unknown or missing role", () => {
    expect(redirectForRole("unknown")).toBe("/");
    expect(redirectForRole(undefined)).toBe("/");
    expect(redirectForRole(null)).toBe("/");
  });
});

describe("adminHomeForRole", () => {
  it("sends admin_store to /admin/store", () => {
    expect(adminHomeForRole("admin_store")).toBe("/admin/store");
  });

  it("sends admin_exam to first exam item", () => {
    expect(adminHomeForRole("admin_exam")).toBe("/admin/exam/tests");
  });

  it("sends admin_school to first coming-soon school item", () => {
    expect(adminHomeForRole("admin_school")).toBe("/admin/school/students");
  });

  // The store summary is now the first non-/admin item in super_admin's nav.
  // In practice super_admin never takes this path — admin/page.tsx only calls
  // adminHomeForRole for other roles — but the helper's contract is "first live
  // admin item", and that item changed.
  it("sends super_admin to first live admin item", () => {
    expect(adminHomeForRole("super_admin")).toBe("/admin/store");
  });
});
