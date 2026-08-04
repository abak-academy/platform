"use client";

import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type { TopProduct, TopProductWithRevenue } from "@/lib/types";

interface TopProductsTableProps {
  products: TopProduct[];
  /**
   * Only super_admin sees money. The prop is narrowed rather than trusted: with
   * `showRevenue` the rows must be TopProductWithRevenue, so a caller holding
   * money-free rows cannot ask for a revenue column at all.
   */
  showRevenue?: boolean;
  /** Ranking bar is drawn against this; pass the first row's metric. */
  emptyLabel?: string;
}

function hasRevenue(p: TopProduct): p is TopProductWithRevenue {
  return typeof (p as TopProductWithRevenue).product_revenue === "number";
}

export function TopProductsTable({ products, showRevenue = false, emptyLabel }: TopProductsTableProps) {
  const { t } = useTranslation();

  if (products.length === 0) {
    return (
      <p className="px-1 py-6 text-center text-sm text-ink-600">
        {emptyLabel ?? t("revenue_no_data")}
      </p>
    );
  }

  const maxQty = Math.max(...products.map((p) => p.qty_sold), 1);

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-line">
            <th className="w-8 px-2 py-2 text-left text-xs font-semibold uppercase tracking-wide text-ink-600">
              #
            </th>
            <th className="px-2 py-2 text-left text-xs font-semibold uppercase tracking-wide text-ink-600">
              {t("th_product")}
            </th>
            <th className="px-2 py-2 text-right text-xs font-semibold uppercase tracking-wide text-ink-600">
              {t("th_qty_sold")}
            </th>
            <th className="px-2 py-2 text-right text-xs font-semibold uppercase tracking-wide text-ink-600">
              {t("th_order_count")}
            </th>
            {showRevenue && (
              <th className="px-2 py-2 text-right text-xs font-semibold uppercase tracking-wide text-ink-600">
                {t("revenue")}
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {products.map((p, i) => (
            <tr key={p.product_id} className="border-b border-line/60 last:border-0">
              <td className="px-2 py-2.5 text-right align-top text-xs font-bold tabular-nums text-ink-600">
                {i + 1}
              </td>
              <td className="px-2 py-2.5 align-top">
                <div className="font-medium">{p.name}</div>
                <div className="mt-0.5 text-xs text-ink-600">{p.product_type}</div>
                {!showRevenue && (
                  // Ranking is by quantity on the money-free view, so the bar
                  // makes the order legible without printing a second number.
                  <div
                    className="mt-1.5 h-1 rounded-full bg-brand-500/70 transition-[width] duration-500 motion-reduce:transition-none"
                    style={{ width: `${Math.max((p.qty_sold / maxQty) * 100, 4)}%` }}
                    aria-hidden
                  />
                )}
              </td>
              <td className="px-2 py-2.5 text-right align-top tabular-nums">{p.qty_sold}</td>
              <td className="px-2 py-2.5 text-right align-top tabular-nums">{p.order_count}</td>
              {showRevenue && (
                <td className="px-2 py-2.5 text-right align-top font-medium tabular-nums">
                  {hasRevenue(p) ? formatRupiah(p.product_revenue) : "—"}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
