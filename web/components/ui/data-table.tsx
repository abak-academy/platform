import * as React from "react";

import { cn } from "@/lib/utils";

export interface DataTableColumn<T> {
  key: string;
  header: React.ReactNode;
  cell: (row: T) => React.ReactNode;
  align?: "left" | "right";
  className?: string;
}

export interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  empty: React.ReactNode;
  stickyHeader?: boolean;
  footer?: React.ReactNode;
  "data-testid"?: string;
}

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  empty,
  stickyHeader = false,
  footer,
  "data-testid": dataTestId,
}: DataTableProps<T>) {
  return (
    <div className="md-card-outlined" data-testid={dataTestId}>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr>
              {columns.map((column) => (
                <th
                  key={column.key}
                  className={cn(
                    "bg-surface-2 px-4 py-3 text-left text-xs font-semibold text-ink-600",
                    column.align === "right" && "text-right",
                    stickyHeader && "sticky top-0 z-10"
                  )}
                >
                  {column.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="px-4 py-8 text-center text-sm text-ink-500">
                  {empty}
                </td>
              </tr>
            ) : (
              rows.map((row) => (
                <tr key={rowKey(row)}>
                  {columns.map((column) => (
                    <td
                      key={column.key}
                      className={cn(
                        "px-4 py-3",
                        column.align === "right" && "text-right",
                        column.className
                      )}
                    >
                      {column.cell(row)}
                    </td>
                  ))}
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
