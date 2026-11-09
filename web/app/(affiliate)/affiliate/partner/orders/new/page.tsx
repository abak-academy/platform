"use client";

import { useEffect, useState } from "react";
import { ShoppingCart } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { SnapCheckout } from "@/components/cart/SnapCheckout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { partnerOrderErrorMessage } from "@/lib/partner-order-errors";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import {
  useCreatePartnerOrder,
  usePartnerProducts,
  usePartnerStudents,
  usePartnerVouchers,
  usePreviewPartnerOrder,
} from "@/lib/hooks/partner";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { PartnerOrder } from "@/lib/types";

const NO_VOUCHER = "none";

export default function NewPartnerOrderPage() {
  const { t } = useTranslation();
  const products = usePartnerProducts();
  const vouchers = usePartnerVouchers();
  const [productId, setProductId] = useState("");
  const [voucher, setVoucher] = useState(NO_VOUCHER);
  const [q, setQ] = useState("");
  const [selected, setSelected] = useState<Record<string, string>>({});
  const [created, setCreated] = useState<PartnerOrder | null>(null);
  const [error, setError] = useState<string | null>(null);
  const students = usePartnerStudents({ q: useDebouncedValue(q, 300) });
  const preview = usePreviewPartnerOrder();
  const create = useCreatePartnerOrder();

  const ids = Object.keys(selected);
  const promo = voucher === NO_VOUCHER ? undefined : voucher;
  const rows = students.data?.pages.flatMap((p) => p.data) ?? [];
  const noSchools = rows.length === 0 && students.data?.pages[0]?.hint === "no_schools";
  const key = `${productId}|${promo ?? ""}|${ids.join(",")}`;

  // The preview is recomputed whenever the choice changes; the server decides
  // who is charged and what the voucher does, and the page only shows it.
  useEffect(() => {
    setError(null);
    if (!productId || ids.length === 0) {
      preview.reset();
      return;
    }
    preview.mutate(
      { product_id: productId, student_ids: ids, promo_code: promo },
      { onError: (err) => setError(partnerOrderErrorMessage(err, t)) },
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  const toggle = (id: string, name: string) =>
    setSelected((prev) => {
      const next = { ...prev };
      if (id in next) delete next[id];
      else next[id] = name;
      return next;
    });

  const submit = () =>
    create.mutate(
      { product_id: productId, student_ids: ids, promo_code: promo },
      {
        onSuccess: (order) => {
          setCreated(order);
          toast.success(t("partner_new_created"));
        },
        onError: (err) => setError(partnerOrderErrorMessage(err, t)),
      },
    );

  const p = preview.data;
  const ready = Boolean(p && p.net_new_count > 0 && !error);

  return (
    <div className="space-y-6">
      <AdminPageHeader icon={ShoppingCart} title={t("partner_new_title")} description={t("partner_new_desc")} />

      <section className="md-card-outlined grid gap-4 p-5 md:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="partner-order-product">{t("partner_new_product")}</Label>
          {products.isLoading ? (
            <Skeleton className="h-10 w-full" />
          ) : (products.data?.data ?? []).length === 0 ? (
            <p className="text-sm text-ink-600">{t("partner_new_no_products")}</p>
          ) : (
            <Select value={productId} onValueChange={setProductId} disabled={Boolean(created)}>
              <SelectTrigger id="partner-order-product">
                <SelectValue placeholder={t("partner_new_product_ph")} />
              </SelectTrigger>
              <SelectContent>
                {(products.data?.data ?? []).map((prod) => (
                  <SelectItem key={prod.id} value={prod.id}>
                    {prod.name} ({formatRupiah(prod.price)})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="partner-order-voucher">{t("partner_new_voucher")}</Label>
          <Select value={voucher} onValueChange={setVoucher} disabled={Boolean(created)}>
            <SelectTrigger id="partner-order-voucher">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NO_VOUCHER}>{t("partner_new_voucher_none")}</SelectItem>
              {(vouchers.data?.data ?? []).map((v) => (
                <SelectItem key={v.id} value={v.code}>
                  {v.code}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </section>

      <section className="md-card-outlined space-y-3 p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="font-semibold text-ink-900">{t("partner_new_students")}</h2>
            <p className="text-xs text-ink-500">{t("partner_new_students_hint")}</p>
          </div>
          <Input className="max-w-xs" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t("partner_students_search")} aria-label={t("partner_students_search")} />
        </div>
        {students.isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : noSchools ? (
          <p className="text-sm text-ink-600">{t("partner_students_no_schools")}</p>
        ) : rows.length === 0 ? (
          <p className="text-sm text-ink-600">{t("partner_students_empty")}</p>
        ) : (
          <ul className="max-h-72 divide-y divide-line overflow-y-auto rounded-lg border border-line">
            {rows.map((s) => (
              <li key={s.id}>
                <label className="flex cursor-pointer items-center gap-3 px-4 py-2 text-sm">
                  <input type="checkbox" checked={s.id in selected} disabled={Boolean(created)} onChange={() => toggle(s.id, s.name)} />
                  <span className="font-medium text-ink-900">{s.name}</span>
                  <span className="text-ink-500">{s.school_name ?? "-"}</span>
                </label>
              </li>
            ))}
          </ul>
        )}
        {students.hasNextPage && (
          <Button variant="outline" size="sm" onClick={() => students.fetchNextPage()} disabled={students.isFetchingNextPage}>
            {t("partner_load_more")}
          </Button>
        )}
        <p className="text-sm text-ink-600">{t("partner_new_selected").replace("{n}", String(ids.length))}</p>
      </section>

      {error && (
        <p role="alert" className="rounded-lg border border-danger/30 bg-danger/5 px-4 py-3 text-sm text-danger">
          {error}
        </p>
      )}

      {p && !error && (
        <section className="md-card-outlined space-y-3 p-5" aria-label={t("partner_new_preview_title")}>
          <h2 className="font-semibold text-ink-900">{t("partner_new_preview_title")}</h2>
          <p className="text-sm text-ink-700">{t("partner_new_charged").replace("{n}", String(p.net_new_count))}</p>
          {(p.excluded ?? []).length > 0 && (
            <p className="text-sm text-warn">
              {t("partner_new_excluded").replace("{names}", (p.excluded ?? []).map((e) => e.name).join(", "))}
            </p>
          )}
          <dl className="grid grid-cols-2 gap-2 text-sm">
            <dt className="text-ink-500">{t("partner_new_unit_price")}</dt>
            <dd className="text-right">{formatRupiah(p.unit_price)}</dd>
            <dt className="text-ink-500">{t("partner_new_subtotal")}</dt>
            <dd className="text-right">{formatRupiah(p.subtotal)}</dd>
            <dt className="text-ink-500">{t("partner_new_discount")}</dt>
            <dd className="text-right">- {formatRupiah(p.discount)}</dd>
            <dt className="font-semibold text-ink-900">{t("partner_new_total")}</dt>
            <dd className="text-right font-semibold text-ink-900">{formatRupiah(p.total)}</dd>
          </dl>
        </section>
      )}

      {created ? (
        <section className="md-card-outlined space-y-2 p-5">
          <p className="text-sm text-ink-700">{t("partner_new_pay_hint")}</p>
          <SnapCheckout orderId={created.id} basePath="/partners/me/orders" />
        </section>
      ) : (
        <div className="flex justify-end">
          <Button onClick={submit} disabled={!ready || create.isPending}>
            {t("partner_new_create")}
          </Button>
        </div>
      )}
    </div>
  );
}
