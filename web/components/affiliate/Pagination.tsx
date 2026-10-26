"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

// 1 … 4 5 [6] 7 8 … 20: always the first and last page, two either side of the
// current one, and an ellipsis wherever pages are skipped.
export function pageItems(page: number, pages: number): (number | "gap")[] {
  const wanted = new Set([1, pages, page - 2, page - 1, page, page + 1, page + 2]);
  const sorted = [...wanted].filter((n) => n >= 1 && n <= pages).sort((a, b) => a - b);
  const items: (number | "gap")[] = [];
  sorted.forEach((n, i) => {
    if (i > 0 && n - sorted[i - 1] > 1) items.push("gap");
    items.push(n);
  });
  return items;
}

interface PaginationProps {
  page: number;
  pages: number;
  onPageChange: (page: number) => void;
}

export function Pagination({ page, pages, onPageChange }: PaginationProps) {
  const { t } = useTranslation();
  if (pages <= 1) return null;
  return (
    <nav aria-label={t("aff_pagination_label")} className="flex flex-wrap items-center gap-1">
      <Button variant="outline" size="icon" className="size-8" disabled={page <= 1} aria-label={t("pagination_prev")} onClick={() => onPageChange(page - 1)}>
        <ChevronLeft className="size-4" aria-hidden />
      </Button>
      {pageItems(page, pages).map((item, i) =>
        item === "gap" ? (
          <span key={`gap-${i}`} aria-hidden className="px-1 text-ink-400">
            …
          </span>
        ) : (
          <Button
            key={item}
            variant={item === page ? "default" : "outline"}
            size="sm"
            className={cn("h-8 min-w-8 px-2 tabular-nums")}
            aria-current={item === page ? "page" : undefined}
            aria-label={`${item}`}
            onClick={() => item !== page && onPageChange(item)}
          >
            {item}
          </Button>
        ),
      )}
      <Button variant="outline" size="icon" className="size-8" disabled={page >= pages} aria-label={t("pagination_next")} onClick={() => onPageChange(page + 1)}>
        <ChevronRight className="size-4" aria-hidden />
      </Button>
    </nav>
  );
}
