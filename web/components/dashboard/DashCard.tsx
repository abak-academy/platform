import Link from "next/link";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

const TONE = {
  gold: "bg-gold-50 text-gold-700",
  sky: "bg-sky-50 text-sky-700",
  teal: "bg-teal-50 text-teal-700",
} as const;

// Prototype .dcard + .dcard-head + .dico + .more.
export function DashCard({
  icon: Icon,
  tone,
  title,
  more,
  children,
}: {
  icon: LucideIcon;
  tone: keyof typeof TONE;
  title: string;
  more?: { href: string; label: string };
  children: React.ReactNode;
}) {
  return (
    <section className="flex min-w-0 flex-col gap-3.5 rounded-[22px] border border-line bg-surface p-5 shadow-card">
      <div className="flex items-center gap-2.5">
        <span data-testid="dash-card-icon" className={cn("grid size-9 flex-none place-items-center rounded-xl", TONE[tone])}>
          <Icon className="size-5" aria-hidden />
        </span>
        <h2 className="flex-1 font-[family-name:var(--font-display)] text-[15px] font-black text-ink-900">{title}</h2>
        {more && (
          <Link href={more.href} className="px-2 py-1 font-[family-name:var(--font-display)] text-[13px] font-bold text-ink-600">
            {more.label}
          </Link>
        )}
      </div>
      {children}
    </section>
  );
}
