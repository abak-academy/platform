import { afterEach, describe, expect, it, vi } from "vitest";
import { DICT } from "./i18n";
import { ADMIN_ROLES, NAV_CONFIG, USER_MENU, type UserRole } from "./nav-config";

const hrefs = (role: UserRole, titleKey?: string) =>
  NAV_CONFIG[role].find((g) => g.titleKey === titleKey)!.items.map((i) => i.href);
const titles = (role: UserRole) => NAV_CONFIG[role].map((g) => g.titleKey);

describe("admin navigation", () => {
  it("orders super-admin groups Beranda, Akademik, Toko, Affiliator, Sistem", () => {
    expect(titles("super_admin")).toEqual([
      undefined,
      "nav_group_academic",
      "nav_group_store",
      "nav_group_affiliate",
      "system",
    ]);
    expect(NAV_CONFIG.super_admin[0].items.map((i) => i.href)).toEqual(["/admin"]);
  });

  it("orders super-admin Akademik items by workflow", () => {
    expect(hrefs("super_admin", "nav_group_academic")).toEqual([
      "/admin/product-groupings",
      "/admin/exam/packages",
      "/admin/exam/monitor",
      "/admin/exam/tests",
      "/admin/exam/questions",
      "/admin/system/schools",
      "/admin/school/students",
      "/admin/notifications",
    ]);
  });

  it("orders super-admin Toko items without the event entry", () => {
    expect(hrefs("super_admin", "nav_group_store")).toEqual([
      "/admin/orders",
      "/admin/products",
      "/admin/courses",
      "/admin/promos",
      "/admin/store",
      "/admin/revenue",
    ]);
  });

  it("labels the announcements entry with nav_announcements", () => {
    const item = NAV_CONFIG.super_admin
      .flatMap((g) => g.items)
      .find((i) => i.href === "/admin/notifications")!;
    expect(item.labelKey).toBe("nav_announcements");
  });

  it("keeps system navigation ordered as accounts, config, and audit", () => {
    expect(hrefs("super_admin", "system")).toEqual([
      "/admin/system/accounts",
      "/admin/system/config",
      "/admin/system/audit",
    ]);
  });

  it("uses Beranda, Akademik, Toko for admin_exam", () => {
    expect(titles("admin_exam")).toEqual([undefined, "nav_group_academic", "nav_group_store"]);
    expect(NAV_CONFIG.admin_exam[0].items.map((i) => i.href)).toEqual(["/admin/exam"]);
    expect(hrefs("admin_exam", "nav_group_academic")).toEqual([
      "/admin/product-groupings",
      "/admin/exam/packages",
      "/admin/exam/monitor",
      "/admin/exam/tests",
      "/admin/exam/questions",
      "/admin/school/students",
    ]);
    expect(hrefs("admin_exam", "nav_group_store")).toEqual(["/admin/products", "/admin/courses"]);
  });

  it("uses Beranda, Akademik for admin_school", () => {
    expect(titles("admin_school")).toEqual([undefined, "nav_group_academic"]);
    expect(NAV_CONFIG.admin_school[0].items.map((i) => i.href)).toEqual(["/admin/school"]);
    expect(hrefs("admin_school", "nav_group_academic")).toEqual([
      "/admin/exam/packages",
      "/admin/school/students",
    ]);
  });

  it("uses Beranda, Akademik, Toko for admin_store", () => {
    expect(titles("admin_store")).toEqual([undefined, "nav_group_academic", "nav_group_store"]);
    expect(NAV_CONFIG.admin_store[0].items.map((i) => i.href)).toEqual(["/admin/store"]);
    expect(hrefs("admin_store", "nav_group_academic")).toEqual(["/admin/product-groupings", "/admin/notifications"]);
    expect(hrefs("admin_store", "nav_group_store")).toEqual([
      "/admin/orders",
      "/admin/products",
      "/admin/courses",
      "/admin/promos",
    ]);
  });

  it("lists the event entry in Akademik for every role that manages events and never in Toko", () => {
    const event = NAV_CONFIG.super_admin.flatMap((g) => g.items).find((i) => i.href === "/admin/product-groupings")!;
    expect(event.labelKey).toBe("admin_nav_product_groupings");
    expect(DICT.id.admin_nav_product_groupings).toBe("Event");
    for (const role of ["super_admin", "admin_exam", "admin_store"] as const) {
      expect(hrefs(role, "nav_group_academic")[0]).toBe("/admin/product-groupings");
      expect(hrefs(role, "nav_group_store")).not.toContain("/admin/product-groupings");
    }
    expect(NAV_CONFIG.admin_school.flatMap((g) => g.items.map((i) => i.href))).not.toContain("/admin/product-groupings");
  });

  it("never renders a group with zero items and hides school reports", () => {
    for (const role of ADMIN_ROLES) {
      for (const group of NAV_CONFIG[role]) expect(group.items.length).toBeGreaterThan(0);
      expect(NAV_CONFIG[role].flatMap((g) => g.items.map((i) => i.href))).not.toContain(
        "/admin/school/reports",
      );
    }
  });

  it("no longer uses the split exam/school group keys", () => {
    for (const role of ADMIN_ROLES) {
      expect(titles(role)).not.toContain("nav_group_exam");
      expect(titles(role)).not.toContain("nav_group_school");
    }
  });
});

