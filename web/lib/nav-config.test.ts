import { describe, it, expect } from "vitest";
import { ADMIN_EXAM_NAV, ADMIN_SCHOOL_NAV, NAV_CONFIG } from "./nav-config";

describe("EXAM_NAV_ITEMS order: question bank -> tests -> packages -> session monitor", () => {
  it("ADMIN_EXAM_NAV's nav_group_exam group follows the order", () => {
    const group = ADMIN_EXAM_NAV.find((g) => g.titleKey === "nav_group_exam")!;
    expect(group.items.map((i) => i.href)).toEqual([
      "/admin/exam",
      "/admin/exam/questions",
      "/admin/exam/tests",
      "/admin/exam/packages",
      "/admin/exam/monitor",
    ]);
  });

  it("super-admin's nav_group_exam group follows the order (EXAM_NAV_ITEMS then SCHOOL_NAV_ITEMS)", () => {
    const group = NAV_CONFIG.super_admin.find((g) => g.titleKey === "nav_group_exam")!;
    expect(group.items.map((i) => i.href)).toEqual([
      "/admin/exam/questions",
      "/admin/exam/tests",
      "/admin/exam/packages",
      "/admin/exam/monitor",
      "/admin/school/students",
      "/admin/school/reports",
    ]);
  });

  it("ADMIN_SCHOOL_NAV's exam group is unchanged", () => {
    const group = ADMIN_SCHOOL_NAV.find((g) => g.titleKey === "nav_group_exam")!;
    expect(group.items.map((i) => i.href)).toEqual([
      "/admin/school",
      "/admin/school/students",
      "/admin/school/reports",
      "/admin/exam/packages",
    ]);
  });
});
