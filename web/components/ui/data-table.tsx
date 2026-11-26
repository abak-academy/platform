import * as React from "react";
import { Inbox } from "lucide-react";

import { cn } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";
import { EmptyState } from "@/components/ui/empty-state";

export interface DataTableColumn<T> {
  key: string;
  header: React.ReactNode;
  cell: (row: T) => React.ReactNode;
  align?: "left" | "right";
  strong?: boolean;
  nowrap?: boolean;
  className?: string;
}

export interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  empty: React.ReactNode;
  surface?: "card" | "plain";
  stickyHeader?: boolean;
  footer?: React.ReactNode;
  onRowClick?: (row: T) => void;
  rowActions?: (row: T) => React.ReactNode;
  "data-testid"?: string;
}

const INTERACTIVE = "a,button,input,select,textarea,label,[role=menuitem],[role=checkbox],[role=switch]";

export function TableItem({
  thumb,
  title,
  sub,
}: {
  thumb?: React.ReactNode;
  title: React.ReactNode;
  sub?: React.ReactNode;
}) {
  return (
    <span className="inline-flex items-center gap-2.5 text-ink-900">
      {thumb}
      <span>
        <b className="block font-[family-name:var(--font-display)] font-bold">{title}</b>
        {sub && <small className="block text-xs text-ink-600">{sub}</small>}
      </span>
    </span>
  );
}

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  empty,
  surface = "card",
  stickyHeader = false,
  footer,
  onRowClick,
  rowActions,
  "data-testid": dataTestId,
}: DataTableProps<T>) {
  const { t } = useTranslation();
  const colCount = columns.length + (rowActions ? 1 : 0);
  const hover = onRowClick && "group-hover/row:bg-brand-50";
  return (
    <div
      className={
        surface === "card"
          ? "min-w-0 overflow-hidden rounded-[var(--radius-card)] border border-line bg-surface shadow-card"
          : undefined
      }
      data-testid={dataTestId}
    >
      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-[13.5px]">
          <thead>
            <tr>
              {columns.map((column) => (
                <th
                  key={column.key}
                  className={cn(
                    "bg-surface-sunken px-4 py-3 text-left font-mono text-[10.5px] font-semibold tracking-[.1em] whitespace-nowrap text-ink-600 uppercase",
                    column.align === "right" && "text-right",
                    stickyHeader && "sticky top-0 z-10"
                  )}
                >
                  {column.header}
                </th>
              ))}
              {rowActions && (
                <th className="bg-surface-sunken px-4 py-3">
                  <span className="sr-only">{t("table_actions")}</span>
                </th>
              )}
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={colCount} className="border-t border-line p-0 text-sm text-ink-600">
                  {typeof empty === "string" ? <EmptyState icon={Inbox} title={empty} /> : empty}
                </td>
              </tr>
            ) : (
              rows.map((row) => (
                <tr
                  key={rowKey(row)}
                  className={cn(onRowClick && "group/row cursor-pointer")}
                  tabIndex={onRowClick ? 0 : undefined}
                  onClick={
                    onRowClick
                      ? (e) => {
                          if (!e.currentTarget.contains(e.target as Node)) return;
                          if ((e.target as HTMLElement).closest(INTERACTIVE)) return;
                          onRowClick(row);
                        }
                      : undefined
                  }
                  onKeyDown={
                    onRowClick
                      ? (e) => {
                          if (e.target !== e.currentTarget) return;
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            onRowClick(row);
                          }
                        }
                      : undefined
                  }
                >
                  {columns.map((column) => (
                    <td
                      key={column.key}
                      className={cn(
                        "border-t border-line px-4 py-3 text-ink-700",
                        (column.nowrap || column.strong || column.align === "right") && "whitespace-nowrap",
                        column.strong && "font-bold text-ink-900",
                        column.align === "right" && "text-right",
                        hover,
                        column.className
                      )}
                    >
                      {column.cell(row)}
                    </td>
                  ))}
                  {rowActions && (
                    <td className={cn("border-t border-line px-4 py-3 text-right whitespace-nowrap", hover)}>
                      {rowActions(row)}
                    </td>
                  )}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
      {footer}
    </div>
  );
}
