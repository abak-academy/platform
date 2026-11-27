import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { Card } from "@/components/ui/card";

export function EmptyState({
  icon: Icon,
  title,
  text,
  action,
  className,
  framed = false,
}: {
  icon: LucideIcon;
  title: string;
  text?: string;
  action?: ReactNode;
  className?: string;
  framed?: boolean;
}) {
  const body = (
    <div className={cn("flex flex-col items-center gap-2 px-4 py-10 text-center text-ink-600", className)}>
      <span className="grid size-14 place-items-center rounded-[20px] bg-ember-50 text-ember-700">
        <Icon className="size-7" aria-hidden />
      </span>
      <h3 className="mt-1.5 type-h3 font-bold tracking-[-0.02em] text-ink-900">{title}</h3>
      {text && <p className="mb-2">{text}</p>}
      {action}
    </div>
  );
  return framed ? <Card className="gap-0 py-0">{body}</Card> : body;
}
