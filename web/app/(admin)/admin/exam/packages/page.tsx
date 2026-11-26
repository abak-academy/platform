"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Calendar, Info, Plus, Trophy } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { ExamModal } from "@/components/admin/ExamModal";
import { Button } from "@/components/ui/button";
import { DataTable, TableItem } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import {
  ActiveChip,
  FILTER_PANEL_CLASS,
  FilterGrid,
  FilterPanelFooter,
  FilterSelect,
  ToolbarCount,
  ToolbarFilterButton,
  ToolbarSearch,
  ToolbarSort,
} from "@/components/admin/ListToolbar";
import { Skeleton } from "@/components/ui/skeleton";
import { useExams } from "@/lib/hooks/admin-exams";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { examModeLabel } from "@/lib/exam-mode";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import { accessWindow } from "@/lib/exam-schedule";
import { useAuthStore } from "@/stores/auth";
import type { ExamListItem } from "@/lib/types";

function Sub({ children }: { children: React.ReactNode }) {
  return <small className="block text-xs leading-[1.55] text-ink-600">{children}</small>;
}

type Phase = "upcoming" | "open" | "ended" | "none";

function phaseOf(exam: ExamListItem, now: number): Phase {
  const start = exam.scheduled_at ? Date.parse(exam.scheduled_at) : NaN;
  if (Number.isNaN(start)) return "none";
  if (now < start) return "upcoming";
  const end = exam.scheduled_end_at ? Date.parse(exam.scheduled_end_at) : NaN;
  return !Number.isNaN(end) && now > end ? "ended" : "open";
}

const startTime = (e: ExamListItem) => (e.scheduled_at ? Date.parse(e.scheduled_at) : NaN);

function plural(t: (key: I18nKey) => string, key: "exam_admin_list_sections" | "exam_admin_list_questions" | "exam_admin_list_sessions_started", n: number) {
  return t(n === 1 ? `${key}_one` : key).replace("{n}", String(n));
}

