import type { LucideIcon } from "lucide-react";
import type { AffiliatorProgramStates } from "./types";
import {
  LayoutDashboard,
  Trophy,
  BookOpen,
  Receipt,
  User,
  Package,
  Library,
  Tag,
  BarChart3,
  ClipboardList,
  UserCog,
  Users,
  Calendar,
  Building,
  Settings,
  Store,
  Wallet,
  Link2,
  SlidersHorizontal,
  FileBarChart,
  GraduationCap,
  School,
  ShoppingCart,
  Ticket,
  Languages,
  Moon,
  Globe,
  Handshake,
  Shield,
  MessageSquare,
  Monitor,
  Megaphone,
  Coins,
  KeyRound,
} from "lucide-react";

export type UserRole =
  | "student"
  | "affiliator"
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
  countKey?: string;
}

export interface NavGroup {
  /** Affiliator portal groups that belong to one program; hidden from an account outside it. */
  program?: "affiliate" | "partner";
  titleKey?: keyof (typeof import("./i18n").DICT)["id"];
  icon?: LucideIcon;
  items: NavItem[];
}

export type RoleNavConfig = NavGroup[];

export const ROLE_LABEL_KEYS: Record<UserRole, keyof (typeof import("./i18n").DICT)["id"]> = {
  student: "role_student",
  affiliator: "role_affiliator",
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
      { labelKey: "nav_competition", href: "/exam", icon: Trophy, countKey: "kompetisi" },
      { labelKey: "nav_courses", href: "/courses", icon: BookOpen },
      { labelKey: "nav_billing", href: "/orders", icon: Receipt, countKey: "tagihan" },
    ],
  },
];

const academicItems = (...items: NavItem[]): NavGroup => ({ titleKey: "nav_group_academic", icon: GraduationCap, items });
const PACKAGES: NavItem = { labelKey: "packages", href: "/admin/exam/packages", icon: Calendar };
const MONITOR: NavItem = { labelKey: "session_monitor", href: "/admin/exam/monitor", icon: Monitor };
const TESTS: NavItem = { labelKey: "tests", href: "/admin/exam/tests", icon: ClipboardList };
const QUESTIONS: NavItem = { labelKey: "question_bank", href: "/admin/exam/questions", icon: Library };
const STUDENTS: NavItem = { labelKey: "students", href: "/admin/school/students", icon: Users };
const ANNOUNCEMENTS: NavItem = { labelKey: "nav_announcements", href: "/admin/notifications", icon: Megaphone };
const ORDERS: NavItem = { labelKey: "admin_nav_orders", href: "/admin/orders", icon: Receipt };
const PRODUCTS: NavItem = { labelKey: "admin_nav_products", href: "/admin/products", icon: Package };
const EVENT: NavItem = { labelKey: "admin_nav_product_groupings", href: "/admin/product-groupings", icon: Trophy };
const TESTIMONIALS: NavItem = { labelKey: "admin_nav_testimonials", href: "/admin/testimonials", icon: MessageSquare };
const COURSES: NavItem = { labelKey: "admin_nav_courses", href: "/admin/courses", icon: BookOpen };
const PROMOS: NavItem = { labelKey: "promos", href: "/admin/promos", icon: Tag };

const home = (href: string): NavGroup => ({
  items: [{ labelKey: "nav_dashboard", href, icon: LayoutDashboard, exact: true }],
});

export const CONTENT_MANAGER_NAV: RoleNavConfig = [
  home("/admin/store"),
  academicItems(EVENT, ANNOUNCEMENTS),
  { titleKey: "nav_group_store", icon: Store, items: [ORDERS, PRODUCTS, COURSES, PROMOS] },
];

export const ADMIN_EXAM_NAV: RoleNavConfig = [
  home("/admin/exam"),
  academicItems(EVENT, PACKAGES, MONITOR, TESTS, QUESTIONS, STUDENTS),
  { titleKey: "nav_group_store", icon: Store, items: [PRODUCTS, COURSES] },
];

export const ADMIN_SCHOOL_NAV: RoleNavConfig = [
  home("/admin/school"),
  academicItems(PACKAGES, STUDENTS),
];

const SUPER_ADMIN_NAV: RoleNavConfig = [
  home("/admin"),
  academicItems(
    EVENT,
    TESTIMONIALS,
    PACKAGES,
    MONITOR,
    TESTS,
    QUESTIONS,
    { labelKey: "schools", href: "/admin/system/schools", icon: School },
    STUDENTS,
    ANNOUNCEMENTS
  ),
  {
    titleKey: "nav_group_store",
    icon: Store,
    items: [
      ORDERS,
      PRODUCTS,
      COURSES,
      PROMOS,
      { labelKey: "nav_store_summary", href: "/admin/store", icon: BarChart3, exact: true },
      { labelKey: "revenue", href: "/admin/revenue", icon: Coins },
    ],
  },
  {
    titleKey: "nav_group_affiliate",
    icon: Handshake,
    items: [
      { labelKey: "admin_aff_nav_list", href: "/admin/affiliates", icon: Handshake, exact: true },
      { labelKey: "admin_aff_nav_payouts", href: "/admin/affiliates/payouts", icon: Wallet },
      { labelKey: "admin_aff_nav_reports", href: "/admin/affiliates/reports", icon: FileBarChart },
      { labelKey: "admin_aff_nav_settings", href: "/admin/affiliates/settings", icon: Settings },
    ],
  },
  {
    titleKey: "system",
    icon: Shield,
    items: [
      { labelKey: "accounts", href: "/admin/system/accounts", icon: KeyRound },
      { labelKey: "config", href: "/admin/system/config", icon: Settings },
      { labelKey: "audit", href: "/admin/system/audit", icon: Shield },
    ],
  },
];

