"use client";

import { useEffect, useMemo, useState } from "react";
import { useProductsByIds } from "@/lib/hooks/products";
import type { CheckoutAgreements } from "@/lib/hooks/orders";
import { useTranslation } from "@/lib/i18n";

export interface AgreementChecklistItem {
  product_id: string;
  name: string;
}

export interface AgreementChecklistProps {
  items: AgreementChecklistItem[];
  // Fires on mount and on every checkbox toggle, including once with
  // allAgreed=true and an empty agreements map when no item's product has
  // any configured checkbox — the checkout gate has nothing to block on.
  onChange: (agreements: CheckoutAgreements, allAgreed: boolean) => void;
}

// Shared by the student cart and the partner (mitra) order detail page —
// both checkout flows are required to show and gate on this, per the
// agreed spec. Admin bulk exam order checkout deliberately has no
// equivalent: that path skips the agreement gate server-side.
export function AgreementChecklist({ items, onChange }: AgreementChecklistProps) {
  const { t } = useTranslation();
  const productIds = useMemo(() => items.map((i) => i.product_id), [items]);
  const { byId } = useProductsByIds(productIds);

  const rows = useMemo(() => {
    const out: { productId: string; checkboxId: string; title: string; body: string }[] = [];
    for (const item of items) {
      const product = byId.get(item.product_id);
      for (const checkbox of product?.agreement_checkboxes ?? []) {
        out.push({ productId: item.product_id, checkboxId: checkbox.id, title: checkbox.title, body: checkbox.body });
      }
    }
    return out;
  }, [items, byId]);

  const [checked, setChecked] = useState<Record<string, boolean>>({});

  useEffect(() => {
    const agreements: CheckoutAgreements = {};
    let allAgreed = true;
    for (const row of rows) {
      const isChecked = checked[`${row.productId}:${row.checkboxId}`] ?? false;
      if (!isChecked) allAgreed = false;
      agreements[row.productId] = { ...agreements[row.productId], [row.checkboxId]: isChecked };
    }
    onChange(agreements, allAgreed);
    // onChange is expected to be stable-ish (parent setState); rows/checked
    // are what actually must retrigger this.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, checked]);

  if (rows.length === 0) return null;

  return (
    <div className="mt-4 border-t border-line pt-4" data-testid="agreement-checklist">
      <div className="text-[11px] font-semibold uppercase tracking-[0.05em] text-ink-500">
        {t("cart_agreement_title")}
      </div>
      <div className="mt-2 flex flex-col gap-2">
        {rows.map((row) => {
          const key = `${row.productId}:${row.checkboxId}`;
          return (
            <label
              key={key}
              className="flex items-start gap-2 rounded-lg border border-line p-2 text-xs"
            >
              <input
                type="checkbox"
                className="mt-0.5 size-4 accent-brand-600"
                checked={checked[key] ?? false}
                onChange={(e) => setChecked((prev) => ({ ...prev, [key]: e.target.checked }))}
                aria-label={row.title}
              />
              <span className="leading-5 text-ink-700">{row.body}</span>
            </label>
          );
        })}
      </div>
    </div>
  );
}
