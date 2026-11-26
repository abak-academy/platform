"use client";

import { PublicShell } from "@/components/shell/PublicShell";

export default function PublicLayout({ children }: { children: React.ReactNode }) {
  return <PublicShell>{children}</PublicShell>;
}
