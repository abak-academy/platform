"use client";

import type { UseQueryResult } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Pagination } from "@/components/affiliate/Pagination";
import { useTranslation } from "@/lib/i18n";

interface PageShape<T> {
  data: T[] | null;
  page?: number;
  per_page?: number;
  total?: number;
}

interface AffiliateListProps<T> {
  query: UseQueryResult<PageShape<T>>;
  columns: DataTableColumn<T>[];
  rowKey: (row: T) => string;
  page?: number;
  onPageChange?: (page: number) => void;
  // Shown when the list is empty with no search or filter applied.
  empty?: string;
  // True while a search or filter is active, so an empty result reads as
  // "nothing matches" instead of "nothing yet".
  filtered?: boolean;
  expandedKey?: string | null;
  renderExpanded?: (row: T) => React.ReactNode;
}

export function AffiliateList<T>({ query, columns, rowKey, page = 1, onPageChange, empty, filtered, expandedKey, renderExpanded }: AffiliateListProps<T>) {
  const { t } = useTranslation();

  if (query.isError) {
    return (
      <div role="alert" className="md-card-outlined space-y-3 p-6 text-sm text-ink-600">
        <p>{t("aff_error")}</p>
        <Button variant="outline" size="sm" onClick={() => query.refetch()}>
          {t("aff_retry")}
        </Button>
      </div>
    );
  }
  if (query.isLoading || !query.data) {
    return <div className="p-6 text-sm text-ink-500">{t("aff_loading")}</div>;
  }

  // The API answers [] for an empty list, but a null must never take the page down.
  const data = query.data.data ?? [];
  const { total = data.length, per_page = 0 } = query.data;
  const pages = per_page > 0 ? Math.max(1, Math.ceil(total / per_page)) : 1;
  const first = (page - 1) * per_page + 1;
  const last = Math.min(total, (page - 1) * per_page + data.length);

  return (
    <DataTable
      columns={columns}
      rows={data}
      rowKey={rowKey}
      expandedKey={expandedKey}
      renderExpanded={renderExpanded}
      empty={filtered ? t("aff_no_match") : (empty ?? t("aff_empty"))}
      footer={
        total > 0 && data.length > 0 ? (
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-4 py-3 text-xs text-ink-500">
            <span>
              {t("aff_range_summary")
                .replace("{from}", String(first))
                .replace("{to}", String(last))
                .replace("{total}", String(total))}
            </span>
            {onPageChange && <Pagination page={page} pages={pages} onPageChange={onPageChange} />}
          </div>
        ) : null
      }
    />
  );
}
