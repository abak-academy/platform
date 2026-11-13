"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Check, Search, ShoppingCart, SlidersHorizontal, X } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { partnerOrderErrorMessage } from "@/lib/partner-order-errors";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import {
  useCreatePartnerOrder,
  usePartnerProducts,
  usePartnerSchools,
  usePartnerStudents,
  usePartnerVouchers,
  usePreviewPartnerOrder,
} from "@/lib/hooks/partner";
import { useProductGroupings } from "@/lib/hooks/product-groupings";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";
import type { PartnerOrderPreview } from "@/lib/types";

const NO_VOUCHER = "none";
const ALL_GROUPINGS = "all";
const ALL_FILTER = "__all__";

function SearchField({
  value,
  onChange,
  placeholder,
  disabled,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <div className="relative min-w-0 flex-1">
      <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
      <Input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={placeholder}
        disabled={disabled}
        className="pr-10 pl-9"
      />
      {value && (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="absolute top-0 right-0 size-9"
          aria-label={t("aff_search_clear")}
          onClick={() => onChange("")}
        >
          <X className="size-4" aria-hidden />
        </Button>
      )}
    </div>
  );
}

function PickerActionRow({
  count,
  countLabel,
  onSelectAll,
  selectAllDisabled,
  onClearAll,
  clearAllDisabled,
}: {
  count: number;
  countLabel: string;
  onSelectAll: () => void;
  selectAllDisabled: boolean;
  onClearAll: () => void;
  clearAllDisabled: boolean;
}) {
  const { t } = useTranslation();
  return (
    <div className="flex items-center justify-between text-xs text-ink-500">
      <span>{countLabel.replace("{n}", String(count))}</span>
      <div className="flex gap-3">
        <button type="button" onClick={onSelectAll} disabled={selectAllDisabled} className="font-medium text-brand-600 hover:underline disabled:cursor-default disabled:text-ink-400 disabled:no-underline">
          {t("partner_new_select_all")}
        </button>
        <button type="button" onClick={onClearAll} disabled={clearAllDisabled} className="font-medium text-ink-500 hover:underline disabled:cursor-default disabled:text-ink-300 disabled:no-underline">
          {t("partner_new_deselect_all")}
        </button>
      </div>
    </div>
  );
}

