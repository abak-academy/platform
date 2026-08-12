import { describe, it, expect } from "vitest";
import { adminHomeForRole, redirectForRole } from "./auth-redirect";
import { ADMIN_ROLES, NAV_CONFIG, type UserRole } from "./nav-config";

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
  it("sends every admin role to its own dashboard, not to a work list", () => {
    expect(adminHomeForRole("admin_exam")).toBe("/admin/exam");
    expect(adminHomeForRole("admin_school")).toBe("/admin/school");
    expect(adminHomeForRole("admin_store")).toBe("/admin/store");
  });

  it("leaves super_admin on /admin", () => {
    expect(redirectForRole("super_admin")).toBe("/admin");
    expect(adminHomeForRole("super_admin")).toBe("/admin");
  });
});

describe("admin nav", () => {
  it("puts a Dashboard item first in every admin role's nav", () => {
    for (const role of ADMIN_ROLES) {
      const first = NAV_CONFIG[role][0]?.items[0];
      expect(first, `${role} has no nav items`).toBeDefined();
      expect(first?.exact, `${role}'s first item is not exact-matched`).toBe(true);
      expect(first?.href).toBe(
        role === "super_admin" ? "/admin" : adminHomeForRole(role),
      );
    }
  });

  it("never routes an admin to a nav item that does not exist as a page", () => {
    for (const role of ADMIN_ROLES) {
      const hrefs = NAV_CONFIG[role].flatMap((g) => g.items.map((i) => i.href));
      expect(hrefs).toContain(adminHomeForRole(role));
    }
  });
});
