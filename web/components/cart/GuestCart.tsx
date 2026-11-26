"use client";

import Link from "next/link";
import { useQueries } from "@tanstack/react-query";
import { Minus, Plus, ShoppingCart, Trash2 } from "lucide-react";
import { apiFetch, ApiError } from "@/lib/api";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { productsKeys } from "@/lib/hooks/products";
import type { Product } from "@/lib/types";
import { isDigitalType, useGuestCart } from "@/stores/guest-cart";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";

function outsideWindow(p: Product, now: number): boolean {
  const from = p.available_from ? Date.parse(p.available_from) : null;
  const until = p.available_until ? Date.parse(p.available_until) : null;
  return (from !== null && from > now) || (until !== null && until < now);
}

export function GuestCart() {
  const { t } = useTranslation();
  const lines = useGuestCart((s) => s.lines);
  const setQty = useGuestCart((s) => s.setQty);
  const remove = useGuestCart((s) => s.remove);

  const results = useQueries({
    queries: lines.map((l) => ({
      queryKey: productsKeys.detail(l.product_id),
      queryFn: () => apiFetch<Product>(`/products/${encodeURIComponent(l.product_id)}`),
      retry: false,
    })),
  });

  const now = Date.now();
  const rows = lines.map((line, i) => {
    const { data, error } = results[i];
    const unavailable =
      (error instanceof ApiError && error.status === 404) || (!!data && outsideWindow(data, now));
    const price = data && !unavailable ? data.price : line.unit_price;
    return { line, unavailable, price, priceChanged: !unavailable && !!data && data.price !== line.unit_price, stock: data?.stock };
  });

  const subtotal = rows.reduce((sum, r) => (r.unavailable ? sum : sum + r.price * r.line.qty), 0);
  const hasPhysical = rows.some((r) => !isDigitalType(r.line.type));

  return (
    <div className="mx-auto max-w-public-narrow px-6 pb-20 pt-32">
      <h1 className="mb-6 font-serif text-2xl font-bold text-ink-900 md:text-3xl">{t("cart_title")}</h1>

      {rows.length === 0 ? (
        <EmptyState
          icon={ShoppingCart}
          title={t("gcart_empty_title")}
          text={t("gcart_empty_text")}
          action={
            <Button asChild>
              <Link href="/catalog">{t("cart_view_catalog")}</Link>
            </Button>
          }
        />
      ) : (
        <>
          <ul className="overflow-hidden rounded-lg border border-line bg-surface">
            {rows.map(({ line, unavailable, price, priceChanged, stock }) => {
              const digital = isDigitalType(line.type);
              return (
                <li key={line.product_id} className="flex flex-col gap-2 border-b border-line px-4 py-4 last:border-b-0">
                  <div className="flex items-start justify-between gap-2">
                    <span className="text-[15px] font-semibold text-ink-900">{line.name}</span>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8 text-ink-400 hover:text-danger"
                      onClick={() => remove(line.product_id)}
                      aria-label={`${t("gcart_remove")} ${line.name}`}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                  {unavailable ? (
                    <p className="text-sm font-medium text-danger">{t("gcart_unavailable")}</p>
                  ) : (
                    <div className="flex items-center justify-between gap-2 text-sm">
                      {digital ? (
                        <span className="text-xs text-ink-500">{t("product_digital_single_qty")}</span>
                      ) : (
                        <div className="flex items-center gap-2">
                          <button
                            type="button"
                            onClick={() => setQty(line.product_id, line.qty - 1, stock)}
                            disabled={line.qty <= 1}
                            aria-label={`${t("product_qty_label")} −`}
                            className="flex size-7 items-center justify-center rounded-full border border-line text-ink-600 hover:bg-paper disabled:opacity-40"
                          >
                            <Minus className="size-3" />
                          </button>
                          <span className="w-6 text-center font-semibold text-ink-900">{line.qty}</span>
                          <button
                            type="button"
                            onClick={() => setQty(line.product_id, line.qty + 1, stock)}
                            disabled={typeof stock === "number" ? line.qty >= stock : true}
                            aria-label={`${t("product_qty_label")} +`}
                            className="flex size-7 items-center justify-center rounded-full border border-line text-ink-600 hover:bg-paper disabled:opacity-40"
                          >
                            <Plus className="size-3" />
                          </button>
                        </div>
                      )}
                      <div className="text-right">
                        {priceChanged && <p className="text-xs font-medium text-warn">{t("gcart_price_changed")}</p>}
                        <span className="font-serif text-base font-bold text-ink-900">{formatRupiah(price * line.qty)}</span>
                      </div>
                    </div>
                  )}
                </li>
              );
            })}
          </ul>

          <div className="mt-6 rounded-lg border border-line bg-surface p-5">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-ink-900">{t("gcart_subtotal")}</span>
              <span data-testid="guest-subtotal" className="font-serif text-2xl font-bold text-ink-900">
                {formatRupiah(subtotal)}
              </span>
            </div>
            <div className="mt-3 flex flex-col gap-1 text-xs text-ink-500">
              {hasPhysical && <p>{t("gcart_note_address")}</p>}
              <p>{t("gcart_note_promo")}</p>
            </div>
            <Button asChild size="lg" className="mt-4 w-full">
              <Link href="/login?next=/cart">{t("gcart_pay")}</Link>
            </Button>
            <p className="mt-2 text-center text-xs text-ink-500">{t("gcart_pay_hint")}</p>
          </div>
        </>
      )}
    </div>
  );
}
