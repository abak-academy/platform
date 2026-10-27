"use client";

import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useTranslation } from "@/lib/i18n";

export const PAGE_SIZES = [20, 50, 100] as const;

interface TablePagerProps {
  total: number;
  /** Zero-based index of the first row on this page. */
  offset: number;
  /** Rows on this page. */
  count: number;
  limit: number;
  onLimitChange: (limit: number) => void;
  canPrev: boolean;
  canNext: boolean;
  onPrev: () => void;
  onNext: () => void;
}

// Table footer in the admin pattern: rows per page on the left, "a–b of N" and
// Prev/Next on the right.
export function TablePager({ total, offset, count, limit, onLimitChange, canPrev, canNext, onPrev, onNext }: TablePagerProps) {
  const { t } = useTranslation();
  const from = count === 0 ? 0 : offset + 1;
  const to = offset + count;

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-4 py-3 text-sm">
      <label className="flex items-center gap-2">
        <span className="text-ink-600">{t("aff_rows_per_page")}</span>
        <Select value={String(limit)} onValueChange={(v) => onLimitChange(Number(v))}>
          <SelectTrigger className="h-8 w-20" aria-label={t("aff_rows_per_page")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {PAGE_SIZES.map((n) => (
              <SelectItem key={n} value={String(n)}>
                {n}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </label>
      <div className="flex items-center gap-3">
        <span className="tabular-nums" data-testid="pager-range">
          {t("aff_range").replace("{from}", String(from)).replace("{to}", String(to)).replace("{total}", String(total))}
        </span>
        <Button variant="outline" size="sm" disabled={!canPrev} onClick={onPrev}>
          {t("aff_prev")}
        </Button>
        <Button variant="outline" size="sm" disabled={!canNext} onClick={onNext}>
          {t("aff_next")}
        </Button>
      </div>
    </div>
  );
}
