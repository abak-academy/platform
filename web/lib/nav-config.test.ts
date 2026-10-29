import { describe, expect, it } from "vitest";
import { adminHomeForRole, loginPathForRole, redirectForRole } from "./auth-redirect";
import { ADMIN_EXAM_NAV, ADMIN_ROLES, ADMIN_SCHOOL_NAV, NAV_CONFIG } from "./nav-config";

describe("admin navigation", () => {
  it("keeps super-admin exam navigation limited to exam tools", () => {
    const group = NAV_CONFIG.super_admin.find((item) => item.titleKey === "nav_group_exam")!;
    expect(group.items.map((item) => item.href)).toEqual([
      "/admin/exam/questions",
      "/admin/exam/tests",
      "/admin/exam/packages",
      "/admin/exam/monitor",
    ]);
  });

  it("groups schools and students together for super-admin", () => {
    const group = NAV_CONFIG.super_admin.find((item) => item.titleKey === "nav_group_school")!;
    expect(group.items.map((item) => item.href)).toEqual([
      "/admin/system/schools",
      "/admin/school/students",
    ]);
  });

  it("keeps system navigation ordered as accounts, config, and audit", () => {
    const group = NAV_CONFIG.super_admin.find((item) => item.titleKey === "system")!;
    expect(group.items.map((item) => item.href)).toEqual([
      "/admin/system/accounts",
      "/admin/system/config",
      "/admin/system/audit",
    ]);
  });

  it("gives affiliators their own super-admin menu, apart from System", () => {
    const group = NAV_CONFIG.super_admin.find((item) => item.titleKey === "nav_group_affiliate")!;
    expect(group.items.map((item) => item.href)).toEqual(["/admin/affiliators", "/admin/affiliators/reports"]);
    const system = NAV_CONFIG.super_admin.find((item) => item.titleKey === "system")!;
    expect(system.items.map((item) => item.href)).not.toContain("/admin/affiliators");
    for (const role of ["admin_school", "admin_exam", "admin_store"] as const) {
      expect(NAV_CONFIG[role].flatMap((g) => g.items.map((i) => i.href))).not.toContain("/admin/affiliators");
    }
  });

  it("hides school reports from every admin sidebar", () => {
    for (const role of ["admin_school", "super_admin"] as const) {
      expect(NAV_CONFIG[role].flatMap((group) => group.items.map((item) => item.href))).not.toContain(
        "/admin/school/reports",
      );
    }
  });

  it("keeps the existing exam and school-admin destinations otherwise unchanged", () => {
    expect(ADMIN_EXAM_NAV[0].items.map((item) => item.href)).toEqual([
      "/admin/exam",
      "/admin/exam/questions",
      "/admin/exam/tests",
      "/admin/exam/packages",
      "/admin/exam/monitor",
      "/admin/school/students",
    ]);
    expect(ADMIN_SCHOOL_NAV[0].items.map((item) => item.href)).toEqual([
      "/admin/school",
      "/admin/school/students",
      "/admin/exam/packages",
    ]);
  });

  it("offers the students roster to admin_exam", () => {
    const hrefs = ADMIN_EXAM_NAV.flatMap((group) => group.items.map((item) => item.href));
    expect(hrefs).toContain("/admin/school/students");
    expect(NAV_CONFIG.admin_exam.flatMap((group) => group.items.map((item) => item.href))).toEqual(hrefs);
  });

  it("gives affiliators their own pages and nothing from the other admin areas", () => {
    expect(ADMIN_ROLES).toContain("affiliator");
    const hrefs = NAV_CONFIG.affiliator.flatMap((group) => group.items.map((item) => item.href));
    expect(hrefs).toEqual([
      "/admin/affiliate",
      "/admin/affiliate/students",
      "/admin/affiliate/orders",
      "/admin/affiliate/promos",
      "/admin/profile",
    ]);
  });

  it("signs affiliators in and out through the admin pages", () => {
    expect(adminHomeForRole("affiliator")).toBe("/admin/affiliate");
    expect(redirectForRole("affiliator")).toBe("/admin");
    expect(loginPathForRole("affiliator")).toBe("/admin/login");
  });

  // The sidebar marks an item active for its whole sub-tree unless it is exact, so a
  // parent link next to its own child would light up together with it.
  it("never lets a sidebar item stay lit on a sibling's page", () => {
    for (const [role, groups] of Object.entries(NAV_CONFIG)) {
      const items = groups.flatMap((g) => g.items);
      for (const parent of items) {
        for (const child of items) {
          if (parent === child || parent.exact) continue;
          expect(child.href.startsWith(parent.href + "/"), `${role}: "${parent.href}" would stay active on "${child.href}"`).toBe(false);
        }
      }
    }
  });
});