export default function ExamPackagesPage() {
  const { t, lang } = useTranslation();
  const router = useRouter();
  const role = useAuthStore((s) => s.user?.role);
  const [showCreate, setShowCreate] = useState(false);
  const [editing, setEditing] = useState<ExamListItem | null>(null);

  const [searchInput, setSearchInput] = useState("");
  const searchRef = useRef<HTMLDivElement>(null);
  const q = useDebouncedValue(searchInput).trim();
  const [cursor, setCursor] = useState<string | undefined>();
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [items, setItems] = useState<ExamListItem[]>([]);
  const [phase, setPhase] = useState("");
  const [product, setProduct] = useState("");
  const [sort, setSort] = useState("default");

  // A debounced search commit must restart pagination at page 1.
  useEffect(() => {
    setItems([]);
    setCursor(undefined);
    setNextCursor(undefined);
  }, [q]);

  const filters = useMemo(
    () => ({ cursor, limit: 20, q: q || undefined }),
    [cursor, q]
  );

  const { data, isLoading, isFetching, isError, error, isPlaceholderData } = useExams(filters);

  useEffect(() => {
    // keepPreviousData: don't adopt the old query's rows and cursor while the new one loads.
    if (!data || isPlaceholderData) return;
    setItems((previous) => {
      if (!cursor) return data.data;
      const ids = new Set(previous.map((exam) => exam.id));
      return [...previous, ...data.data.filter((exam) => !ids.has(exam.id))];
    });
    setNextCursor(data.next_cursor);
  }, [cursor, data, q, isPlaceholderData]);

  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(id);
  }, []);

  // Filters and sorts only see loaded rows; sort is locked until every page is in.
  const partial = !!nextCursor;
  const clientFiltered = !!phase || !!product;
  const shown = useMemo(() => {
    const rows = items.filter(
      (e) =>
        (!phase || phaseOf(e, now) === phase) &&
        (!product || (product === "on") === !!e.has_published_product)
    );
    if (sort === "default") return rows;
    const dir = sort === "late" ? -1 : 1;
    return [...rows].sort((a, b) => {
      if (sort === "reg") return (b.registration_count ?? 0) - (a.registration_count ?? 0);
      const [x, y] = [startTime(a), startTime(b)];
      if (Number.isNaN(x) || Number.isNaN(y)) return Number.isNaN(x) ? (Number.isNaN(y) ? 0 : 1) : -1;
      return (x - y) * dir;
    });
  }, [items, phase, product, sort, now]);

  const phaseOptions = (["upcoming", "open", "ended", "none"] as const).map((v) => ({
    value: v,
    label: t(`exam_filter_phase_${v}` as I18nKey),
  }));
  const productOptions = [
    { value: "on", label: t("exam_filter_product_on") },
    { value: "off", label: t("exam_filter_product_off") },
  ];
  const active = [
    ...(phase ? [{ key: "phase", label: `${t("exam_filter_phase")}: ${phaseOptions.find((o) => o.value === phase)?.label}`, clear: () => setPhase("") }] : []),
    ...(product ? [{ key: "product", label: `${t("exam_filter_product")}: ${productOptions.find((o) => o.value === product)?.label}`, clear: () => setProduct("") }] : []),
  ];
  const resetFilters = () => {
    setPhase("");
    setProduct("");
  };

  return (
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        title={t("exam_packages_page_title")}
        description={t("exam_packages_page_description")}
        actions={
          role !== "admin_school" ? (
            <Button variant="dark" onClick={() => setShowCreate(true)}>
              <Plus aria-hidden />
              {t("exam_packages_create")}
            </Button>
          ) : undefined
        }
      />

      <div className="md-card-flush">
          <div className="flex flex-wrap items-center gap-2.5 border-b border-line px-4 py-3.5">
            <div ref={searchRef} className="contents">
              <ToolbarSearch
                value={searchInput}
                onChange={(v) => {
                  setSearchInput(v);
                  if (!v) searchRef.current?.querySelector("input")?.focus();
                }}
                placeholder={t("exam_packages_search_placeholder")}
                clearLabel={t("exam_packages_search_clear")}
              />
            </div>
            <Popover>
              <PopoverTrigger asChild>
                <ToolbarFilterButton label={t("list_toolbar_filter")} count={active.length} />
              </PopoverTrigger>
              <PopoverContent align="end" className={FILTER_PANEL_CLASS} aria-label={t("list_toolbar_filter")}>
                <div className="flex flex-col gap-3.5">
                  <FilterGrid>
                    <FilterSelect label={t("exam_filter_phase")} value={phase} options={phaseOptions} onChange={setPhase} />
                    <FilterSelect label={t("exam_filter_product")} value={product} options={productOptions} onChange={setProduct} />
                  </FilterGrid>
                  <FilterPanelFooter
                    onReset={resetFilters}
                    showLabel={t("list_toolbar_show_n").replace("{n}", `${shown.length}${partial && clientFiltered ? "+" : ""}`)}
                  />
                </div>
              </PopoverContent>
            </Popover>
            <ToolbarSort
              value={sort}
              onChange={setSort}
              label={t("list_toolbar_sort")}
              options={[
                { value: "default", label: t("filter_sort_default") },
                { value: "soon", label: t("exam_sort_soon"), disabled: partial },
                { value: "late", label: t("exam_sort_late"), disabled: partial },
                { value: "reg", label: t("exam_sort_reg"), disabled: partial },
              ]}
            />
            <ToolbarCount>
              {t("exam_packages_count").replace("{n}", `${shown.length}${partial ? "+" : ""}`)}
            </ToolbarCount>
          </div>
          {partial && (
            <p className="border-b border-line px-4 py-2 text-xs text-ink-600">{t("filter_sort_locked")}</p>
          )}
          {active.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5 border-b border-line px-4 py-2.5" role="group" aria-label={t("list_toolbar_active")}>
              {active.map((c) => (
                <ActiveChip key={c.key} label={c.label} removeLabel={`${t("list_toolbar_remove")}: ${c.label}`} onRemove={c.clear} />
              ))}
              <Button variant="ghost" size="sm" onClick={resetFilters}>{t("list_toolbar_clear_all")}</Button>
            </div>
          )}
          {isLoading ? (
            <div className="space-y-2 p-4">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : isError ? (
            <div className="m-4 rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger">
              {error instanceof Error ? error.message : t("error_generic")}
            </div>
          ) : (
          <DataTable
            surface="plain"
            rows={shown}
            rowKey={(exam) => exam.id}
            onRowClick={(exam) => router.push(`/admin/exam/packages/${exam.id}`)}
            empty={
              partial && clientFiltered ? (
                <EmptyState
                  icon={Calendar}
                  title={t("exam_packages_empty")}
                  text={t("filter_empty_loaded")}
                  action={
                    <Button variant="outline" onClick={() => setCursor(nextCursor)} disabled={isFetching}>
                      {isFetching ? t("sys_loading") : t("load_more")}
                    </Button>
                  }
                />
              ) : (
                <EmptyState icon={Calendar} title={t("exam_packages_empty")} />
              )
            }
            columns={[
              {
                key: "exam",
                header: t("exam_admin_col_exam"),
                className: "min-w-[14rem] max-w-[24rem]",
                cell: (exam) => (
                  <TableItem
                    thumb={
                      <span className="grid size-[38px] shrink-0 place-items-center rounded-[12px] bg-brand-100 text-brand-800">
                        <Trophy className="size-4" aria-hidden />
                      </span>
                    }
                    title={exam.title}
                    sub={
                      <>
                        {/* ?? not || — a registration_count of 0 must still render */}
                        {t("exam_packages_registered_count").replace("{n}", String(exam.registration_count ?? 0))}
                        {exam.format_version === 2 && exam.session_count != null && (
                          <>
                            {" · "}
                            {exam.session_count === 0
                              ? t("exam_admin_list_no_sessions")
                              : plural(t, "exam_admin_list_sessions_started", exam.session_count)}
                          </>
                        )}
                      </>
                    }
                  />
                ),
              },
              {
                key: "composition",
                nowrap: true,
                header: t("exam_admin_col_composition"),
                cell: (exam) =>
                  exam.format_version === 2 ? (
                    <>
                      <span>{plural(t, "exam_admin_list_sections", exam.section_count ?? 0)}</span>
                      <Sub>{plural(t, "exam_admin_list_questions", exam.question_count ?? 0)}</Sub>
                    </>
                  ) : (
                    <>
                      <span>{t("exam_admin_legacy_format")}</span>
                      <Sub>{t("exam_admin_legacy_settings")}</Sub>
                    </>
                  ),
              },
              {
                key: "delivery",
                nowrap: true,
                header: t("exam_admin_col_delivery"),
                cell: (exam) =>
                  exam.format_version === 2 ? (
                    <>
                      <span>
                        {t(
                          exam.navigation_policy === "sequential"
                            ? "exam_admin_list_nav_sequential"
                            : "exam_admin_list_nav_free"
                        )}
                      </span>
                      <Sub>{t("exam_admin_presentation_per_section")}</Sub>
                    </>
                  ) : (
                    <>
                      <span>{examModeLabel(exam.mode, t)}</span>
                      <Sub>{t("exam_admin_legacy_format")}</Sub>
                    </>
                  ),
              },
              {
                key: "time",
                nowrap: true,
                header: t("exam_admin_col_time"),
                cell: (exam) => {
                  const composable = exam.format_version === 2;
                  if (composable && exam.timer_mode === "per_test") {
                    return (
                      <>
                        <span>{t("exam_admin_time_per_section")}</span>
                        {exam.sections_duration_minutes != null && (
                          <Sub>
                            {t("exam_admin_time_sections_total").replace("{n}", String(exam.sections_duration_minutes))}
                          </Sub>
                        )}
                      </>
                    );
                  }
                  if (exam.duration_minutes == null) return "—";
                  return (
                    <>
                      <span>
                        {t(composable ? "exam_admin_time_overall" : "exam_admin_time_legacy").replace(
                          "{n}",
                          String(exam.duration_minutes)
                        )}
                      </span>
                      <Sub>{t(composable ? "exam_admin_time_overall_hint" : "exam_admin_time_legacy_hint")}</Sub>
                    </>
                  );
                },
              },
              {
                key: "access",
                nowrap: true,
                header: t("exam_admin_col_access"),
                cell: (exam) => {
                  const access = accessWindow(exam.scheduled_at ?? null, exam.scheduled_end_at ?? null, lang);
                  if (!access) return t("exam_admin_access_none");
                  return (
                    <>
                      <span>{`${access.from.date}, ${access.from.time} WIB`}</span>
                      <Sub>
                        {access.to
                          ? t("exam_admin_access_until").replace("{date}", `${access.to.date}, ${access.to.time} WIB`)
                          : t("exam_admin_access_no_end")}
                      </Sub>
                    </>
                  );
                },
              },
            ]}
          />
          )}
      </div>

      {!isLoading && !isError && (
        <p className="flex flex-wrap items-center gap-1.5 text-[13px] leading-[1.55] text-ink-600">
          <Info className="size-4" aria-hidden />
          {t("exam_admin_products_footnote")}
          <Link href="/admin/products" className="font-bold text-brand-ink hover:underline">
            {t("admin_nav_products")}
          </Link>
        </p>
      )}

      {nextCursor && !isError && (
        <div className="text-center">
          <Button
            variant="outline"
            onClick={() => setCursor(nextCursor)}
            disabled={isFetching}
          >
            {isFetching ? t("sys_loading") : t("load_more")}
          </Button>
        </div>
      )}

      <ExamModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onSaved={() => setShowCreate(false)}
      />

      <ExamModal
        open={Boolean(editing)}
        exam={editing}
        onClose={() => setEditing(null)}
        onSaved={() => setEditing(null)}
      />
    </div>
  );
}
