"use client";

import { useEffect, useState } from "react";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/lib/hooks/auth";
import type { UserRole } from "@/lib/nav-config";

// Mirrors internal/service/rbac.go. The server is the real boundary — this only
// decides what to render, so drift here is a cosmetic bug, not a hole.
const ROLE_CAPABILITIES: Record<string, string[]> = {
  student: [],
  admin_exam: ["questions:*", "tests:*", "products(exam):*", "sessions:*", "uploads:write"],
  admin_store: ["products(book|course):write", "sections:*", "orders:*", "promos:*", "notifications:*", "uploads:write"],
  admin_school: ["students:*", "results:read", "bulk-exam-orders:write", "products(exam):read"],
  super_admin: ["*", "schools:write"],
};

export function hasCapability(role: string | undefined, required: string): boolean {
  if (!role) return false;
  const caps = ROLE_CAPABILITIES[role];
  if (!caps) return false;
  return caps.some(
    (cap) => cap === "*" || cap === required || (cap.endsWith(":*") && required.startsWith(cap.slice(0, -1))),
  );
}

export interface ResolvedAdminRole {
  role: UserRole | undefined;
  hydrated: boolean;
  meIsError: boolean;
}

// Single resolver for the admin nav and the route guards — if they disagree
// about who you are, the sidebar shows a link the page then refuses to open.
export function useResolvedAdminRole(): ResolvedAdminRole {
  const token = useAuthStore((s) => s.token);
  const storeUser = useAuthStore((s) => s.user);
  const [hydrated, setHydrated] = useState(false);

  const storeRole = storeUser?.role as UserRole | undefined;
  const needsMeFetch = hydrated && !!token && !storeRole;
  const me = useMe({ enabled: needsMeFetch });

  useEffect(() => {
    setHydrated(true);
  }, []);

  return {
    role: storeRole ?? (me.data?.role as UserRole | undefined),
    hydrated,
    meIsError: me.isError,
  };
}

export function useHasCapability(required: string): boolean {
  const { role } = useResolvedAdminRole();
  return hasCapability(role, required);
}
