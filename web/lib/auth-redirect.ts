import { ADMIN_ROLES, type UserRole } from "./nav-config";

export function redirectForRole(role?: string | null): string {
  if (ADMIN_ROLES.includes(role as UserRole)) return "/admin";
  return "/";
}

// Explicit per role, not a nav scan — the scan is how admin_exam ended up on /admin/exam/tests.
const ADMIN_HOME: Record<UserRole, string> = {
  student: "/",
  admin_store: "/admin/store",
  admin_exam: "/admin/exam",
  admin_school: "/admin/school",
  super_admin: "/admin",
};

export function adminHomeForRole(role: UserRole): string {
  return ADMIN_HOME[role] ?? "/admin/products";
}

// Where a session ends. Admins sign in through their own page, so dropping them
// on the student login after logout stranded them on a form that is not theirs.
export function loginPathForRole(role?: string | null): string {
  if (ADMIN_ROLES.includes(role as UserRole)) return "/admin/login";
  return "/login";
}
