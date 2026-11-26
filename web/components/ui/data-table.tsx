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
  primary?: boolean;
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
  mobileCards?: boolean;
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
        {sub && <small className="block text-xs leading-[1.55] text-ink-600">{sub}</small>}
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
  mobileCards = false,
  onRowClick,
  rowActions,
  "data-testid": dataTestId,
}: DataTableProps<T>) {
  const { t } = useTranslation();
  const colCount = columns.length + (rowActions ? 1 : 0);
  const primaryIndex = Math.max(0, columns.findIndex((column) => column.primary));
  const mc = (classes: string) => (mobileCards ? classes : undefined);
  // Card mode restyles table display; explicit roles keep it a table for WebKit screen readers.
  const role = (r: string) => (mobileCards ? r : undefined);
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
      <div className="relative overflow-x-auto">
        <table role={role("table")} className={cn("w-full border-collapse text-[13.5px] [&_[data-slot=abak-avatar]]:shadow-none!", mc("max-sm:block"))}>
          <thead role={role("rowgroup")} className={mc("max-sm:sr-only")}>
            <tr role={role("row")}>
              {columns.map((column) => (
                <th
                  key={column.key}
                  role={role("columnheader")}
                  className={cn(
                    "bg-surface-sunken px-4 py-3 text-left font-mono text-[10.5px] leading-normal font-semibold tracking-[.1em] whitespace-nowrap text-ink-600 uppercase",
                    column.align === "right" && "text-right",
                    stickyHeader && "sticky top-0 z-10"
                  )}
                >
                  {column.header}
                </th>
              ))}
              {rowActions && (
                <th role={role("columnheader")} className="bg-surface-sunken px-4 py-3">
                  <span className="sr-only">{t("table_actions")}</span>
                </th>
              )}
            </tr>
          </thead>
          <tbody role={role("rowgroup")} className={mc("max-sm:flex max-sm:flex-col max-sm:gap-3 max-sm:p-3")}>
            {rows.length === 0 ? (
              <tr role={role("row")}>
                <td role={role("cell")} colSpan={colCount} className="border-t border-line p-0 text-sm text-ink-600">
                  {typeof empty === "string" ? <EmptyState icon={Inbox} title={empty} /> : empty}
                </td>
              </tr>
            ) : (
              rows.map((row) => (
                <tr
                  key={rowKey(row)}
                  role={role("row")}
                  className={cn(
                    onRowClick && "group/row cursor-pointer",
                    mc(
                      "max-sm:flex max-sm:flex-col max-sm:gap-2 max-sm:rounded-[var(--radius-card)] max-sm:border max-sm:border-line max-sm:bg-card max-sm:p-4"
                    )
                  )}
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
                  {columns.map((column, index) => (
                    <td
                      key={column.key}
                      role={role("cell")}
                      data-label={mobileCards && typeof column.header === "string" ? column.header : undefined}
                      className={cn(
                        mobileCards &&
                          (index === primaryIndex
                            ? "max-sm:order-first max-sm:border-0 max-sm:p-0 max-sm:text-left max-sm:text-base"
                            : "max-sm:flex max-sm:items-baseline max-sm:justify-between max-sm:gap-3 max-sm:border-0 max-sm:p-0 max-sm:text-right max-sm:before:font-mono max-sm:before:text-[10.5px] max-sm:before:font-semibold max-sm:before:tracking-[.1em] max-sm:before:text-ink-600 max-sm:before:uppercase max-sm:before:content-[attr(data-label)]"),
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
                    <td role={role("cell")} className={cn("border-t border-line px-4 py-3 text-right whitespace-nowrap", hover, mc("max-sm:border-0 max-sm:p-0"))}>
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
