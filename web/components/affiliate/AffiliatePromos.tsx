"use client";

import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { formatRupiah } from "@/lib/format";
import { formatAffDate } from "@/lib/affiliate-labels";
import { useAffiliatePromos } from "@/lib/hooks/affiliate";
import { useTranslation } from "@/lib/i18n";
import type { AffiliatePromo } from "@/lib/types";

function discountText(p: AffiliatePromo): string {
  if (p.discount_percent != null) return `${p.discount_percent}%`;
  return p.discount_amount != null ? formatRupiah(p.discount_amount) : "";
}

export function AffiliatePromos() {
  const { t } = useTranslation();
  const promos = useAffiliatePromos();
  const items = promos.data?.data ?? [];

  const copy = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code);
      toast.success(t("aff_promos_copied"));
    } catch {
      toast.error(t("aff_error"));
    }
  };

  return (
    <section className="md-card-outlined space-y-3 p-5">
      <div>
        <h2 className="font-semibold text-ink-900">{t("aff_promos_title")}</h2>
        <p className="text-xs text-ink-500">{t("aff_promos_desc")}</p>
      </div>
      {items.length === 0 ? (
        <p className="text-sm text-ink-600">{t("aff_promos_empty")}</p>
      ) : (
        <ul className="divide-y divide-line">
          {items.map((p) => (
            <li key={p.id} className="flex flex-wrap items-center justify-between gap-3 py-2 text-sm">
              <div>
                <div className="font-mono font-bold tracking-wider text-ink-900">{p.code}</div>
                <div className="text-xs text-ink-500">
                  {discountText(p)}
                  {p.min_order_amount ? ` · min ${formatRupiah(p.min_order_amount)}` : ""}
                  {p.expires_at ? ` · s.d. ${formatAffDate(p.expires_at)}` : ""}
                </div>
              </div>
              <Button size="sm" variant="outline" onClick={() => copy(p.code)}>
                {t("aff_promos_copy")}
              </Button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