// Both program groups are always listed: a group the account does not belong
// to opens a notice instead of data, so the menu never hides what exists.
const AFFILIATOR_NAV: RoleNavConfig = [
  {
    program: "affiliate",
    titleKey: "aff_nav_group_affiliator",
    icon: Handshake,
    items: [
      { labelKey: "aff_nav_dashboard", href: "/affiliate/overview", icon: LayoutDashboard, exact: true },
      { labelKey: "aff_nav_students", href: "/affiliate/students", icon: Users },
      { labelKey: "aff_nav_commissions", href: "/affiliate/commissions", icon: Link2 },
      { labelKey: "aff_nav_vouchers", href: "/affiliate/vouchers", icon: Ticket },
      { labelKey: "aff_nav_wallet", href: "/affiliate/wallet", icon: Wallet },
      { labelKey: "aff_nav_adjustments", href: "/affiliate/adjustments", icon: SlidersHorizontal },
    ],
  },
  {
    program: "partner",
    titleKey: "aff_nav_group_partner",
    icon: Building,
    items: [
      { labelKey: "partner_nav_dashboard", href: "/affiliate/partner", icon: LayoutDashboard, exact: true },
      { labelKey: "partner_nav_schools", href: "/affiliate/partner/schools", icon: School },
      { labelKey: "partner_nav_students", href: "/affiliate/partner/students", icon: GraduationCap },
      { labelKey: "partner_nav_orders", href: "/affiliate/partner/orders", icon: ShoppingCart },
      { labelKey: "partner_nav_vouchers", href: "/affiliate/partner/vouchers", icon: Ticket },
    ],
  },
  {
    items: [{ labelKey: "aff_nav_account", href: "/affiliate/account", icon: UserCog }],
  },
];

export const NAV_CONFIG: Record<UserRole, RoleNavConfig> = {
  student: STUDENT_NAV,
  affiliator: AFFILIATOR_NAV,
  admin_store: CONTENT_MANAGER_NAV,
  admin_exam: ADMIN_EXAM_NAV,
  admin_school: ADMIN_SCHOOL_NAV,
  super_admin: SUPER_ADMIN_NAV,
};

export interface UserMenuItem {
  labelKey: keyof (typeof import("./i18n").DICT)["id"];
  icon: LucideIcon;
  href?: string;
  action?: "theme" | "language";
  countKey?: string;
  hintKey?: keyof (typeof import("./i18n").DICT)["id"];
  external?: boolean;
}

const LANGUAGE_ITEM: UserMenuItem = { labelKey: "user_menu_language", icon: Languages, action: "language" };
const profileMenu = (href: string): UserMenuItem[] => [
  { labelKey: "nav_profile", icon: User, href },
  LANGUAGE_ITEM,
];

const STUDENT_MENU: UserMenuItem[] = [
  { labelKey: "stu_menu_cart", icon: ShoppingCart, href: "/cart", countKey: "cart" },
  { labelKey: "dark_mode", icon: Moon, action: "theme" },
  LANGUAGE_ITEM,
  { labelKey: "stu_menu_profile", icon: User, href: "/profile" },
];

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL;
const ADMIN_MENU: UserMenuItem[] = [
  { labelKey: "user_menu_account_role", icon: UserCog, href: "/admin/profile" },
  LANGUAGE_ITEM,
  ...(SITE_URL
    ? [{ labelKey: "user_menu_public_site", icon: Globe, href: SITE_URL, hintKey: "user_menu_public_site_hint", external: true } as UserMenuItem]
    : []),
];

export const USER_MENU: Record<UserRole, UserMenuItem[]> = {
  student: STUDENT_MENU,
  affiliator: profileMenu("/affiliate/account"),
  admin_store: ADMIN_MENU,
  admin_exam: ADMIN_MENU,
  admin_school: ADMIN_MENU,
  super_admin: ADMIN_MENU,
};

export function roleLabelKey(role?: string): keyof (typeof import("./i18n").DICT)["id"] | undefined {
  if (!role) return undefined;
  return ROLE_LABEL_KEYS[role as UserRole];
}

// The "affiliator" role covers two independently-togglable programs, so the
// label shown to the user must match which one(s) they are actually in.
export function affiliatorRoleLabelKey(programStates?: AffiliatorProgramStates): keyof (typeof import("./i18n").DICT)["id"] {
  if (!programStates) return "role_affiliator";
  const inAffiliate = programStates.affiliate !== "not_enrolled";
  const inPartner = programStates.partner !== "not_enrolled";
  if (inAffiliate && inPartner) return "role_affiliator_partner";
  if (inPartner) return "role_partner";
  return "role_affiliator";
}
