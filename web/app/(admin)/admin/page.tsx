"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  Shield, DollarSign, Receipt, UserPlus, Store, Clock, PackageCheck,
  AlertTriangle, Activity, Clipboard, BarChart3, type LucideIcon,
} from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/lib/hooks/auth";
import { useAdminAuditLog } from "@/lib/hooks/admin-audit";
import { useAdminDashboard } from "@/lib/hooks/admin-dashboard";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { adminHomeForRole } from "@/lib/auth-redirect";
import { NoAccess } from "@/components/admin/NoAccess";
import { StatCard } from "@/components/admin/StatCard";
import { PeriodBar, presetRange, type PeriodPreset, type PeriodRange } from "@/components/admin/PeriodBar";
import { AreaLineChart } from "@/components/admin/charts/AreaLineChart";
import { MultiLineChart } from "@/components/admin/charts/MultiLineChart";
import { StackedBarChart } from "@/components/admin/charts/StackedBarChart";
import { Skeleton } from "@/components/ui/skeleton";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { UserRole } from "@/lib/nav-config";
import type { AuditLogEntry, AdminDashboard } from "@/lib/types";

// Series colours, verified 3:1 against BOTH the light surface (#FFFFFF) and
// the dark surface (--md-sys-color-surface, #27213b) — the mockup's own
// hexes fail this on one surface or the other (#F5A623 2.03:1 light,
// #1A5CFF 2.94:1 dark). None of these six ever puts blue next to purple in
// the same chart, the flagged risky pair under protanopia/deuteranopia.
const CHART_BLUE = "#2F6FED"; // 4.55:1 light / 3.38:1 dark
const CHART_ORANGE = "#D2691E"; // 3.63:1 light / 4.23:1 dark
const CHART_PURPLE = "#9B51E0"; // 4.52:1 light / 3.40:1 dark
const CHART_TEAL = "#0E8074"; // 4.82:1 light / 3.19:1 dark
const CHART_AMBER = "#B26A00"; // 4.24:1 light / 3.62:1 dark
const CHART_MAGENTA = "#D6409F"; // 4.12:1 light / 3.73:1 dark

const EMPTY_DASHBOARD: AdminDashboard = {
  period: { from: "", to: "", bucket: "day" },
  kpi: {
    revenue: { value: 0 },
    order_count: { value: 0 },
    new_students: { value: 0 },
    schools: { value: 0 },
    students_total: { value: 0 },
  },
  series: [],
  attention: { needs_confirm: 0, ready_to_ship: 0, shipment_failed: 0, active_sessions: 0 },
  top_products: [],
  upcoming_exams: [],
};

