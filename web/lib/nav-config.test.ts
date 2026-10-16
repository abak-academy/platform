import { describe, expect, it } from "vitest";
import { ADMIN_EXAM_NAV, ADMIN_SCHOOL_NAV, NAV_CONFIG } from "./nav-config";

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
});
