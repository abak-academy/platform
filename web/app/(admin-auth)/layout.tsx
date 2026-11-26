"use client";

import { AuthShell } from "@/components/auth/AuthShell";

export default function AdminAuthLayout({ children }: { children: React.ReactNode }) {
  return <AuthShell variant="admin">{children}</AuthShell>;
}
