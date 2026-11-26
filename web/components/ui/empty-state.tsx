import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

export function EmptyState({
  icon: Icon,
  title,
  text,
  action,
  className,
}: {
  icon: LucideIcon;
  title: string;
  text?: string;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center gap-2 px-4 py-10 text-center text-ink-500", className)}>
      <span className="grid size-14 place-items-center rounded-2xl bg-ember-50 text-ember-700">
        <Icon className="size-6" aria-hidden />
      </span>
      <h3 className="mt-1.5 type-h3 text-ink-900">{title}</h3>
      {text && <p className="mb-2">{text}</p>}
      {action}
    </div>
  );
}
