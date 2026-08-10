import { describe, it, expect } from "vitest";
import {
  NAV_CONFIG,
  ROLE_LABEL_KEYS,
  roleLabelKey,
  ADMIN_EXAM_NAV,
  ADMIN_SCHOOL_NAV,
  CONTENT_MANAGER_NAV,
} from "@/lib/nav-config";
import { DICT } from "@/lib/i18n";

type Dict = Record<string, string>;
const idDict = DICT.id as Dict;
const enDict = DICT.en as Dict;

// --- FR-FE-15: New i18n keys are present, distinct from role labels ---

describe("FR-FE-15: nav group i18n keys are distinct from role-badge keys", () => {
  it("nav_group_store exists in both id and en with correct values", () => {
    expect(idDict["nav_group_store"]).toBe("Toko");
    expect(enDict["nav_group_store"]).toBe("Store");
  });

  it("nav_group_exam exists in both id and en with correct values", () => {
    expect(idDict["nav_group_exam"]).toBe("Ujian");
    expect(enDict["nav_group_exam"]).toBe("Exam");
  });

  it("nav_group_store value is distinct from role_admin_store", () => {
    // role_admin_store is "Store Manager" (role badge), nav_group_store is "Store" (nav title)
    expect(idDict["nav_group_store"]).not.toBe(idDict["role_admin_store"]);
    expect(enDict["nav_group_store"]).not.toBe(enDict["role_admin_store"]);
  });

  it("nav_group_exam value is distinct from role_admin_exam and role_admin_school", () => {
    expect(idDict["nav_group_exam"]).not.toBe(idDict["role_admin_exam"]);
    expect(idDict["nav_group_exam"]).not.toBe(idDict["role_admin_school"]);
    expect(enDict["nav_group_exam"]).not.toBe(enDict["role_admin_exam"]);
    expect(enDict["nav_group_exam"]).not.toBe(enDict["role_admin_school"]);
  });
});

// --- FR-FE-15b: CONTENT_MANAGER_NAV uses new key ---

describe("FR-FE-15b: CONTENT_MANAGER_NAV titleKey uses nav_group_store", () => {
  it("uses nav_group_store, not role_admin_store", () => {
    expect(CONTENT_MANAGER_NAV[0].titleKey).toBe("nav_group_store");
    expect(CONTENT_MANAGER_NAV[0].titleKey).not.toBe("role_admin_store");
  });
});

// --- FR-FE-16: Exam/School nav groups merged under nav_group_exam ---

describe("FR-FE-16: ADMIN_EXAM_NAV and ADMIN_SCHOOL_NAV use nav_group_exam titleKey", () => {
  it("ADMIN_EXAM_NAV titleKey is nav_group_exam", () => {
    expect(ADMIN_EXAM_NAV[0].titleKey).toBe("nav_group_exam");
    expect(ADMIN_EXAM_NAV[0].titleKey).not.toBe("role_admin_exam");
  });

  it("ADMIN_SCHOOL_NAV titleKey is nav_group_exam", () => {
    expect(ADMIN_SCHOOL_NAV[0].titleKey).toBe("nav_group_exam");
    expect(ADMIN_SCHOOL_NAV[0].titleKey).not.toBe("role_admin_school");
  });
});

describe("FR-FE-16: admin_exam sees exam items under Exam and catalog items under Katalog", () => {
  const examNav = NAV_CONFIG["admin_exam"];
  const examItems = examNav[0].items;

  it("has two nav groups: Exam and Katalog", () => {
    expect(examNav).toHaveLength(2);
  });

  it("second group is Katalog with products and courses links", () => {
    expect(examNav[1].titleKey).toBe("nav_group_catalog");
    const hrefs = examNav[1].items.map((i) => i.href);
    expect(hrefs).toEqual(["/admin/products", "/admin/courses"]);
  });

  it("contains only exam items (tests, packages, question_bank, session_monitor)", () => {
    const hrefs = examItems.map((i) => i.href);
    expect(hrefs).toContain("/admin/exam/tests");
    expect(hrefs).toContain("/admin/exam/packages");
    expect(hrefs).toContain("/admin/exam/questions");
    expect(hrefs).toContain("/admin/exam/monitor");
  });

  it("does NOT contain school items (students, reports)", () => {
    const hrefs = examItems.map((i) => i.href);
    expect(hrefs).not.toContain("/admin/school/students");
    expect(hrefs).not.toContain("/admin/school/reports");
  });
});

describe("FR-FE-16: admin_school sees school items + a scoped Exams link under Exam", () => {
  const schoolNav = NAV_CONFIG["admin_school"];
  const schoolItems = schoolNav[0].items;

  it("has exactly one nav group", () => {
    expect(schoolNav).toHaveLength(1);
  });

  it("contains school items (students, reports) plus the exam list", () => {
    const hrefs = schoolItems.map((i) => i.href);
    expect(hrefs).toContain("/admin/school/students");
    expect(hrefs).toContain("/admin/school/reports");
    expect(hrefs).toContain("/admin/exam/packages");
  });

  it("does NOT contain the content-management exam items", () => {
    const hrefs = schoolItems.map((i) => i.href);
    expect(hrefs).not.toContain("/admin/exam/tests");
    expect(hrefs).not.toContain("/admin/exam/questions");
    expect(hrefs).not.toContain("/admin/exam/monitor");
  });
});

describe("FR-FE-16: super_admin sees all six items under one Exam group", () => {
  const saNav = NAV_CONFIG["super_admin"];
  const examGroup = saNav.find((g) => g.titleKey === "nav_group_exam");

  it("has a single Exam nav group (not two separate groups)", () => {
    const examGroups = saNav.filter((g) => g.titleKey === "nav_group_exam");
    expect(examGroups).toHaveLength(1);
  });

  it("contains all six exam+school items", () => {
    expect(examGroup).toBeDefined();
    const hrefs = examGroup!.items.map((i) => i.href);
    // Exam items
    expect(hrefs).toContain("/admin/exam/tests");
    expect(hrefs).toContain("/admin/exam/packages");
    expect(hrefs).toContain("/admin/exam/questions");
    expect(hrefs).toContain("/admin/exam/monitor");
    // School items
    expect(hrefs).toContain("/admin/school/students");
    expect(hrefs).toContain("/admin/school/reports");
  });

  it("has exactly six items — bulk-exam-order/grant/bulk-register were merged into the exam detail page, not separate nav items", () => {
    expect(examGroup).toBeDefined();
    const hrefs = examGroup!.items.map((i) => i.href);
    expect(hrefs).not.toContain("/admin/exam-grants");
    expect(hrefs).not.toContain("/admin/school/bulk-exam-order");
    expect(hrefs).not.toContain("/admin/school/bulk-register");
    expect(examGroup!.items).toHaveLength(6);
  });
});

// --- FR-FE-18: Role badges are UNCHANGED ---

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

  it("nav section title keys are NOT the same as ROLE_LABEL_KEYS values", () => {
    // This is the core of FR-FE-18: the decoupling is real
    expect(CONTENT_MANAGER_NAV[0].titleKey).not.toBe(ROLE_LABEL_KEYS["admin_store"]);
    expect(ADMIN_EXAM_NAV[0].titleKey).not.toBe(ROLE_LABEL_KEYS["admin_exam"]);
    expect(ADMIN_SCHOOL_NAV[0].titleKey).not.toBe(ROLE_LABEL_KEYS["admin_school"]);
  });
});
