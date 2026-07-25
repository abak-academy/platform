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
    <section className="rounded-lg border border-line bg-surface p-5">
      <h2 className="mb-4 font-serif text-lg font-semibold text-ink-900">
        {t("product_specs_heading" as any)}
      </h2>
      <dl className="flex flex-col gap-3 text-sm">
        {rows.map((s, i) => (
          <div key={`${s.key}-${i}`} className="grid grid-cols-[minmax(0,180px)_1fr] gap-4">
            <dt className="text-ink-500">{s.label}</dt>
            <dd className="text-ink-900">{s.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
