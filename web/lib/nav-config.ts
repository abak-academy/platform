import type { LucideIcon } from "lucide-react";
import {
  LayoutDashboard,
  Trophy,
  BookOpen,
  ShoppingBag,
  Receipt,
  User,
  Package,
  Library,
  Tag,
  BarChart3,
  Bell,
  ClipboardList,
  BarChart,
  Users,
  Calendar,
  FileText,
  Building,
  Settings,
  ShieldCheck,
  Store,
} from "lucide-react";

export type UserRole =
  | "student"
  | "admin_store"
  | "admin_exam"
  | "admin_school"
  | "super_admin";

export interface NavItem {
  labelKey: keyof (typeof import("./i18n").DICT)["id"];
  href: string;
  icon: LucideIcon;
  exact?: boolean;
  comingSoon?: boolean;
}

export interface NavGroup {
  titleKey?: keyof (typeof import("./i18n").DICT)["id"];
  items: NavItem[];
}

export type RoleNavConfig = NavGroup[];

export const ROLE_LABEL_KEYS: Record<UserRole, keyof (typeof import("./i18n").DICT)["id"]> = {
  student: "role_student",
  admin_store: "role_admin_store",
  admin_exam: "role_admin_exam",
  admin_school: "role_admin_school",
  super_admin: "role_super_admin",
};

export const ADMIN_ROLES: UserRole[] = [
  "admin_store",
  "admin_exam",
  "admin_school",
  "super_admin",
];

function cs(
  labelKey: keyof (typeof import("./i18n").DICT)["id"],
  href: string,
  icon: LucideIcon
): NavItem {
  return { labelKey, href, icon, comingSoon: true };
}

const STUDENT_NAV: RoleNavConfig = [
  {
    items: [
      { labelKey: "nav_dashboard", href: "/", icon: LayoutDashboard, exact: true },
      { labelKey: "nav_competition", href: "/exam", icon: Trophy },
      { labelKey: "nav_courses", href: "/courses", icon: BookOpen },
      { labelKey: "nav_store", href: "/catalog", icon: ShoppingBag },
      { labelKey: "nav_billing", href: "/orders", icon: Receipt },
      { labelKey: "nav_profile", href: "/profile", icon: User },
    ],
  },
];

const EXAM_NAV_ITEMS: NavItem[] = [
  { labelKey: "tests", href: "/admin/exam/tests", icon: ClipboardList },
  { labelKey: "packages", href: "/admin/exam/packages", icon: Calendar },
  { labelKey: "question_bank", href: "/admin/exam/questions", icon: Library },
  { labelKey: "session_monitor", href: "/admin/exam/monitor", icon: BarChart },
];

const SCHOOL_NAV_ITEMS: NavItem[] = [
  { labelKey: "students", href: "/admin/school/students", icon: Users },
  { labelKey: "reports", href: "/admin/school/reports", icon: FileText },
];

export const CONTENT_MANAGER_NAV: RoleNavConfig = [
  {
    titleKey: "nav_group_store",
    items: [
      { labelKey: "nav_dashboard", href: "/admin/store", icon: LayoutDashboard, exact: true },
      { labelKey: "admin_nav_products", href: "/admin/products", icon: Package },
      { labelKey: "admin_nav_courses", href: "/admin/courses", icon: Library },
      { labelKey: "admin_nav_orders", href: "/admin/orders", icon: Receipt },
      { labelKey: "promos", href: "/admin/promos", icon: Tag },
      { labelKey: "notifications", href: "/admin/notifications", icon: Bell },
    ],
  },
];

export const ADMIN_EXAM_NAV: RoleNavConfig = [
  {
    titleKey: "nav_group_exam",
    items: [
      { labelKey: "nav_dashboard", href: "/admin/exam", icon: LayoutDashboard, exact: true },
      ...EXAM_NAV_ITEMS,
    ],
  },
  {
    titleKey: "nav_group_catalog",
    items: [
      { labelKey: "admin_nav_products", href: "/admin/products", icon: Package },
      { labelKey: "admin_nav_courses", href: "/admin/courses", icon: Library },
    ],
  },
];

export const ADMIN_SCHOOL_NAV: RoleNavConfig = [
  {
    titleKey: "nav_group_exam",
    items: [
      { labelKey: "nav_dashboard", href: "/admin/school", icon: LayoutDashboard, exact: true },
      ...SCHOOL_NAV_ITEMS,
      { labelKey: "packages", href: "/admin/exam/packages", icon: Calendar },
    ],
  },
];

// Store items for super admin. The store dashboard is included: it is the order
// summary, and super_admin is meant to see everything a store admin sees plus
// revenue. Leaving it out left super_admin with no route to the summary at all.
const SUPER_ADMIN_STORE_ITEMS: NavGroup = {
  titleKey: "nav_group_store",
  items: [
    { labelKey: "nav_store_summary", href: "/admin/store", icon: Store, exact: true },
    { labelKey: "admin_nav_products", href: "/admin/products", icon: Package },
    { labelKey: "admin_nav_courses", href: "/admin/courses", icon: Library },
    { labelKey: "admin_nav_orders", href: "/admin/orders", icon: Receipt },
    { labelKey: "promos", href: "/admin/promos", icon: Tag },
    { labelKey: "revenue", href: "/admin/revenue", icon: BarChart3 },
    { labelKey: "notifications", href: "/admin/notifications", icon: Bell },
  ],
};

const SUPER_ADMIN_NAV: RoleNavConfig = [
  {
    items: [
      { labelKey: "nav_dashboard", href: "/admin", icon: LayoutDashboard, exact: true },
    ],
  },
  SUPER_ADMIN_STORE_ITEMS,
  {
    titleKey: "nav_group_exam",
    items: [...EXAM_NAV_ITEMS, ...SCHOOL_NAV_ITEMS],
  },
  {
    titleKey: "system",
    items: [
      { labelKey: "accounts", href: "/admin/system/accounts", icon: Users },
      { labelKey: "schools", href: "/admin/system/schools", icon: Building },
      { labelKey: "config", href: "/admin/system/config", icon: Settings },
      { labelKey: "audit", href: "/admin/system/audit", icon: ShieldCheck },
    ],
  },
];

export const NAV_CONFIG: Record<UserRole, RoleNavConfig> = {
  student: STUDENT_NAV,
  admin_store: CONTENT_MANAGER_NAV,
  admin_exam: ADMIN_EXAM_NAV,
  admin_school: ADMIN_SCHOOL_NAV,
  super_admin: SUPER_ADMIN_NAV,
};

export function roleLabelKey(role?: string): keyof (typeof import("./i18n").DICT)["id"] | undefined {
  if (!role) return undefined;
  return ROLE_LABEL_KEYS[role as UserRole];
}
