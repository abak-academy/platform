"use client";

import { useState } from "react";
import Link from "next/link";
import { ShoppingCart, X } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliatorNotice, useAffiliatorReadOnly } from "@/components/admin/AffiliatorNotice";
import { NoAccess } from "@/components/admin/NoAccess";
import { SnapCheckout } from "@/components/cart/SnapCheckout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAdminStudents } from "@/lib/hooks/admin-students";
import {
  useAffiliatorMe,
  useAffiliatorOrders,
  useAffiliatorProducts,
  useCreateAffiliatorOrder,
  usePreviewAffiliatorOrder,
} from "@/lib/hooks/affiliates";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { affiliateOrderStatusLabel } from "@/lib/affiliate-order-status";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type { AffiliatorOrderInput, AffiliatorOrderPreview, Order } from "@/lib/types";

const ALL = "__all_schools__";

interface Picked {
  id: string;
  name: string;
  school: string | null;
}

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}

export default function AffiliateNewOrderPage() {
  const { t } = useTranslation();
  const allowed = useHasCapability("affiliates:self");
  const readOnly = useAffiliatorReadOnly();
  const me = useAffiliatorMe();
  const products = useAffiliatorProducts();
  const preview = usePreviewAffiliatorOrder();
  const create = useCreateAffiliatorOrder();

  const [schoolId, setSchoolId] = useState("");
  const [productId, setProductId] = useState("");
  const [selected, setSelected] = useState<Picked[]>([]);
  const [search, setSearch] = useState("");
  const [promoCode, setPromoCode] = useState("");
  const [result, setResult] = useState<AffiliatorOrderPreview | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [order, setOrder] = useState<Order | null>(null);
  const q = useDebouncedValue(search).trim();
  // Once an order exists its status is the only proof that payment went through,
  // for the free path (no popup) as much as for Snap, so follow it until it settles.
  const created = useAffiliatorOrders({ limit: 10, refetchInterval: order ? 3000 : undefined });

  const students = useAdminStudents({
    schoolId,
    q: q || undefined,
    status: "active",
    limit: 100,
  });

  if (!allowed) return <NoAccess />;

  const schools = me.data?.schools ?? [];
  const rows = students.data?.data ?? [];
  const selectedIds = selected.map((p) => p.id);
  const input: AffiliatorOrderInput = {
    product_id: productId,
    student_ids: selectedIds,
    ...(promoCode.trim() ? { promo_code: promoCode.trim() } : {}),
  };
  const ready = productId !== "" && selected.length > 0;
  const toPicked = (r: (typeof rows)[number]): Picked => ({ id: r.id, name: r.name, school: r.school_name ?? null });

  function toggle(p: Picked, on: boolean) {
    setSelected((cur) => (on ? [...cur.filter((x) => x.id !== p.id), p] : cur.filter((x) => x.id !== p.id)));
    touch();
  }

  // Any change to the inputs voids the shown preview: the order is re-resolved on the server anyway.
  function touch() {
    setResult(null);
    setPreviewError(null);
  }

  async function handlePreview() {
    touch();
    try {
      setResult(await preview.mutateAsync(input));
    } catch (err) {
      setPreviewError(errorMessage(err, t("error_generic")));
    }
  }

  async function handleCreate() {
    try {
      setOrder(await create.mutateAsync(input));
    } catch (err) {
      toast.error(errorMessage(err, t("error_generic")));
    }
  }

  if (order) {
    const live = created.data?.data.find((o) => o.id === order.id);
    return (
      <div className="fade-in space-y-6">
        <AdminPageHeader icon={ShoppingCart} title={t("aff_order_created_title")} description={t("aff_order_created_hint")} />
        <div className="md-card-outlined space-y-3 p-6">
          <p className="text-sm">
            {t("aff_stat_paid_amount")}: <strong>{formatRupiah(order.total)}</strong>
          </p>
          <p className="text-sm" data-testid="aff-order-status">
            {t("aff_order_status")}: <strong>{live ? affiliateOrderStatusLabel(live.status, t) : t("aff_order_not_paid")}</strong>
          </p>
          <SnapCheckout orderId={order.id} basePath="/affiliates/me/orders" />
          <Link href="/admin/affiliate/orders" className="md-btn-tonal inline-block">
            {t("aff_view_orders")}
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="fade-in space-y-6">
      <AdminPageHeader icon={ShoppingCart} title={t("aff_order_new")} description={t("aff_order_new_hint")} />
      <AffiliatorNotice />

      <div className="md-card-outlined grid gap-5 p-6">
        <div className="grid gap-2">
          <Label htmlFor="aff-order-product">{t("aff_th_product")}</Label>
          <Select value={productId} onValueChange={(v) => { setProductId(v); touch(); }}>
            <SelectTrigger id="aff-order-product" className="w-full" aria-label={t("aff_th_product")}>
              <SelectValue placeholder={t("aff_choose_product")} />
            </SelectTrigger>
            <SelectContent>
              {(products.data ?? []).map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {`${p.name} · ${t(p.type === "exam" ? "product_type_exam" : "product_type_course")} · ${formatRupiah(p.price)}`}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <fieldset className="grid gap-2">
          <legend className="text-sm font-medium">{t("aff_th_students")}</legend>
          <div className="flex flex-wrap gap-2">
            <Input className="min-w-48 flex-1" aria-label={t("aff_students_search")} value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t("aff_students_search")} />
            <div className="w-full sm:w-56">
              <Select value={schoolId || ALL} onValueChange={(v) => setSchoolId(v === ALL ? "" : v)}>
                <SelectTrigger className="w-full" aria-label={t("aff_th_school")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ALL}>{t("aff_filter_all_schools")}</SelectItem>
                  {schools.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span>{t("aff_selected_count").replace("{n}", String(selected.length))}</span>
            <div className="flex gap-1">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={rows.length === 0}
                onClick={() => { setSelected((cur) => [...cur, ...rows.filter((r) => !cur.some((x) => x.id === r.id)).map(toPicked)]); touch(); }}
              >
                {t("aff_select_shown")}
              </Button>
              <Button type="button" variant="ghost" size="sm" disabled={selected.length === 0} onClick={() => { setSelected([]); touch(); }}>
                {t("aff_clear_selection")}
              </Button>
            </div>
          </div>
          <div className="max-h-56 space-y-1 overflow-y-auto rounded-md border p-2" data-testid="aff-student-list">
            {rows.map((r) => (
              <label key={r.id} className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={selectedIds.includes(r.id)} onChange={(e) => toggle(toPicked(r), e.target.checked)} />
                <span>{r.name}</span>
                <span className="text-xs text-ink-600">{r.school_name ?? t("aff_no_school")}</span>
              </label>
            ))}
            {rows.length === 0 && <p className="text-sm text-ink-500">{students.isLoading ? t("aff_loading") : t("aff_students_empty")}</p>}
          </div>
          {selected.length > 0 && (
            <div className="flex flex-wrap gap-2" role="group" aria-label={t("aff_selected_students")}>
              {selected.map((p) => (
                <span key={p.id} className="md-chip md-chip-primary gap-1">
                  {p.name}
                  {p.school ? ` · ${p.school}` : ""}
                  <Button type="button" variant="ghost" size="icon" className="size-8" aria-label={`${t("aff_remove_filter")}: ${p.name}`} onClick={() => toggle(p, false)}>
                    <X className="size-4" aria-hidden />
                  </Button>
                </span>
              ))}
            </div>
          )}
        </fieldset>

        <div className="grid gap-2">
          <Label htmlFor="aff-order-promo">{t("aff_promo_code")}</Label>
          <Input id="aff-order-promo" value={promoCode} onChange={(e) => { setPromoCode(e.target.value); touch(); }} placeholder={t("aff_promo_optional")} />
        </div>

        <div>
          <Button type="button" variant="outline" onClick={handlePreview} disabled={!ready || preview.isPending}>
            {preview.isPending ? t("aff_loading") : t("aff_preview")}
          </Button>
        </div>

        {previewError && (
          <p role="alert" className="text-sm text-destructive">
            {previewError}
          </p>
        )}

        {result && (
          <div className="grid gap-2 rounded-[12px] border border-line p-4 text-sm" data-testid="aff-preview">
            <div className="flex justify-between"><span>{t("aff_preview_net_new")}</span><strong>{result.net_new_count}</strong></div>
            {(result.students?.length ?? 0) > 0 && (
              <ul className="list-disc pl-5" data-testid="aff-preview-students">
                {result.students.map((st) => (
                  <li key={st.student_id}>
                    {st.name} <span className="text-ink-600">· {st.school_name ?? t("aff_no_school")}</span>
                  </li>
                ))}
              </ul>
            )}
            {(result.excluded?.length ?? 0) > 0 && (
              <div>
                <p className="text-ink-600">{t("aff_preview_excluded").replace("{n}", String(result.excluded?.length ?? 0))}</p>
                <ul className="list-disc pl-5">
                  {result.excluded?.map((e) => (
                    <li key={e.student_id}>{e.name}</li>
                  ))}
                </ul>
              </div>
            )}
            <div className="flex justify-between"><span>{t("aff_preview_subtotal")}</span><span>{formatRupiah(result.subtotal)}</span></div>
            <div className="flex justify-between"><span>{t("aff_stat_discount")}</span><span>-{formatRupiah(result.discount)}</span></div>
            <div className="flex justify-between border-t pt-2"><span>{t("aff_preview_total")}</span><strong>{formatRupiah(result.total)}</strong></div>
            <Button type="button" onClick={handleCreate} disabled={readOnly || result.net_new_count === 0 || create.isPending}>
              {create.isPending ? t("aff_saving") : t("aff_order_create")}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
