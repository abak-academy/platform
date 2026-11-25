import { describe, it, expect } from "vitest";
import { NAV_CONFIG, ROLE_LABEL_KEYS, roleLabelKey } from "@/lib/nav-config";
import { DICT } from "@/lib/i18n";

type Dict = Record<string, string>;
const idDict = DICT.id as Dict;
const enDict = DICT.en as Dict;

const hrefsOf = (role: keyof typeof NAV_CONFIG) =>
  NAV_CONFIG[role].flatMap((g) => g.items.map((i) => i.href));

describe("FR-FE-15: nav group i18n keys are distinct from role-badge keys", () => {
  it("nav_group_store and nav_group_academic exist in both locales", () => {
    expect(idDict["nav_group_store"]).toBe("Toko");
    expect(enDict["nav_group_store"]).toBe("Store");
    expect(idDict["nav_group_academic"]).toBe("Akademik");
    expect(enDict["nav_group_academic"]).toBe("Academics");
  });

  it("group titles differ from the role badges", () => {
    for (const dict of [idDict, enDict]) {
      expect(dict["nav_group_store"]).not.toBe(dict["role_admin_store"]);
      expect(dict["nav_group_academic"]).not.toBe(dict["role_admin_exam"]);
      expect(dict["nav_group_academic"]).not.toBe(dict["role_admin_school"]);
    }
  });
});

describe("FR-FE-16: role scoping survives the Akademik merge", () => {
  it("admin_exam carries the exam tools and students but no school reports", () => {
    const hrefs = hrefsOf("admin_exam");
    expect(hrefs).toEqual(
      expect.arrayContaining([
        "/admin/exam/tests",
        "/admin/exam/packages",
        "/admin/exam/questions",
        "/admin/exam/monitor",
        "/admin/school/students",
      ])
    );
    expect(hrefs).not.toContain("/admin/school/reports");
  });

  it("admin_school sees students and the exam list, not the content tools", () => {
    const hrefs = hrefsOf("admin_school");
    expect(hrefs).toEqual(expect.arrayContaining(["/admin/school/students", "/admin/exam/packages"]));
    for (const href of ["/admin/exam/tests", "/admin/exam/questions", "/admin/exam/monitor", "/admin/school/reports"]) {
      expect(hrefs).not.toContain(href);
    }
  });

  it("super_admin keeps System ordered and limited to platform administration", () => {
    const system = NAV_CONFIG["super_admin"].find((g) => g.titleKey === "system");
    expect(system?.items.map((i) => i.href)).toEqual([
      "/admin/system/accounts",
      "/admin/system/config",
      "/admin/system/audit",
    ]);
  });
});

describe("FR-FE-18: role badge keys are unchanged", () => {
  it("ROLE_LABEL_KEYS still uses role_admin_store, role_admin_exam, role_admin_school", () => {
    expect(ROLE_LABEL_KEYS["admin_store"]).toBe("role_admin_store");
    expect(ROLE_LABEL_KEYS["admin_exam"]).toBe("role_admin_exam");
    expect(ROLE_LABEL_KEYS["admin_school"]).toBe("role_admin_school");
  });

  it("roleLabelKey returns the correct role-badge keys", () => {
    expect(roleLabelKey("admin_store")).toBe("role_admin_store");
    expect(roleLabelKey("admin_exam")).toBe("role_admin_exam");
    expect(roleLabelKey("admin_school")).toBe("role_admin_school");
    expect(roleLabelKey("super_admin")).toBe("role_super_admin");
    expect(roleLabelKey("student")).toBe("role_student");
  });
});
