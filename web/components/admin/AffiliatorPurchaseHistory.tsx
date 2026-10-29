"use client";

import { useState } from "react";
import { Search, SlidersHorizontal, X } from "lucide-react";
import { TablePager } from "@/components/admin/TablePager";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAffiliatorMe, useAffiliatorPurchases } from "@/lib/hooks/affiliates";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { useOffsetPaging } from "@/lib/hooks/use-offset-paging";
import { AFFILIATE_ORDER_STATUS_LABEL, affiliateOrderStatusLabel } from "@/lib/affiliate-order-status";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliatorPurchaseRow } from "@/lib/types";

const ALL = "__all__";
const STATUSES = Object.keys(AFFILIATE_ORDER_STATUS_LABEL);

// Which students each order covers, one row per order and student.
export function AffiliatorPurchaseHistory() {
  const { t, lang } = useTranslation();
  const me = useAffiliatorMe();
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [schoolId, setSchoolId] = useState("");
  const [limit, setLimit] = useState(20);
  const q = useDebouncedValue(search).trim();
  const paging = useOffsetPaging([q, status, schoolId, limit].join("|"), limit);
  const list = useAffiliatorPurchases({ q: q || undefined, status: status || undefined, schoolId: schoolId || undefined, limit, offset: paging.offset });
  const rows = list.data?.data ?? [];
  const total = list.data?.total ?? 0;
  const schools = me.data?.schools ?? [];
  const statusLabel = (s: string) => affiliateOrderStatusLabel(s, t);
  const formatDate = (iso: string) =>
    new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", { day: "numeric", month: "short", year: "numeric", timeZone: "Asia/Jakarta" }).format(new Date(iso));
  const active = [status, schoolId].filter(Boolean).length;

  const chips = [
    ...(status ? [{ key: "status", label: `${t("th_status")}: ${statusLabel(status)}`, clear: () => setStatus("") }] : []),
    ...(schoolId ? [{ key: "school", label: `${t("aff_th_school")}: ${schools.find((s) => s.id === schoolId)?.name ?? schoolId}`, clear: () => setSchoolId("") }] : []),
  ];

  const columns: DataTableColumn<AffiliatorPurchaseRow>[] = [
    { key: "date", header: t("aff_th_date"), cell: (r) => formatDate(r.created_at) },
    {
      key: "student",
      header: t("aff_th_student"),
      cell: (r) => (
        <div>
          <div className="font-medium">{r.student_name}</div>
          <div className="text-xs text-ink-600">{r.student_username}</div>
        </div>
      ),
    },
    { key: "school", header: t("aff_th_school"), cell: (r) => r.school_name ?? t("aff_no_school") },
    { key: "product", header: t("aff_th_product"), cell: (r) => r.product_name },
    { key: "price", header: t("aff_th_unit_price"), align: "right", cell: (r) => formatRupiah(r.unit_price) },
    { key: "promo", header: t("aff_th_promo"), cell: (r) => r.promo_code ?? "-" },
    { key: "status", header: t("th_status"), cell: (r) => statusLabel(r.order_status) },
  ];

  return (
    <div className="space-y-4">
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative w-full min-w-0 sm:w-auto sm:min-w-56 sm:max-w-md sm:flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
            <Input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t("aff_purchases_search")} aria-label={t("aff_purchases_search")} className="pr-10 pl-9" />
            {search && (
              <Button type="button" variant="ghost" size="icon" className="absolute top-0 right-0 size-9" aria-label={t("aff_clear_search")} onClick={() => setSearch("")}>
                <X className="size-4" aria-hidden />
              </Button>
            )}
          </div>
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="h-9" aria-label={`${t("aff_filters")}${active ? ` (${active})` : ""}`}>
                <SlidersHorizontal className="size-4" aria-hidden />
                {t("aff_filters")}
                {active > 0 && <span className="tabular-nums">{active}</span>}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="admin-shell w-[min(24rem,calc(100vw-2rem))]" aria-label={t("aff_purchases_filter_title")}>
              <div className="space-y-3">
                <h2 className="font-semibold">{t("aff_purchases_filter_title")}</h2>
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
                <span className="block text-sm font-medium">{t("aff_th_school")}</span>
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
                <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
                  <Button variant="ghost" onClick={() => { setStatus(""); setSchoolId(""); }}>
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
        {chips.length > 0 && (
          <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t("aff_active_filters")}>
            {chips.map((chip) => (
              <span key={chip.key} className="md-chip md-chip-primary gap-1">
                {chip.label}
                <Button variant="ghost" size="icon" className="size-8" aria-label={`${t("aff_remove_filter")}: ${chip.label}`} onClick={chip.clear}>
                  <X className="size-4" aria-hidden />
                </Button>
              </span>
            ))}
          </div>
        )}
      </div>

      {list.isError ? (
        <div className="md-card-outlined py-12 text-center text-ink-500">
          <p className="mb-4">{t("dash_load_failed")}</p>
          <button type="button" className="md-btn-tonal" onClick={() => list.refetch()}>
            {t("admin_home_reload")}
          </button>
        </div>
      ) : (
        <DataTable
          columns={columns}
          rows={list.isLoading ? [] : rows}
          rowKey={(r) => `${r.order_id}:${r.student_id}`}
          empty={list.isLoading ? t("aff_loading") : t("aff_purchases_empty")}
          footer={
            <TablePager
              total={total}
              offset={paging.offset}
              count={rows.length}
              limit={limit}
              onLimitChange={setLimit}
              canPrev={paging.canPrev}
              canNext={paging.offset + rows.length < total}
              onPrev={paging.prev}
              onNext={paging.next}
            />
          }
        />
      )}
    </div>
  );
}
