"use client";

import { useEffect, useMemo, useState } from "react";
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
  partnerKeys,
  useCreatePartnerOrder,
  usePartnerProducts,
  usePartnerStudents,
  usePartnerVouchers,
  usePreviewPartnerOrder,
} from "@/lib/hooks/partner";
import { useProductGroupings } from "@/lib/hooks/product-groupings";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { PartnerOrder, PartnerOrderPreview } from "@/lib/types";

const NO_VOUCHER = "none";
const ALL_GROUPINGS = "all";

export default function NewPartnerOrderPage() {
  const { t } = useTranslation();
  const products = usePartnerProducts();
  const groupings = useProductGroupings();
  const vouchers = usePartnerVouchers();

  const [groupingId, setGroupingId] = useState(ALL_GROUPINGS);
  const [productSearch, setProductSearch] = useState("");
  const [selectedProductIds, setSelectedProductIds] = useState<string[]>([]);
  const [voucher, setVoucher] = useState(NO_VOUCHER);
  const [q, setQ] = useState("");
  const [selected, setSelected] = useState<Record<string, string>>({});
  const [createdOrders, setCreatedOrders] = useState<PartnerOrder[]>([]);
  const [creating, setCreating] = useState(false);
  const [previews, setPreviews] = useState<Record<string, PartnerOrderPreview>>({});
  const [previewErrors, setPreviewErrors] = useState<Record<string, string>>({});
  const students = usePartnerStudents({ q: useDebouncedValue(q, 300) });
  const preview = usePreviewPartnerOrder();
  const create = useCreatePartnerOrder();

  const allProducts = products.data?.data ?? [];
  const visibleProducts = useMemo(() => {
    const needle = productSearch.trim().toLowerCase();
    return allProducts.filter((p) => {
      if (groupingId !== ALL_GROUPINGS && p.grouping_id !== groupingId) return false;
      if (needle !== "" && !p.name.toLowerCase().includes(needle)) return false;
      return true;
    });
  }, [allProducts, groupingId, productSearch]);

  const ids = Object.keys(selected);
  const promo = voucher === NO_VOUCHER ? undefined : voucher;
  const rows = students.data?.pages.flatMap((p) => p.data) ?? [];
  const noSchools = rows.length === 0 && students.data?.pages[0]?.hint === "no_schools";
  const key = `${selectedProductIds.join(",")}|${promo ?? ""}|${ids.join(",")}`;

  // Every selected product gets its own preview request: the server decides
  // who is charged and what the voucher does for that product, per product.
  useEffect(() => {
    setPreviews({});
    setPreviewErrors({});
    if (selectedProductIds.length === 0 || ids.length === 0) return;
    for (const pid of selectedProductIds) {
      preview.mutate(
        { product_id: pid, student_ids: ids, promo_code: promo },
        {
          onSuccess: (data) => setPreviews((prev) => ({ ...prev, [pid]: data })),
          onError: (err) => setPreviewErrors((prev) => ({ ...prev, [pid]: partnerOrderErrorMessage(err, t) })),
        },
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  const toggleStudent = (id: string, name: string) =>
    setSelected((prev) => {
      const next = { ...prev };
      if (id in next) delete next[id];
      else next[id] = name;
      return next;
    });

  const toggleProduct = (id: string) =>
    setSelectedProductIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));

  const ready =
    selectedProductIds.length > 0 &&
    ids.length > 0 &&
    selectedProductIds.every((pid) => (previews[pid]?.net_new_count ?? 0) > 0 && !previewErrors[pid]);

  // Creates one order per selected product (same students, same voucher),
  // one after another — the Mitra order model stays one product per order.
  async function submit() {
    setCreating(true);
    const created: PartnerOrder[] = [];
    for (const pid of selectedProductIds) {
      try {
        const order = await create.mutateAsync({ product_id: pid, student_ids: ids, promo_code: promo });
        created.push(order);
      } catch (err) {
        toast.error(partnerOrderErrorMessage(err, t));
        break;
      }
    }
    if (created.length > 0) {
      setCreatedOrders((prev) => [...prev, ...created]);
      toast.success(t("partner_new_created"));
    }
    setSelectedProductIds([]);
    setCreating(false);
  }

  return (
    <div className="space-y-6">
      <AdminPageHeader icon={ShoppingCart} title={t("partner_new_title")} description={t("partner_new_desc")} />

      <section className="md-card-outlined space-y-3 p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <Label>{t("partner_new_product")}</Label>
          <div className="flex flex-wrap items-center gap-2">
            <Input
              value={productSearch}
              onChange={(e) => setProductSearch(e.target.value)}
              placeholder={t("partner_new_search_ph")}
              aria-label={t("partner_new_search_ph")}
              className="max-w-xs"
            />
            {(groupings.data?.length ?? 0) > 0 && (
              <Select value={groupingId} onValueChange={setGroupingId}>
                <SelectTrigger aria-label={t("partner_new_grouping_label")} className="w-48">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ALL_GROUPINGS}>{t("partner_new_grouping_all")}</SelectItem>
                  {groupings.data!.map((g) => (
                    <SelectItem key={g.id} value={g.id}>
                      {g.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        </div>

        {products.isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : allProducts.length === 0 ? (
          <p className="text-sm text-ink-600">{t("partner_new_no_products")}</p>
        ) : visibleProducts.length === 0 ? (
          <p className="text-sm text-ink-600">{t("partner_new_products_empty")}</p>
        ) : (
          <ul className="max-h-64 divide-y divide-line overflow-y-auto rounded-lg border border-line">
            {visibleProducts.map((prod) => (
              <li key={prod.id}>
                <label className="flex w-full cursor-pointer items-center gap-3 px-4 py-2 text-left text-sm">
                  <input
                    type="checkbox"
                    checked={selectedProductIds.includes(prod.id)}
                    onChange={() => toggleProduct(prod.id)}
                    disabled={creating}
                  />
                  <span className="font-medium text-ink-900">{prod.name}</span>
                  <span className="text-ink-500">({formatRupiah(prod.price)})</span>
                </label>
              </li>
            ))}
          </ul>
        )}
        <p className="text-sm text-ink-600">
          {t("partner_new_selected_products").replace("{n}", String(selectedProductIds.length))}
        </p>

        <div className="space-y-1.5">
          <Label htmlFor="partner-order-voucher">{t("partner_new_voucher")}</Label>
          <Select value={voucher} onValueChange={setVoucher} disabled={creating}>
            <SelectTrigger id="partner-order-voucher" className="w-full sm:w-64">
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
                <button
                  type="button"
                  role="checkbox"
                  aria-checked={s.id in selected}
                  disabled={creating}
                  onClick={() => toggleStudent(s.id, s.name)}
                  className="flex w-full cursor-pointer items-center gap-3 px-4 py-2 text-left text-sm disabled:cursor-default disabled:opacity-60"
                >
                  <span aria-hidden className={`flex size-4 shrink-0 items-center justify-center rounded border text-[10px] leading-none ${s.id in selected ? "border-primary bg-primary text-white" : "border-line-strong"}`}>
                    {s.id in selected ? "✓" : ""}
                  </span>
                  <span className="font-medium text-ink-900">{s.name}</span>
                  <span className="text-ink-500">{s.school_name ?? "-"}</span>
                </button>
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

      {selectedProductIds.map((pid) => {
        const prod = allProducts.find((p) => p.id === pid);
        const p = previews[pid];
        const err = previewErrors[pid];
        return (
          <section key={pid} className="md-card-outlined space-y-3 p-5" aria-label={prod?.name ?? pid}>
            <h2 className="font-semibold text-ink-900">{prod?.name ?? pid}</h2>
            {err && (
              <p role="alert" className="rounded-lg border border-danger/30 bg-danger/5 px-4 py-3 text-sm text-danger">
                {err}
              </p>
            )}
            {p && !err && (
              <>
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
              </>
            )}
          </section>
        );
      })}

      <div className="flex justify-end">
        <Button onClick={submit} disabled={!ready || creating}>
          {t("partner_new_create")}
        </Button>
      </div>

      {createdOrders.length > 0 && (
        <section className="md-card-outlined space-y-3 p-5">
          <h2 className="font-semibold text-ink-900">{t("partner_new_orders_created_heading")}</h2>
          <p className="text-sm text-ink-700">{t("partner_new_pay_hint")}</p>
          {createdOrders.map((order) => (
            <div key={order.id} className="rounded-lg border border-line p-3">
              <SnapCheckout orderId={order.id} basePath="/partners/me/orders" invalidateKey={partnerKeys.orders()} />
            </div>
          ))}
        </section>
      )}
    </div>
  );
}
