"use client";

import { Tag } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliatorNotice } from "@/components/admin/AffiliatorNotice";
import { NoAccess } from "@/components/admin/NoAccess";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { useAffiliatorProducts, useAffiliatorPromos } from "@/lib/hooks/affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type { PromoCode } from "@/lib/types";

export default function AffiliatePromosPage() {
  const { t, lang } = useTranslation();
  const allowed = useHasCapability("affiliates:self");
  const promos = useAffiliatorPromos();
  const products = useAffiliatorProducts();

  if (!allowed) return <NoAccess />;

  const nameOf = new Map((products.data ?? []).map((p) => [p.id, p.name]));
  const formatDate = (iso?: string | null) =>
    iso ? new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", { day: "numeric", month: "short", year: "numeric", timeZone: "Asia/Jakarta" }).format(new Date(iso)) : null;

  const discount = (p: PromoCode) => {
    if (p.discount_percent != null) {
      const cap = p.max_discount_amount != null ? ` (${t("aff_promo_max")} ${formatRupiah(p.max_discount_amount)})` : "";
      return `${p.discount_percent}%${cap}`;
    }
    return p.discount_amount != null ? formatRupiah(p.discount_amount) : "-";
  };
  const conditions = (p: PromoCode) =>
    [
      p.min_qty ? t("aff_promo_min_qty").replace("{n}", String(p.min_qty)) : null,
      p.min_order_amount ? t("aff_promo_min_amount").replace("{n}", formatRupiah(p.min_order_amount)) : null,
    ]
      .filter(Boolean)
      .join(" · ") || "-";
  const validity = (p: PromoCode) => {
    const from = formatDate(p.starts_at);
    const to = formatDate(p.expires_at);
    if (!from && !to) return t("aff_promo_no_expiry");
    return `${from ?? "…"} – ${to ?? "…"}`;
  };
  const scope = (p: PromoCode) =>
    (p.product_ids?.length ?? 0) === 0
      ? t("aff_promo_all_products")
      : (p.product_ids ?? []).map((id) => nameOf.get(id) ?? id).join(", ");

  const columns: DataTableColumn<PromoCode>[] = [
    { key: "code", header: t("th_code"), cell: (p) => <span className="font-medium">{p.code}</span> },
    { key: "discount", header: t("th_discount"), cell: discount },
    { key: "conditions", header: t("aff_th_conditions"), cell: conditions },
    { key: "validity", header: t("aff_th_validity"), cell: validity },
    { key: "products", header: t("aff_th_product"), cell: scope },
  ];

  return (
    <div className="fade-in space-y-6">
      <AdminPageHeader icon={Tag} title={t("aff_promos_title")} description={t("aff_promos_subtitle")} />
      <AffiliatorNotice />
      <DataTable columns={columns} rows={promos.data ?? []} rowKey={(p) => p.id} empty={promos.isLoading ? t("aff_loading") : t("aff_promos_empty")} />
    </div>
  );
}
