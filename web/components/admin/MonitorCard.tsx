"use client";

import Link from "next/link";
import type { LucideIcon } from "lucide-react";
import { StatCard } from "@/components/admin/StatCard";

type Accent = "primary" | "secondary" | "error" | "tertiary";

interface MonitorCardProps {
  testId: string;
  href: string;
  label: string;
  value: number;
  icon: LucideIcon;
  accent?: Accent;
}

export function MonitorCard({ testId, href, label, value, icon, accent = "primary" }: MonitorCardProps) {
  return (
    <Link
      href={href}
      data-testid={`${testId}-link`}
      className="block rounded-[20px] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
      style={{ outlineColor: "var(--md-sys-color-primary)" }}
    >
      <div data-testid={testId}>
        <StatCard label={label} value={String(value)} accent={accent} icon={icon} />
      </div>
    </Link>
  );
}
