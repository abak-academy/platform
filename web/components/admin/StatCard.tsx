import type * as React from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

type StatAccent = "primary" | "secondary" | "error" | "tertiary";
export type StatTone = "brand" | "ember" | "teal" | "sky" | "gold" | "berry" | "danger";

interface StatCardProps {
  label: string;
  value: string;
  icon?: LucideIcon;
  trend?: React.ReactNode;
  accent?: StatAccent;
  tone?: StatTone;
  className?: string;
}

const ACCENT_TONE: Record<StatAccent, StatTone> = {
  primary: "brand",
  secondary: "teal",
  tertiary: "sky",
  error: "danger",
};

const TONE: Record<StatTone, { box: string; icon: string }> = {
  brand: { box: "bg-brand-50", icon: "text-brand-ink" },
  ember: { box: "bg-ember-50", icon: "text-ember-700" },
  teal: { box: "bg-teal-50", icon: "text-teal-700" },
  sky: { box: "bg-sky-50", icon: "text-sky-700" },
  gold: { box: "bg-gold-50", icon: "text-gold-700" },
  berry: { box: "bg-berry-50", icon: "text-berry-700" },
  danger: { box: "bg-danger-50", icon: "text-danger-600" },
};

export function StatCard({ label, value, icon: Icon, trend, accent = "primary", tone, className }: StatCardProps) {
  const t = TONE[tone ?? ACCENT_TONE[accent]];
  return (
    <div
      className={cn(
        "relative flex min-w-0 flex-col gap-1 overflow-hidden rounded-[var(--radius-card)] border border-transparent p-[18px] transition-transform duration-300 ease-spring hover:-translate-y-[3px]",
        t.box,
        className
      )}
    >
      {Icon ? (
        <div className={cn("mb-2 grid size-10 place-items-center rounded-[13px] bg-surface", t.icon)}>
          <Icon size={20} />
        </div>
      ) : null}
      <div className="type-label-sm text-ink-700">{label}</div>
      <div className="font-[family-name:var(--font-display)] text-[28px] leading-[1.15] font-extrabold tracking-[-0.03em] text-ink-900 max-narrow:text-[22px]">
        {value}
      </div>
      {trend ? <div className="text-xs leading-[1.55] text-ink-600">{trend}</div> : null}
    </div>
  );
}