describe("USER_MENU", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.resetModules();
  });

  it("gives every admin role account, language and external public-site items", async () => {
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "https://hub.example.test");
    vi.resetModules();
    const { USER_MENU: menu, ADMIN_ROLES: admins } = await import("./nav-config");
    for (const role of admins) {
      expect(menu[role].map((i) => i.labelKey)).toEqual([
        "user_menu_account_role",
        "user_menu_language",
        "user_menu_public_site",
      ]);
      expect(menu[role][0].href).toBe("/admin/profile");
      expect(menu[role][1].action).toBe("language");
      expect(menu[role][2]).toMatchObject({
        href: "https://hub.example.test",
        hintKey: "user_menu_public_site_hint",
        external: true,
      });
    }
  });

  it("omits the public-site item when NEXT_PUBLIC_SITE_URL is unset or empty", async () => {
    for (const value of [undefined, ""]) {
      vi.stubEnv("NEXT_PUBLIC_SITE_URL", value as string);
      vi.resetModules();
      const { USER_MENU: menu } = await import("./nav-config");
      expect(menu.super_admin.map((i) => i.labelKey)).toEqual([
        "user_menu_account_role",
        "user_menu_language",
      ]);
    }
  });

  it("gives the student exactly Beranda, Kompetisi, Kursus Saya, Tagihan in one ungrouped list", () => {
    expect(NAV_CONFIG.student).toHaveLength(1);
    expect(NAV_CONFIG.student[0].titleKey).toBeUndefined();
    expect(NAV_CONFIG.student[0].items.map((i) => [i.href, i.countKey])).toEqual([
      ["/", undefined],
      ["/exam", "kompetisi"],
      ["/courses", undefined],
      ["/orders", "tagihan"],
    ]);
  });

  it("gives the student Keranjang, theme, Bahasa, Profil & akun", () => {
    expect(USER_MENU.student.map((i) => [i.labelKey, i.href ?? i.action, i.countKey])).toEqual([
      ["stu_menu_cart", "/cart", "cart"],
      ["dark_mode", "theme", undefined],
      ["user_menu_language", "language", undefined],
      ["stu_menu_profile", "/profile", undefined],
    ]);
  });

  it("keeps Profil then Bahasa for the affiliator", () => {
    expect(USER_MENU.affiliator.map((i) => [i.labelKey, i.href ?? i.action])).toEqual([
      ["nav_profile", "/affiliate/account"],
      ["user_menu_language", "language"],
    ]);
  });
});
