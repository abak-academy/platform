"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { ShoppingCart } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
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
import { useProductGroupings } from "@/lib/hooks/product-groupings";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { PartnerOrderPreview } from "@/lib/types";

const NO_VOUCHER = "none";
const ALL_GROUPINGS = "all";

export default function NewPartnerOrderPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const products = usePartnerProducts();
  const groupings = useProductGroupings();
  const vouchers = usePartnerVouchers();

  const [groupingId, setGroupingId] = useState(ALL_GROUPINGS);
  const [productSearch, setProductSearch] = useState("");
  const [selectedProductIds, setSelectedProductIds] = useState<string[]>([]);
  const [voucher, setVoucher] = useState(NO_VOUCHER);
  const [q, setQ] = useState("");
  const [selected, setSelected] = useState<Record<string, string>>({});
  const [creating, setCreating] = useState(false);
  const [preview, setPreview] = useState<PartnerOrderPreview | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const students = usePartnerStudents({ q: useDebouncedValue(q, 300), productIds: selectedProductIds });
  const previewMutation = usePreviewPartnerOrder();
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
  const selectableRows = rows.filter((s) => (s.owned_product_ids?.length ?? 0) === 0);
  const key = `${selectedProductIds.join(",")}|${promo ?? ""}|${ids.join(",")}`;

  // A single preview request for every selected product at once — the order
  // model is one order, several products, one payment.
  useEffect(() => {
    setPreview(null);
    setPreviewError(null);
    if (selectedProductIds.length === 0 || ids.length === 0) return;
    let cancelled = false;
    previewMutation
      .mutateAsync({ product_ids: selectedProductIds, student_ids: ids, promo_code: promo })
      .then((data) => {
        if (!cancelled) setPreview(data);
      })
      .catch((err) => {
        if (!cancelled) setPreviewError(partnerOrderErrorMessage(err, t));
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  // A student checked while eligible can become blocked by a later product
  // pick (they already own it) — drop them so the request never carries a
  // student who isn't eligible for anything currently selected.
  useEffect(() => {
    setSelected((prev) => {
      const blocked = rows.filter((s) => (s.owned_product_ids?.length ?? 0) > 0);
      if (blocked.length === 0) return prev;
      let changed = false;
      const next = { ...prev };
      for (const s of blocked) {
        if (s.id in next) {
          delete next[s.id];
          changed = true;
        }
      }
      return changed ? next : prev;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows]);

  const toggleStudent = (id: string, name: string) =>
    setSelected((prev) => {
      const next = { ...prev };
      if (id in next) delete next[id];
      else next[id] = name;
      return next;
    });

  const toggleProduct = (id: string) =>
    setSelectedProductIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));

  // Selects or clears every currently filtered (grouping + search) product,
  // leaving a selection outside the current filter untouched.
  const toggleSelectAllProducts = () => {
    const allSelected = visibleProducts.every((p) => selectedProductIds.includes(p.id));
    setSelectedProductIds((prev) => {
      if (allSelected) {
        const visibleIds = new Set(visibleProducts.map((p) => p.id));
        return prev.filter((id) => !visibleIds.has(id));
      }
      return Array.from(new Set([...prev, ...visibleProducts.map((p) => p.id)]));
    });
  };

  // Only ever selects students who are not blocked by an already-owned product.
  const toggleSelectAllStudents = () => {
    const allSelected = selectableRows.length > 0 && selectableRows.every((s) => s.id in selected);
    setSelected((prev) => {
      const next = { ...prev };
      if (allSelected) {
        selectableRows.forEach((s) => delete next[s.id]);
      } else {
        selectableRows.forEach((s) => {
          next[s.id] = s.name;
        });
      }
      return next;
    });
  };

  const ready =
    selectedProductIds.length > 0 &&
    ids.length > 0 &&
    !previewError &&
    (preview?.items ?? []).some((item) => item.net_new_count > 0);

  async function submit() {
    setCreating(true);
    try {
      const order = await create.mutateAsync({ product_ids: selectedProductIds, student_ids: ids, promo_code: promo });
      toast.success(t("partner_new_created"));
      router.push(`/affiliate/partner/orders/${order.id}`);
    } catch (err) {
      toast.error(partnerOrderErrorMessage(err, t));
      setCreating(false);
    }
  }

  return (
    <div className="space-y-6">
      <AdminPageHeader icon={ShoppingCart} title={t("partner_new_title")} description={t("partner_new_desc")} />

      <section className="md-card-outlined space-y-3 p-5">
        <h2 className="font-semibold text-ink-900">{t("partner_new_product")}</h2>

        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <Input
            value={productSearch}
            onChange={(e) => setProductSearch(e.target.value)}
            placeholder={t("partner_new_search_ph")}
            aria-label={t("partner_new_search_ph")}
            className="w-full sm:max-w-xs"
          />
          {(groupings.data?.length ?? 0) > 0 && (
            <Select value={groupingId} onValueChange={setGroupingId}>
              <SelectTrigger aria-label={t("partner_new_grouping_label")} className="w-full sm:w-48">
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

        {products.isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : allProducts.length === 0 ? (
          <p className="text-sm text-ink-600">{t("partner_new_no_products")}</p>
        ) : visibleProducts.length === 0 ? (
          <p className="text-sm text-ink-600">{t("partner_new_products_empty")}</p>
        ) : (
          <>
            <label className="flex items-center gap-2 px-1 text-sm font-medium text-ink-700">
              <input
                type="checkbox"
                checked={visibleProducts.every((p) => selectedProductIds.includes(p.id))}
                onChange={() => toggleSelectAllProducts()}
                disabled={creating}
              />
              {t("partner_new_select_all")}
            </label>
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
          </>
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
          <>
            <label className="flex items-center gap-2 px-1 text-sm font-medium text-ink-700">
              <input
                type="checkbox"
                checked={selectableRows.length > 0 && selectableRows.every((s) => s.id in selected)}
                onChange={() => toggleSelectAllStudents()}
                disabled={creating || selectableRows.length === 0}
              />
              {t("partner_new_select_all")}
            </label>
            <ul className="max-h-72 divide-y divide-line overflow-y-auto rounded-lg border border-line">
              {rows.map((s) => {
                const blocked = (s.owned_product_ids?.length ?? 0) > 0;
                return (
                  <li key={s.id}>
                    <button
                      type="button"
                      role="checkbox"
                      aria-checked={s.id in selected}
                      disabled={creating || blocked}
                      onClick={() => toggleStudent(s.id, s.name)}
                      className="flex w-full cursor-pointer items-center gap-3 px-4 py-2 text-left text-sm disabled:cursor-default disabled:opacity-60"
                    >
                      <span aria-hidden className={`flex size-4 shrink-0 items-center justify-center rounded border text-[10px] leading-none ${s.id in selected ? "border-primary bg-primary text-white" : "border-line-strong"}`}>
                        {s.id in selected ? "✓" : ""}
                      </span>
                      <span className="font-medium text-ink-900">{s.name}</span>
                      <span className="text-ink-500">{s.school_name ?? "-"}</span>
                      {blocked && <span className="text-xs text-warn">{t("partner_students_owns_selected")}</span>}
                    </button>
                  </li>
                );
              })}
            </ul>
          </>
        )}
        {students.hasNextPage && (
          <Button variant="outline" size="sm" onClick={() => students.fetchNextPage()} disabled={students.isFetchingNextPage}>
            {t("partner_load_more")}
          </Button>
        )}
        <p className="text-sm text-ink-600">{t("partner_new_selected").replace("{n}", String(ids.length))}</p>
      </section>

      {(preview || previewError) && (
        <section className="md-card-outlined space-y-3 p-5" aria-label={t("partner_new_preview_title")}>
          <h2 className="font-semibold text-ink-900">{t("partner_new_preview_title")}</h2>
          {previewError && (
            <p role="alert" className="rounded-lg border border-danger/30 bg-danger/5 px-4 py-3 text-sm text-danger">
              {previewError}
            </p>
          )}
          {preview && !previewError && (
            <>
              <ul className="divide-y divide-line text-sm">
                {preview.items.map((item) => (
                  <li key={item.product_id} className="space-y-1 py-2">
                    <div className="flex items-center justify-between gap-3">
                      <span className="font-medium text-ink-900">{item.name}</span>
                      <span className="text-ink-600">{formatRupiah(item.unit_price)}</span>
                    </div>
                    <p className="text-ink-700">{t("partner_new_charged").replace("{n}", String(item.net_new_count))}</p>
                    {(item.excluded ?? []).length > 0 && (
                      <p className="text-warn">
                        {t("partner_new_excluded").replace("{names}", (item.excluded ?? []).map((e) => e.name).join(", "))}
                      </p>
                    )}
                  </li>
                ))}
              </ul>
              <dl className="grid grid-cols-2 gap-2 border-t border-line pt-3 text-sm">
                <dt className="text-ink-500">{t("partner_new_subtotal")}</dt>
                <dd className="text-right">{formatRupiah(preview.subtotal)}</dd>
                <dt className="text-ink-500">{t("partner_new_discount")}</dt>
                <dd className="text-right">- {formatRupiah(preview.discount)}</dd>
                <dt className="font-semibold text-ink-900">{t("partner_new_total")}</dt>
                <dd className="text-right font-semibold text-ink-900">{formatRupiah(preview.total)}</dd>
              </dl>
            </>
          )}
        </section>
      )}

      <div className="flex justify-end">
        <Button onClick={submit} disabled={!ready || creating}>
          {t("partner_new_create")}
        </Button>
      </div>
    </div>
  );
}
