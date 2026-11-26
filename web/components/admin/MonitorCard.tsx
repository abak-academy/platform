"use client";

import Link from "next/link";
import type { LucideIcon } from "lucide-react";
import { StatCard, type StatTone } from "@/components/admin/StatCard";

type Accent = "primary" | "secondary" | "error" | "tertiary";

interface MonitorCardProps {
  testId: string;
  href: string;
  label: string;
  value: number;
  icon: LucideIcon;
  accent?: Accent;
  tone?: StatTone;
}

export function MonitorCard({ testId, href, label, value, icon, accent = "primary", tone }: MonitorCardProps) {
  return (
    <Link
      href={href}
      data-testid={`${testId}-link`}
      className="block rounded-[var(--radius-card)] focus-visible:outline-[2.5px] focus-visible:outline-offset-2 focus-visible:outline-solid focus-visible:outline-brand-600"
    >
      <div data-testid={testId}>
        <StatCard label={label} value={String(value)} accent={accent} tone={tone} icon={icon} />
      </div>
    </Link>
  );
}
