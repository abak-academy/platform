"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Receipt, SlidersHorizontal, X } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliatorNotice } from "@/components/admin/AffiliatorNotice";
import { AffiliatorPurchaseHistory } from "@/components/admin/AffiliatorPurchaseHistory";
import { NoAccess } from "@/components/admin/NoAccess";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAffiliatorOrders } from "@/lib/hooks/affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { AFFILIATE_ORDER_STATUS_LABEL, affiliateOrderStatusLabel } from "@/lib/affiliate-order-status";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type { AffiliatorOrderRow } from "@/lib/types";

const ALL = "__all__";
const PAGE_SIZE = 20;

const STATUSES = Object.keys(AFFILIATE_ORDER_STATUS_LABEL);

export default function AffiliateOrdersPage() {
  const { t, lang } = useTranslation();
  const allowed = useHasCapability("affiliates:self");
  const [status, setStatus] = useState("");
  const [cursors, setCursors] = useState<string[]>([""]);
  useEffect(() => setCursors([""]), [status]);

  const { data, isLoading, isError, refetch } = useAffiliatorOrders({
    status: status || undefined,
    cursor: cursors[cursors.length - 1] || undefined,
    limit: PAGE_SIZE,
  });

  if (!allowed) return <NoAccess />;

  const statusLabel = (s: string) => affiliateOrderStatusLabel(s, t);
  const formatDate = (iso: string) =>
    new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", { day: "numeric", month: "short", year: "numeric", timeZone: "Asia/Jakarta" }).format(new Date(iso));

  const columns: DataTableColumn<AffiliatorOrderRow>[] = [
    { key: "date", header: t("aff_th_date"), cell: (r) => formatDate(r.created_at) },
    { key: "product", header: t("aff_th_product"), cell: (r) => <span className="font-medium">{r.product_name}</span> },
    { key: "school", header: t("aff_th_school"), cell: (r) => r.school_name ?? "-" },
    { key: "seats", header: t("aff_th_students"), align: "right", cell: (r) => r.participant_count },
    { key: "promo", header: t("aff_th_promo"), cell: (r) => r.promo_code ?? "-" },
    { key: "total", header: t("aff_th_order_total"), align: "right", cell: (r) => formatRupiah(r.total) },
    { key: "status", header: t("th_status"), cell: (r) => statusLabel(r.status) },
  ];

  return (
    <div className="fade-in space-y-6">
      <AdminPageHeader
        icon={Receipt}
        title={t("aff_orders_title")}
        description={t("aff_orders_subtitle")}
        actions={
          <Link href="/admin/affiliate/orders/new" className="md-btn-filled">
            {t("aff_order_new")}
          </Link>
        }
      />
      <AffiliatorNotice />
      <Tabs defaultValue="orders">
        <TabsList aria-label={t("aff_orders_title")}>
          <TabsTrigger value="orders">{t("aff_orders_tab_orders")}</TabsTrigger>
          <TabsTrigger value="students">{t("aff_orders_tab_students")}</TabsTrigger>
        </TabsList>
        <TabsContent value="orders" className="space-y-6 pt-4">
          <p className="text-sm text-ink-600">{t("aff_orders_total_note")}</p>

          <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-3">
              <Popover>
                <PopoverTrigger asChild>
                  <Button variant="outline" className="h-9" aria-label={`${t("aff_filters")}${status ? " (1)" : ""}`}>
                    <SlidersHorizontal className="size-4" aria-hidden />
                    {t("aff_filters")}
                    {status && <span className="tabular-nums">1</span>}
                  </Button>
                </PopoverTrigger>
                <PopoverContent align="start" className="admin-shell w-[min(24rem,calc(100vw-2rem))]" aria-label={t("aff_filter_orders")}>
                  <div className="space-y-3">
                    <h2 className="font-semibold">{t("aff_filter_orders")}</h2>
                    <span className="block text-sm font-medium">{t("th_status")}</span>
                    <Select value={status || ALL} onValueChange={(v) => setStatus(v === ALL ? "" : v)}>
                      <SelectTrigger className="w-full" aria-label={t("th_status")}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={ALL}>{t("tab_all")}</SelectItem>
                        {STATUSES.map((s) => (
                          <SelectItem key={s} value={s}>
                            {statusLabel(s)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
                      <Button variant="ghost" onClick={() => setStatus("")}>
                        {t("aff_reset_filters")}
                      </Button>
                      <PopoverClose asChild>
                        <Button>{t("aff_show_orders")}</Button>
                      </PopoverClose>
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            </div>
            {status && (
              <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t("aff_active_filters")}>
                <span className="md-chip md-chip-primary gap-1">
                  {`${t("th_status")}: ${statusLabel(status)}`}
                  <Button variant="ghost" size="icon" className="size-8" aria-label={`${t("aff_remove_filter")}: ${statusLabel(status)}`} onClick={() => setStatus("")}>
                    <X className="size-4" aria-hidden />
                  </Button>
                </span>
              </div>
            )}
          </div>

          {isError ? (
            <div className="md-card-outlined py-12 text-center text-ink-500">
              <p className="mb-4">{t("dash_load_failed")}</p>
              <button type="button" className="md-btn-tonal" onClick={() => refetch()}>
                {t("admin_home_reload")}
              </button>
            </div>
          ) : (
            <DataTable
              columns={columns}
              rows={isLoading ? [] : (data?.data ?? [])}
              rowKey={(r) => r.id}
              empty={isLoading ? t("aff_loading") : t("aff_orders_empty")}
              footer={
                <div className="flex justify-end gap-2 px-4 py-3">
                  <Button variant="outline" size="sm" disabled={cursors.length <= 1} onClick={() => setCursors((c) => c.slice(0, -1))}>
                    {t("aff_prev")}
                  </Button>
                  <Button variant="outline" size="sm" disabled={!data?.next_cursor} onClick={() => data?.next_cursor && setCursors((c) => [...c, data.next_cursor as string])}>
                    {t("aff_next")}
                  </Button>
                </div>
              }
            />
          )}
        </TabsContent>
        <TabsContent value="students" className="pt-4">
          <AffiliatorPurchaseHistory />
        </TabsContent>
      </Tabs>
    </div>
  );
}