function useFormatRelativeTime() {
  const { t, lang } = useTranslation();
  return (iso: string): string => {
    const now = Date.now();
    const then = new Date(iso).getTime();
    const diffMs = now - then;
    if (diffMs < 0) return t("time_just_now");
    const minutes = Math.floor(diffMs / 60000);
    if (minutes < 1) return t("time_just_now");
    if (minutes < 60) return `${minutes}${t("time_min_suffix")}`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}${t("time_hour_suffix")}`;
    const days = Math.floor(hours / 24);
    if (days < 7) return `${days}${t("time_day_suffix")}`;
    return new Date(iso).toLocaleDateString(lang === "en" ? "en-US" : "id-ID");
  };
}

// date/scheduled_at are RFC3339 strings already anchored to Asia/Jakarta
// (e.g. "2026-07-03T00:00:00+07:00"). Constructing a Date and immediately
// re-formatting through Intl.DateTimeFormat with an explicit timeZone is
// safe — it never lets the browser's own zone reinterpret the instant.
function useJakartaFormatter() {
  const { lang } = useTranslation();
  const locale = lang === "en" ? "en-US" : "id-ID";
  return {
    short: (iso: string) =>
      new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", timeZone: "Asia/Jakarta" }).format(new Date(iso)),
    dateTime: (iso: string) =>
      new Intl.DateTimeFormat(locale, {
        day: "numeric", month: "short", hour: "2-digit", minute: "2-digit", timeZone: "Asia/Jakarta",
      }).format(new Date(iso)),
  };
}

function computeDelta(value: number, prev?: number): number | null {
  if (prev === undefined || prev === 0) return null;
  return Math.round(((value - prev) / prev) * 100);
}

function DeltaText({ delta }: { delta: number | null }) {
  if (delta === null) return null;
  const color = delta > 0 ? "var(--md-sys-color-tertiary)" : delta < 0 ? "var(--md-sys-color-error)" : "var(--md-sys-color-on-surface-variant)";
  const sign = delta > 0 ? "+" : "";
  return (
    <div className="text-label" style={{ color }}>
      {sign}{delta}%
    </div>
  );
}

function KpiCard({
  icon: Icon, label, value, delta, sub,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  delta: number | null;
  sub?: string;
}) {
  return (
    <div className="md-card-elevated">
      <div className="flex items-start justify-between">
        <div>
          <div className="text-label color-on-surface-variant mb-2">{label}</div>
          <div className="text-title-large mb-2" style={{ fontWeight: 600 }}>{value}</div>
          <DeltaText delta={delta} />
          {sub ? <div className="text-label color-on-surface-variant">{sub}</div> : null}
        </div>
        <div
          className="flex size-10 items-center justify-center rounded-[12px]"
          style={{ backgroundColor: "var(--md-sys-color-primary-container)", color: "var(--md-sys-color-primary)" }}
        >
          <Icon size={20} />
        </div>
      </div>
    </div>
  );
}

// Mirrors QueueCardLink in app/(admin)/admin/store/page.tsx:34 — a count with
// nowhere to go is a dead end, the whole point of this band is to click
// through to the work behind it. Active sessions has no queue, so it's the
// one card in the row with no href.
function AttentionCard({
  testId, accent, label, value, icon: Icon, href,
}: {
  testId: string;
  accent: "primary" | "secondary" | "error" | "tertiary";
  label: string;
  value: number;
  icon: LucideIcon;
  href?: string;
}) {
  const card = (
    <div data-testid={testId} data-accent={accent}>
      <StatCard label={label} value={String(value)} accent={accent} icon={Icon} />
    </div>
  );
  if (!href) return card;
  return (
    <Link
      href={href}
      className="block rounded-[20px] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
      style={{ outlineColor: "var(--md-sys-color-primary)" }}
    >
      {card}
    </Link>
  );
}

export default function AdminIndexPage() {
  const router = useRouter();
  const { t } = useTranslation();
  const jakarta = useJakartaFormatter();
  const user = useAuthStore((s) => s.user);
  const storeRole = user?.role as UserRole | undefined;
  const me = useMe({ enabled: !storeRole });
  const role = storeRole ?? (me.data?.role as UserRole | undefined);
  const name = user?.name ?? me.data?.name ?? t("admin_home_default_name");
  const formatRelativeTime = useFormatRelativeTime();

  const quickActions = [
    { icon: Clipboard, label: t("admin_action_create_question"), route: "/admin/exam/tests" },
    { icon: Store, label: t("admin_action_add_product"), route: "/admin/products" },
    { icon: UserPlus, label: t("admin_action_register_student"), route: "/admin/school/students" },
    { icon: BarChart3, label: t("admin_action_sales_report"), route: "/admin/revenue" },
  ];

  const [preset, setPreset] = useState<PeriodPreset>("30d");
  const [range, setRange] = useState<PeriodRange>({});
  // Picking "Custom" before both dates are typed emits {} from PeriodBar,
  // indistinguishable server-side from the 30-day default while the UI still
  // shows "Custom" as selected. Rather than let that silently reuse stale
  // data under a misleading label, hold back the query's range until both
  // from and to are set — the dashboard keeps showing the last fully
  // resolved period until the custom range is complete.
  const [resolvedRange, setResolvedRange] = useState<PeriodRange>({});

  function handlePeriodChange(newPreset: PeriodPreset, newRange: PeriodRange) {
    setPreset(newPreset);
    setRange(newRange);
    if (newPreset !== "custom" || (newRange.from && newRange.to)) {
      setResolvedRange(newRange);
    }
  }

  const { data: auditEntries = [], isLoading: auditLoading, isError: auditError, refetch: refetchAudit } = useAdminAuditLog();
  const { data: dashboard, isLoading: dashboardLoading } = useAdminDashboard(resolvedRange);
  const canReadRevenue = useHasCapability("revenue:read");

  useEffect(() => {
    if (!role) return;
    if (role !== "super_admin") router.replace(adminHomeForRole(role));
  }, [role, router]);

  if (role !== "super_admin") return null;
  if (!canReadRevenue) return <NoAccess />;

  const d = dashboard ?? EMPTY_DASHBOARD;
  const labels = d.series.map((p) => jakarta.short(p.date));

  return (
    <div className="fade-in">
      {/* Hero band */}
      <div
        className="mb-8 rounded-[20px] px-8 py-7"
        style={{
          background: "linear-gradient(135deg, #1A5CFF 0%, #0A3DBF 55%, #005B8E 100%)",
          color: "#FFFFFF",
          boxShadow: "0 4px 24px rgba(26,92,255,0.28)",
        }}
      >
        <div className="flex items-center gap-6">
          <div
            className="flex size-[72px] shrink-0 items-center justify-center rounded-[24px]"
            style={{
              backgroundColor: "rgba(255,255,255,0.18)",
              backdropFilter: "blur(8px)",
            }}
          >
            <Shield size={36} color="#FFFFFF" />
          </div>
          <div>
            <div
              className="text-label"
              style={{ letterSpacing: "0.08em", textTransform: "uppercase", opacity: 0.75 }}
            >
              {t("admin_home_badge")}
            </div>
            <h1 className="text-headline" style={{ color: "#FFFFFF" }}>{name}</h1>
            <p className="text-body" style={{ marginTop: "4px", opacity: 0.85 }}>
              {t("admin_home_subtitle")}
            </p>
          </div>
        </div>
      </div>

      {/* Period bar */}
      <div className="mb-8">
        <PeriodBar
          preset={preset}
          range={range}
          onChange={handlePeriodChange}
          resolvedFrom={d.period.from || undefined}
          resolvedTo={d.period.to || undefined}
        />
      </div>

      {/* KPI row */}
      {dashboardLoading ? (
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-4 mb-8">
          {[...Array(4)].map((_, i) => <Skeleton key={i} className="h-28 w-full" />)}
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-4 mb-8">
          <KpiCard
            icon={DollarSign}
            label={t("admin_home_kpi_revenue")}
            value={formatRupiah(d.kpi.revenue.value)}
            delta={computeDelta(d.kpi.revenue.value, d.kpi.revenue.prev)}
          />
          <KpiCard
            icon={Receipt}
            label={t("admin_home_kpi_orders")}
            value={String(d.kpi.order_count.value)}
            delta={computeDelta(d.kpi.order_count.value, d.kpi.order_count.prev)}
          />
          <KpiCard
            icon={UserPlus}
            label={t("admin_home_kpi_new_students")}
            value={String(d.kpi.new_students.value)}
            delta={computeDelta(d.kpi.new_students.value, d.kpi.new_students.prev)}
          />
          <KpiCard
            icon={Store}
            label={t("admin_home_kpi_schools")}
            value={String(d.kpi.schools.value)}
            delta={computeDelta(d.kpi.schools.value, d.kpi.schools.prev)}
            sub={t("admin_home_kpi_students_total").replace("{count}", String(d.kpi.students_total.value))}
          />
        </div>
      )}

      {/* Chart 1 — revenue & order trend */}
      {dashboardLoading ? (
        <Skeleton className="h-40 w-full mb-8" />
      ) : (
        <div className="md-card-outlined mb-8">
          <h3 className="text-title-medium mb-4" title={t("admin_home_chart_revenue_sub")}>{t("admin_home_chart_revenue_title")}</h3>
          <AreaLineChart
            labels={labels}
            area={{ values: d.series.map((p) => p.revenue), color: CHART_BLUE, label: t("admin_home_series_revenue") }}
            line={{ values: d.series.map((p) => p.order_count), color: CHART_ORANGE, label: t("admin_home_series_orders") }}
            emptyLabel={t("admin_home_chart_empty")}
          />
        </div>
      )}

      {/* Charts 2 & 3 — student activity, digital vs physical mix */}
      {dashboardLoading ? (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 mb-8">
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 mb-8">
          <div className="md-card-outlined">
            <h3 className="text-title-medium mb-4" title={t("admin_home_chart_students_sub")}>{t("admin_home_chart_students_title")}</h3>
            <MultiLineChart
              labels={labels}
              series={[
                { values: d.series.map((p) => p.new_students), color: CHART_PURPLE, label: t("admin_home_series_new") },
                { values: d.series.map((p) => p.exam_students), color: CHART_TEAL, label: t("admin_home_series_exam") },
                { values: d.series.map((p) => p.buying_students), color: CHART_AMBER, label: t("admin_home_series_buy") },
              ]}
              emptyLabel={t("admin_home_chart_empty")}
            />
          </div>
          <div className="md-card-outlined">
            <h3 className="text-title-medium mb-4" title={t("admin_home_chart_mix_sub")}>{t("admin_home_chart_mix_title")}</h3>
            <StackedBarChart
              labels={labels}
              bottom={{ values: d.series.map((p) => p.revenue_digital), color: CHART_BLUE, label: t("admin_home_series_digital") }}
              top={{ values: d.series.map((p) => p.revenue_physical), color: CHART_MAGENTA, label: t("admin_home_series_physical") }}
              emptyLabel={t("admin_home_chart_empty")}
            />
          </div>
        </div>
      )}

      {/* Attention */}
      <div className="mb-8">
        <h3 className="text-title-large mb-4">{t("admin_home_attention")}</h3>
        {dashboardLoading ? (
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 xl:grid-cols-4">
            {[...Array(4)].map((_, i) => <Skeleton key={i} className="h-28 w-full" />)}
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 xl:grid-cols-4">
            <AttentionCard
              testId="attention-needs-confirm"
              href="/admin/orders?status=pending"
              accent="primary"
              label={t("admin_home_needs_confirm")}
              value={d.attention.needs_confirm}
              icon={Clock}
            />
            <AttentionCard
              testId="attention-ready-to-ship"
              href="/admin/orders?status=paid"
              accent="tertiary"
              label={t("admin_home_ready_to_ship")}
              value={d.attention.ready_to_ship}
              icon={PackageCheck}
            />
            <AttentionCard
              testId="attention-shipment-failed"
              href="/admin/orders?status=shipment_failed"
              accent={d.attention.shipment_failed > 0 ? "error" : "secondary"}
              label={t("admin_home_shipment_failed")}
              value={d.attention.shipment_failed}
              icon={AlertTriangle}
            />
            <AttentionCard
              testId="attention-active-sessions"
              accent="secondary"
              label={t("admin_home_active_sessions")}
              value={d.attention.active_sessions}
              icon={Activity}
            />
          </div>
        )}
      </div>

      {/* Upcoming exams + top products */}
      {dashboardLoading ? (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 mb-8">
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 mb-8">
          <div className="md-card-outlined">
            <h3 className="text-title-large mb-4">{t("admin_home_upcoming_exams")}</h3>
            {d.upcoming_exams.length === 0 ? (
              <div className="py-12 text-center text-ink-500">{t("admin_home_upcoming_empty")}</div>
            ) : (
              <div className="space-y-3">
                {d.upcoming_exams.map((exam) => (
                  <div
                    key={exam.id}
                    className="flex items-center justify-between gap-3 rounded-[12px] p-3"
                    style={{ backgroundColor: "var(--md-sys-color-surface-container-high)" }}
                  >
                    <div>
                      <div className="text-body" style={{ fontWeight: 500 }}>{exam.title}</div>
                      <div className="text-label color-on-surface-variant">{jakarta.dateTime(exam.scheduled_at)}</div>
                    </div>
                    <div className="text-label" style={{ color: "var(--md-sys-color-primary)" }}>
                      {t("admin_home_registrants").replace("{count}", String(exam.registrant_count))}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="md-card-outlined">
            <h3 className="text-title-large mb-4">{t("admin_home_top_products")}</h3>
            {d.top_products.length === 0 ? (
              <div className="py-12 text-center text-ink-500">{t("admin_home_chart_empty")}</div>
            ) : (
              <div className="space-y-3">
                {d.top_products.map((p, i) => (
                  <div key={p.product_id} className="flex items-center justify-between gap-3">
                    {/* A top product can legitimately share its name with an
                        upcoming exam (the product IS the exam registration),
                        so the rank prefix keeps this row's own text from
                        exact-matching that other leaf. */}
                    <div className="text-body" style={{ fontWeight: 500 }}>
                      {`${i + 1}. ${p.name} (${p.is_physical ? t("admin_home_badge_physical") : t("admin_home_badge_digital")})`}
                    </div>
                    <div className="text-label" style={{ fontWeight: 600 }}>{formatRupiah(p.product_revenue)}</div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Content grid */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Log Aktivitas */}
        <div className="lg:col-span-2 md-card-outlined">
          <div className="flex items-center justify-between mb-6">
            <h3 className="text-title-large">{t("admin_home_activity_log")}</h3>
            <button
              className="md-btn-tonal"
              type="button"
              onClick={() => router.push("/admin/system/audit")}
            >
              {t("admin_home_view_all")}
            </button>
          </div>
          {auditLoading ? (
            <div className="space-y-4">
              {[...Array(3)].map((_, i) => (
                <div key={i} className="flex items-center gap-3">
                  <Skeleton className="size-8 rounded-full" />
                  <div className="flex-1 space-y-2">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-3 w-64" />
                  </div>
                </div>
              ))}
            </div>
          ) : auditError ? (
            <div className="py-12 text-center text-ink-500">
              <p className="mb-4">{t("admin_home_audit_failed")}</p>
              <button
                type="button"
                className="md-btn-tonal"
                onClick={() => refetchAudit()}
              >
                {t("admin_home_reload")}
              </button>
            </div>
          ) : auditEntries.length === 0 ? (
            <div className="py-12 text-center text-ink-500">
              {t("admin_home_no_activity")}
            </div>
          ) : (
            <div className="space-y-4">
              {auditEntries.slice(0, 5).map((entry: AuditLogEntry) => (
                <div key={entry.id} className="flex items-center gap-3">
                  <div
                    className="flex size-8 items-center justify-center rounded-full"
                    style={{
                      backgroundColor: "var(--md-sys-color-primary-container)",
                      color: "var(--md-sys-color-primary)",
                    }}
                  >
                    <span className="text-label">
                      {(entry.actor_name ?? "?").charAt(0).toUpperCase()}
                    </span>
                  </div>
                  <div>
                    <div className="text-body" style={{ fontWeight: 500 }}>
                      {entry.actor_name ?? "System"}
                    </div>
                    <div className="text-label color-on-surface-variant">
                      {entry.action} · {entry.target_type} #{entry.target_id} · {formatRelativeTime(entry.created_at)}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Akses Cepat */}
        <div className="md-card-outlined">
          <h3 className="text-title-large mb-6">{t("admin_home_quick_actions")}</h3>
          <div className="grid grid-cols-2 gap-3">
            {quickActions.map((action, i) => (
              <button
                key={i}
                type="button"
                onClick={() => router.push(action.route)}
                className="flex flex-col items-center gap-2 p-4 rounded-[12px] border-none text-center transition-transform duration-200 hover:-translate-y-0.5 hover:shadow-lg"
                style={{
                  backgroundColor: "var(--md-sys-color-surface-container-high)",
                  cursor: "pointer",
                }}
              >
                <div
                  className="flex size-10 items-center justify-center rounded-[10px]"
                  style={{
                    backgroundColor: "var(--md-sys-color-primary-container)",
                    color: "var(--md-sys-color-primary)",
                  }}
                >
                  <action.icon size={20} />
                </div>
                <span className="text-label" style={{ fontWeight: 500 }}>
                  {action.label}
                </span>
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
