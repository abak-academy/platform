"use client";

import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

export interface TableFooterProps {
  summary: React.ReactNode;
  onPrev?: () => void;
  onNext?: () => void;
  hasPrev?: boolean;
  hasNext?: boolean;
  children?: React.ReactNode;
  className?: string;
}

export function TableFooter({ summary, onPrev, onNext, hasPrev = false, hasNext = false, children, className }: TableFooterProps) {
  const { t } = useTranslation();
  return (
    <div className={cn("flex flex-wrap items-center justify-between gap-3 border-t border-line px-4 py-3 text-[13px] text-ink-600", className)}>
      <div className="flex flex-wrap items-center gap-3">
        {children}
        <span aria-live="polite">{summary}</span>
      </div>
      {(onPrev || onNext) && (
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={onPrev} disabled={!hasPrev}>
            {t("pagination_prev")}
          </Button>
          <Button variant="outline" size="sm" onClick={onNext} disabled={!hasNext}>
            {t("pagination_next")}
          </Button>
        </div>
      )}
    </div>
  );
}