export default function NewPartnerOrderPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const products = usePartnerProducts();
  const groupings = useProductGroupings();
  const vouchers = usePartnerVouchers();
  const schools = usePartnerSchools();

  const [groupingId, setGroupingId] = useState(ALL_GROUPINGS);
  const [productSearch, setProductSearch] = useState("");
  const [selectedProductIds, setSelectedProductIds] = useState<string[]>([]);
  const [voucher, setVoucher] = useState(NO_VOUCHER);
  const [q, setQ] = useState("");
  const [studentMine, setStudentMine] = useState("");
  const [studentSchoolId, setStudentSchoolId] = useState("");
  const [studentStatus, setStudentStatus] = useState("");
  const [selected, setSelected] = useState<Record<string, string>>({});
  const [creating, setCreating] = useState(false);
  const [preview, setPreview] = useState<PartnerOrderPreview | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const students = usePartnerStudents({
    q: useDebouncedValue(q, 300),
    productIds: selectedProductIds,
    registeredByMe: studentMine === "mine" || undefined,
    schoolId: studentSchoolId || undefined,
    status: studentStatus || undefined,
  });
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

  const studentFilterCount = (studentMine ? 1 : 0) + (studentSchoolId ? 1 : 0) + (studentStatus ? 1 : 0);
  const studentChips = useMemo(() => {
    const chips: { key: string; label: string; clear: () => void }[] = [];
    if (studentMine) {
      chips.push({ key: "mine", label: `${t("partner_students_filter_registered")}: ${t("partner_students_source_me")}`, clear: () => setStudentMine("") });
    }
    if (studentSchoolId) {
      const school = (schools.data?.data ?? []).find((sc) => sc.id === studentSchoolId);
      chips.push({ key: "school", label: `${t("partner_students_filter_school_label")}: ${school?.name ?? studentSchoolId}`, clear: () => setStudentSchoolId("") });
    }
    if (studentStatus) {
      const statusLabel = t(studentStatus === "active" ? "partner_students_status_active" : "partner_students_status_deactivated");
      chips.push({ key: "status", label: `${t("partner_students_filter_account")}: ${statusLabel}`, clear: () => setStudentStatus("") });
    }
    return chips;
  }, [studentMine, studentSchoolId, studentStatus, schools.data, t]);

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

  // Adds every currently filtered (grouping + search) product to the selection,
  // leaving a selection outside the current filter untouched.
  const selectAllVisibleProducts = () => {
    setSelectedProductIds((prev) => Array.from(new Set([...prev, ...visibleProducts.map((p) => p.id)])));
  };
  const clearSelectedProducts = () => setSelectedProductIds([]);

  // Only ever selects students who are not blocked by an already-owned product.
  const selectAllVisibleStudents = () => {
    setSelected((prev) => {
      const next = { ...prev };
      selectableRows.forEach((s) => {
        next[s.id] = s.name;
      });
      return next;
    });
  };
  const clearSelectedStudents = () => setSelected({});

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
          <SearchField value={productSearch} onChange={setProductSearch} placeholder={t("partner_new_search_ph")} disabled={creating} />
          {(groupings.data?.length ?? 0) > 0 && (
            <Select value={groupingId} onValueChange={setGroupingId} disabled={creating}>
              <SelectTrigger aria-label={t("partner_new_grouping_label")} className="w-full shrink-0 sm:w-48">
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
            <PickerActionRow
              count={selectedProductIds.length}
              countLabel={t("partner_new_selected_products")}
              onSelectAll={selectAllVisibleProducts}
              selectAllDisabled={creating || visibleProducts.length === 0}
              onClearAll={clearSelectedProducts}
              clearAllDisabled={creating || selectedProductIds.length === 0}
            />
            <div className="max-h-64 space-y-1 overflow-y-auto rounded-lg border border-line p-2">
              {visibleProducts.map((prod) => {
                const isSelected = selectedProductIds.includes(prod.id);
                return (
                  <button
                    key={prod.id}
                    type="button"
                    role="checkbox"
                    aria-checked={isSelected}
                    aria-label={prod.name}
                    disabled={creating}
                    onClick={() => toggleProduct(prod.id)}
                    className={cn(
                      "flex w-full items-center gap-3 rounded-md border border-transparent px-3 py-2.5 text-left text-sm transition-colors hover:bg-surface-2 disabled:cursor-default disabled:opacity-60",
                      isSelected && "border-brand-600 bg-brand-50",
                    )}
                  >
                    <span
                      aria-hidden
                      className={cn(
                        "flex size-5 shrink-0 items-center justify-center rounded border",
                        isSelected ? "border-brand-600 bg-brand-600 text-white" : "border-line-strong",
                      )}
                    >
                      {isSelected && <Check className="size-3.5" />}
                    </span>
                    <span className="min-w-0 flex-1 truncate font-medium text-ink-900">{prod.name}</span>
                    <Badge variant="secondary" className="shrink-0 font-normal">
                      {formatRupiah(prod.price)}
                    </Badge>
                  </button>
                );
              })}
            </div>
          </>
        )}

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
        <div>
          <h2 className="font-semibold text-ink-900">{t("partner_new_students")}</h2>
          <p className="text-xs text-ink-500">{t("partner_new_students_hint")}</p>
        </div>

        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <SearchField value={q} onChange={setQ} placeholder={t("partner_students_search")} disabled={creating} />
          <Popover>
            <PopoverTrigger asChild>
              <Button
                type="button"
                variant="outline"
                className="h-9 shrink-0"
                disabled={creating}
                aria-label={`${t("aff_filter_button")}${studentFilterCount ? ` (${studentFilterCount})` : ""}`}
              >
                <SlidersHorizontal className="size-4" aria-hidden />
                {t("aff_filter_button")}
                {studentFilterCount > 0 && <span className="tabular-nums">{studentFilterCount}</span>}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" className="admin-shell w-[min(22rem,calc(100vw-2rem))]" aria-label={t("aff_filter_title")}>
              <div className="space-y-3">
                <h3 className="font-semibold">{t("aff_filter_title")}</h3>
                <div className="space-y-1.5">
                  <span className="block text-sm font-medium">{t("partner_students_filter_registered")}</span>
                  <Select value={studentMine || ALL_FILTER} onValueChange={(v) => setStudentMine(v === ALL_FILTER ? "" : v)}>
                    <SelectTrigger className="w-full" aria-label={t("partner_students_filter_registered")}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={ALL_FILTER}>{t("aff_filter_all")}</SelectItem>
                      <SelectItem value="mine">{t("partner_students_source_me")}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <span className="block text-sm font-medium">{t("partner_students_filter_school_label")}</span>
                  <Select value={studentSchoolId || ALL_FILTER} onValueChange={(v) => setStudentSchoolId(v === ALL_FILTER ? "" : v)}>
                    <SelectTrigger className="w-full" aria-label={t("partner_students_filter_school_label")}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={ALL_FILTER}>{t("aff_filter_all")}</SelectItem>
                      {(schools.data?.data ?? []).map((sc) => (
                        <SelectItem key={sc.id} value={sc.id}>
                          {sc.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <span className="block text-sm font-medium">{t("partner_students_filter_account")}</span>
                  <Select value={studentStatus || ALL_FILTER} onValueChange={(v) => setStudentStatus(v === ALL_FILTER ? "" : v)}>
                    <SelectTrigger className="w-full" aria-label={t("partner_students_filter_account")}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={ALL_FILTER}>{t("aff_filter_all")}</SelectItem>
                      <SelectItem value="active">{t("partner_students_status_active")}</SelectItem>
                      <SelectItem value="deactivated">{t("partner_students_status_deactivated")}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-wrap items-center justify-between gap-2 border-t border-line pt-3">
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={() => {
                      setStudentMine("");
                      setStudentSchoolId("");
                      setStudentStatus("");
                    }}
                  >
                    {t("aff_filter_reset")}
                  </Button>
                  <PopoverClose asChild>
                    <Button type="button">{t("aff_filter_show")}</Button>
                  </PopoverClose>
                </div>
              </div>
            </PopoverContent>
          </Popover>
        </div>

        {studentChips.length > 0 && (
          <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t("aff_filter_active")}>
            {studentChips.map((chip) => (
              <span key={chip.key} className="md-chip md-chip-primary gap-1">
                {chip.label}
                <Button variant="ghost" size="icon" className="size-6" aria-label={`${t("aff_filter_remove")}: ${chip.label}`} onClick={chip.clear}>
                  <X className="size-3.5" aria-hidden />
                </Button>
              </span>
            ))}
          </div>
        )}

        {students.isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : noSchools ? (
          <p className="text-sm text-ink-600">{t("partner_students_no_schools")}</p>
        ) : rows.length === 0 ? (
          <p className="text-sm text-ink-600">{t("partner_students_empty")}</p>
        ) : (
          <>
            <PickerActionRow
              count={ids.length}
              countLabel={t("partner_new_selected")}
              onSelectAll={selectAllVisibleStudents}
              selectAllDisabled={creating || selectableRows.length === 0}
              onClearAll={clearSelectedStudents}
              clearAllDisabled={creating || ids.length === 0}
            />
            <div className="max-h-72 space-y-1 overflow-y-auto rounded-lg border border-line p-2">
              {rows.map((s) => {
                const blocked = (s.owned_product_ids?.length ?? 0) > 0;
                const isSelected = s.id in selected;
                return (
                  <button
                    key={s.id}
                    type="button"
                    role="checkbox"
                    aria-checked={isSelected}
                    aria-label={s.name}
                    disabled={creating || blocked}
                    onClick={() => toggleStudent(s.id, s.name)}
                    className={cn(
                      "flex w-full items-center gap-3 rounded-md border border-transparent px-3 py-2.5 text-left text-sm transition-colors hover:bg-surface-2 disabled:cursor-default disabled:opacity-60 disabled:hover:bg-transparent",
                      isSelected && "border-brand-600 bg-brand-50",
                    )}
                  >
                    <span
                      aria-hidden
                      className={cn(
                        "flex size-5 shrink-0 items-center justify-center rounded border",
                        isSelected ? "border-brand-600 bg-brand-600 text-white" : "border-line-strong",
                      )}
                    >
                      {isSelected && <Check className="size-3.5" />}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate font-medium text-ink-900">{s.name}</span>
                      <span className="block truncate text-xs text-ink-500">{s.school_name ?? "-"}</span>
                    </span>
                    {blocked && (
                      <Badge variant="outline" className="shrink-0 border-transparent bg-warn-bg text-warn">
                        {t("partner_students_owns_selected")}
                      </Badge>
                    )}
                  </button>
                );
              })}
            </div>
          </>
        )}
        {students.hasNextPage && (
          <Button variant="outline" size="sm" onClick={() => students.fetchNextPage()} disabled={students.isFetchingNextPage}>
            {t("partner_load_more")}
          </Button>
        )}
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
