"use client";

import { DataTable } from "@/components/ui/data-table";
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
    <DataTable
      surface="plain"
      rows={products}
      rowKey={(p) => p.product_id}
      empty=""
      columns={[
        { key: "rank", header: "#", className: "w-8 text-xs font-bold tabular-nums text-ink-600", cell: (p) => products.indexOf(p) + 1 },
        {
          key: "name",
          header: t("th_product"),
          cell: (p) => (
            <>
              <div className="font-bold text-ink-900">{p.name}</div>
              <div className="mt-0.5 text-xs text-ink-600">{p.product_type}</div>
              {!showRevenue && (
                <div
                  className="mt-1.5 h-2.5 rounded-full bg-gradient-to-r from-ember-600 to-gold-500 transition-[width] duration-500 motion-reduce:transition-none"
                  style={{ width: `${Math.max((p.qty_sold / maxQty) * 100, 4)}%` }}
                  aria-hidden
                />
              )}
            </>
          ),
        },
        { key: "qty", header: t("th_qty_sold"), align: "right", className: "tabular-nums", cell: (p) => p.qty_sold },
        { key: "orders", header: t("th_order_count"), align: "right", className: "tabular-nums", cell: (p) => p.order_count },
        ...(showRevenue
          ? [{ key: "revenue", header: t("revenue"), align: "right" as const, strong: true, className: "tabular-nums", cell: (p: TopProduct) => (hasRevenue(p) ? formatRupiah(p.product_revenue) : "—") }]
          : []),
      ]}
    />
  );
}
