"use client";

import type { UseQueryResult } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { useTranslation } from "@/lib/i18n";

interface PageShape<T> {
  data: T[];
  page: number;
  per_page: number;
  total: number;
}

interface AffiliateListProps<T> {
  query: UseQueryResult<PageShape<T>>;
  columns: DataTableColumn<T>[];
  rowKey: (row: T) => string;
  page: number;
  onPageChange: (page: number) => void;
  empty?: string;
}

export function AffiliateList<T>({ query, columns, rowKey, page, onPageChange, empty }: AffiliateListProps<T>) {
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

  const { data, total, per_page } = query.data;
  const pages = Math.max(1, Math.ceil(total / per_page));
  return (
    <DataTable
      columns={columns}
      rows={data}
      rowKey={rowKey}
      empty={empty ?? t("aff_empty")}
      footer={
        pages > 1 ? (
          <div className="flex items-center justify-between border-t border-line px-4 py-3 text-xs text-ink-500">
            <span>
              {t("aff_page_summary")
                .replace("{page}", String(page))
                .replace("{pages}", String(pages))
                .replace("{total}", String(total))}
            </span>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
                {t("pagination_prev")}
              </Button>
              <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => onPageChange(page + 1)}>
                {t("pagination_next")}
              </Button>
            </div>
          </div>
        ) : null
      }
    />
  );
}
