"use client";

import type { LucideIcon } from "lucide-react";
import { PageHead } from "@/components/shell/PageHead";

interface DashboardHeroProps {
  /** Kept so existing callers compile; the redesigned head has no icon tile or gradient. */
  icon?: LucideIcon;
  badge: string;
  name: string;
  subtitle: string;
  gradient?: string;
  className?: string;
}

export function DashboardHero({ badge, name, subtitle, className }: DashboardHeroProps) {
  return <PageHead eyebrow={badge} title={name} sub={subtitle} className={className} />;
}
