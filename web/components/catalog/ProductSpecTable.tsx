"use client";

import type { ProductSpec } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";

export interface ProductSpecTableProps {
  specs?: ProductSpec[];
}

export function ProductSpecTable({ specs }: ProductSpecTableProps) {
  const { t } = useTranslation();
  const rows = (specs ?? []).filter((s) => s.value.trim() !== "");
  if (rows.length === 0) return null;

  return (
    <section aria-labelledby="product-specs-heading" className="flex flex-col gap-3">
      <h2
        id="product-specs-heading"
        className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-500"
      >
        {t("product_specs_heading" as any)}
      </h2>
      {/* One column, values on a shared left edge. Two columns fit until a
          value wrapped — "Abak Academy" ran straight into the next column's
          label — and this list sits in the page's narrowest column. */}
      <dl className="text-sm">
        {rows.map((s, i) => (
          <div
            key={`${s.key}-${i}`}
            className="grid grid-cols-[minmax(0,9rem)_minmax(0,1fr)] gap-4 border-b border-line py-2 last:border-b-0"
          >
            <dt className="text-ink-500">{s.label}</dt>
            <dd className="font-medium text-ink-900">{s.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
