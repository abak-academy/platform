"use client";

import { useMergeNoticeStore, type MergeOutcome } from "@/lib/guest-cart-merge";
import { digitalConflictMessage } from "@/lib/digital-conflict";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";

export function MergeNotice() {
  const { t } = useTranslation();
  const outcomes = useMergeNoticeStore((s) => s.outcomes);
  const dismiss = useMergeNoticeStore((s) => s.dismiss);
  if (outcomes.length === 0) return null;

  const message = (o: MergeOutcome): string | null => {
    if (o.price_changed) {
      return t("gcart_price_changed_detail")
        .replace("{from}", formatRupiah(o.price_changed.from))
        .replace("{to}", formatRupiah(o.price_changed.to));
    }
    switch (o.code) {
      case "already_in_cart":
        return t("gcart_already_in_cart");
      case "unavailable":
        return t("gcart_unavailable");
      case "out_of_stock":
        return t("gcart_out_of_stock");
      case "digital_conflict":
        return (o.error && digitalConflictMessage(o.error, t)) || t("digital_conflict_generic");
      default:
        return null;
    }
  };

  const rows = outcomes.map((o) => ({ o, text: message(o) })).filter((r) => r.text);
  if (rows.length === 0) return null;

  return (
    <div role="status" className="mb-6 rounded-lg border border-warn/30 bg-warn-bg px-5 py-4 text-sm text-ink-900">
      <div className="flex items-start justify-between gap-3">
        <p className="font-semibold">{t("gcart_merge_title")}</p>
        <Button type="button" variant="ghost" size="sm" onClick={dismiss}>
          {t("gcart_merge_dismiss")}
        </Button>
      </div>
      <ul className="mt-2 flex flex-col gap-1">
        {rows.map(({ o, text }) => (
          <li key={o.product_id}>
            <span className="font-medium">{o.name}</span> — {text}
          </li>
        ))}
      </ul>
    </div>
  );
}
