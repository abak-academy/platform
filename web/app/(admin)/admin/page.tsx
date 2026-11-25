"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  Receipt, UserPlus, Store, Clock, PackageCheck, AlertTriangle, PenLine, Coins,
  ArrowRight, ArrowUp, ArrowDown, Plus, Calendar, Megaphone, Tag, type LucideIcon,
} from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/lib/hooks/auth";
import { useAdminDashboard } from "@/lib/hooks/admin-dashboard";
import { useAdminNotifications } from "@/lib/hooks/admin-notifications";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { adminHomeForRole } from "@/lib/auth-redirect";
import { NoAccess } from "@/components/admin/NoAccess";
import { RevenueMixCard } from "@/components/admin/RevenueMixCard";
import { PeriodBar, type PeriodPreset, type PeriodRange } from "@/components/admin/PeriodBar";
import { AreaLineChart } from "@/components/admin/charts/AreaLineChart";
import { MultiLineChart } from "@/components/admin/charts/MultiLineChart";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { UserRole } from "@/lib/nav-config";
import type { AdminDashboard } from "@/lib/types";

// Series colours, verified 3:1 against BOTH the light surface (#FFFFFF) and
// the dark surface (--md-sys-color-surface, #211b34) — the mockup's own
// hexes fail this on one surface or the other (#F5A623 2.03:1 light,
// #1A5CFF 2.94:1 dark). None of these six ever puts blue next to purple in
// the same chart, the flagged risky pair under protanopia/deuteranopia.
const CHART_BLUE = "#2F6FED"; // 4.55:1 light / 3.38:1 dark
const CHART_ORANGE = "#D2691E"; // 3.63:1 light / 4.23:1 dark
const CHART_PURPLE = "#9B51E0"; // 4.52:1 light / 3.40:1 dark
const CHART_TEAL = "#0E8074"; // 4.82:1 light / 3.19:1 dark
const CHART_AMBER = "#B26A00"; // 4.24:1 light / 3.62:1 dark

const EMPTY_DASHBOARD: AdminDashboard = {
  period: { from: "", to: "", bucket: "day" },
  kpi: {
    revenue: { value: 0 },
    order_count: { value: 0 },
    new_students: { value: 0 },
    schools: { value: 0 },
    students_total: { value: 0 },
    revenue_digital: { value: 0 },
    revenue_physical: { value: 0 },
  },
  series: [],
  attention: {
    needs_confirm: 0, ready_to_ship: 0, shipment_failed: 0, active_sessions: 0,
    essays_to_grade: 0, essays_exam_id: null,
  },
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

function jakartaHour(d: Date): number {
  return Number(new Intl.DateTimeFormat("en-GB", { hour: "2-digit", hourCycle: "h23", timeZone: "Asia/Jakarta" }).format(d));
}

const KPI_TONE = {
  brand: { card: "bg-brand-50", chip: "text-brand-700" },
  ember: { card: "bg-ember-50", chip: "text-ember-700" },
  teal: { card: "bg-teal-50", chip: "text-teal-700" },
  sky: { card: "bg-sky-50", chip: "text-sky-700" },
};

function KpiCard({
  tone, icon: Icon, label, value, delta, sub,
}: {
  tone: keyof typeof KPI_TONE;
  icon: LucideIcon;
  label: string;
  value: string;
  delta: number | null;
  sub?: string;
}) {
  const { t } = useTranslation();
  const tn = KPI_TONE[tone];
  const DeltaIcon = delta !== null && delta > 0 ? ArrowUp : ArrowDown;
  const deltaColor = delta === null ? "" : delta > 0 ? "text-teal-700" : delta < 0 ? "text-danger-600" : "text-ink-600";
  return (
    <div className={`flex items-start justify-between gap-3 rounded-[var(--radius-card)] p-5 ${tn.card}`}>
      <div className="flex min-w-0 flex-col gap-1">
        <div className="type-label text-ink-600">{label}</div>
        <div className="type-h1 text-ink-900">{value}</div>
        {delta !== null && (
          <div className={`flex items-center gap-1 type-caption ${deltaColor}`}>
            {delta !== 0 && <DeltaIcon aria-hidden className="size-3" />}
            <span>{delta > 0 ? "+" : ""}{delta}% {t("admin_home_vs_prev")}</span>
          </div>
        )}
        {sub ? <div className="type-caption text-ink-600">{sub}</div> : null}
      </div>
      <span className={`flex size-10 shrink-0 items-center justify-center rounded-[var(--radius-md)] bg-surface ${tn.chip}`}>
        <Icon aria-hidden className="size-5" />
      </span>
    </div>
  );
}

const ATTENTION_TONE = {
  gold: "bg-gold-50 text-gold-700",
  sky: "bg-sky-50 text-sky-700",
  ember: "bg-ember-50 text-ember-700",
  danger: "bg-danger-50 text-danger-600",
};

// The whole card is the link: a count with nowhere to go is a dead end.
function AttentionCard({
  testId, tone, label, value, icon: Icon, href,
}: {
  testId: string;
  tone: keyof typeof ATTENTION_TONE;
  label: string;
  value: number;
  icon: LucideIcon;
  href?: string;
}) {
  const className = "flex items-center gap-4 rounded-[var(--radius-card)] border border-line bg-card p-4 shadow-card";
  const body = (
    <>
      <span className={`flex size-11 shrink-0 items-center justify-center rounded-[var(--radius-md)] ${ATTENTION_TONE[tone]}`}>
        <Icon aria-hidden className="size-5" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="type-caption text-ink-600">{label}</div>
        <div className="type-h1 text-ink-900">{value}</div>
      </div>
      {href && <ArrowRight aria-hidden className="size-4 text-ink-600" />}
    </>
  );
  if (!href) return <div data-testid={testId} className={className}>{body}</div>;
  return (
    <Link
      href={href}
      data-testid={testId}
      className={`${className} outline-none transition-shadow hover:shadow-pop focus-visible:outline-[2.5px] focus-visible:outline-offset-2 focus-visible:outline-solid focus-visible:outline-brand-600`}
    >
      {body}
    </Link>
  );
}

export default function AdminIndexPage() {
  const router = useRouter();
  const { t, lang } = useTranslation();
  const jakarta = useJakartaFormatter();
  const user = useAuthStore((s) => s.user);
  const storeRole = user?.role as UserRole | undefined;
  const me = useMe({ enabled: !storeRole });
  const role = storeRole ?? (me.data?.role as UserRole | undefined);
  const name = user?.name ?? me.data?.name ?? t("admin_home_default_name");
  const formatRelativeTime = useFormatRelativeTime();

  const quickActions: { icon: LucideIcon; label: string; href: string }[] = [
    { icon: Plus, label: t("admin_action_add_product"), href: "/admin/products" },
    { icon: Calendar, label: t("admin_home_create_exam"), href: "/admin/exam/packages" },
    { icon: Megaphone, label: t("admin_home_action_announce"), href: "/admin/notifications" },
    { icon: Tag, label: t("admin_home_action_promo"), href: "/admin/promos" },
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

  const {
    data: dashboard,
    isLoading: dashboardLoading,
    isError: dashboardIsError,
    isPaused: dashboardIsPaused,
    refetch: refetchDashboard,
  } = useAdminDashboard(resolvedRange);
  // TanStack Query pauses a scheduled retry while document.visibilityState is
  // "hidden" (tab backgrounded during the retry delay) and only resumes on the
  // next visibilitychange — until then fetchStatus sits at "paused" with
  // isLoading and isError both false and data still undefined, so the plain
  // isError flag never fires and the page below would render fabricated zeros
  // instead of dash_load_failed. Folding in isPaused (only when we have
  // nothing to show yet) catches that stuck state too.
  const dashboardError = dashboardIsError || (dashboardIsPaused && !dashboard);
  const notifications = useAdminNotifications(undefined, { refetchInterval: 30_000 });
  const canReadRevenue = useHasCapability("revenue:read");

  useEffect(() => {
    if (!role) return;
    if (role !== "super_admin") router.replace(adminHomeForRole(role));
  }, [role, router]);

  if (role !== "super_admin") return null;
  if (!canReadRevenue) return <NoAccess />;

  const d = dashboard ?? EMPTY_DASHBOARD;
  const labels = d.series.map((p) => jakarta.short(p.date));
  const now = new Date();
  const hour = jakartaHour(now);
  const greeting = t(
    hour < 11 ? "admin_home_greet_morning" : hour < 15 ? "admin_home_greet_afternoon" : hour < 18 ? "admin_home_greet_evening" : "admin_home_greet_night",
  );
  const today = new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", {
    weekday: "long", day: "numeric", month: "long", year: "numeric", timeZone: "Asia/Jakarta",
  }).format(now);
  const a = d.attention;
  const topMax = Math.max(1, ...d.top_products.map((p) => p.qty_sold));

  const purchaseFeed = (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-3">
          <CardTitle>{t("admin_home_purchase_activity")}</CardTitle>
          <Badge variant="live">{t("admin_home_live")}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {notifications.isLoading ? (
          <div className="flex flex-col gap-4">
            {[...Array(3)].map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}
          </div>
        ) : notifications.isError ? (
          <div className="py-8 text-center text-ink-600">{t("notification_inbox_failed")}</div>
        ) : notifications.items.length === 0 ? (
          <div className="py-8 text-center text-ink-600">{t("notification_inbox_empty")}</div>
        ) : (
          <ul className="flex flex-col gap-4">
            {notifications.items.slice(0, 5).map((n) => (
              <li key={n.id} className="flex items-center gap-3">
                <span aria-hidden className="flex size-9 shrink-0 items-center justify-center rounded-full bg-brand-50 type-label text-brand-700">
                  {n.student_name.charAt(0).toUpperCase()}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="truncate type-label">{n.student_name}</div>
                  <div className="type-caption text-ink-600">{formatRelativeTime(n.created_at)}</div>
                </div>
                <span className="type-label tabular-nums">{formatRupiah(n.amount)}</span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );

  const quickActionsCard = (
    <Card>
      <CardHeader>
        <CardTitle>{t("admin_home_quick_actions")}</CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-2 gap-3">
        {quickActions.map((action) => (
          <Link
            key={action.href}
            href={action.href}
            className="flex flex-col items-center gap-2 rounded-[var(--radius-md)] bg-surface-sunken p-4 text-center transition-transform duration-150 hover:-translate-y-0.5"
          >
            <span className="flex size-10 items-center justify-center rounded-[var(--radius-md)] bg-brand-50 text-brand-700">
              <action.icon aria-hidden className="size-5" />
            </span>
            <span className="type-label-sm">{action.label}</span>
          </Link>
        ))}
      </CardContent>
    </Card>
  );

  return (
    <div className="fade-in flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="type-h1 text-ink-900">{`${greeting}, ${name.split(" ")[0]}`}</h1>
          <p className="type-body-sm text-ink-600">{`${today} · ${t("admin_home_overview_suffix")}`}</p>
        </div>
        <Button asChild>
          <Link href="/admin/exam/packages">
            <Plus aria-hidden />
            {t("admin_home_create_exam")}
          </Link>
        </Button>
      </header>

      {!dashboardError && (
        dashboardLoading ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            {[...Array(3)].map((_, i) => <Skeleton key={i} className="h-24 w-full" />)}
          </div>
        ) : (
          <div
            data-testid="attention-row"
            className={`grid grid-cols-1 gap-4 sm:grid-cols-3 ${a.shipment_failed > 0 ? "xl:grid-cols-4" : ""}`}
          >
            <AttentionCard
              testId="attention-needs-confirm"
              href="/admin/orders?status=pending"
              tone="gold"
              label={t("admin_home_needs_confirm")}
              value={a.needs_confirm}
              icon={Clock}
            />
            <AttentionCard
              testId="attention-ready-to-ship"
              href="/admin/orders?queue=ready_to_ship"
              tone="sky"
              label={t("admin_home_ready_to_ship")}
              value={a.ready_to_ship}
              icon={PackageCheck}
            />
            <AttentionCard
              testId="attention-essays"
              href={a.essays_to_grade > 0 && a.essays_exam_id ? `/admin/exam/packages/${a.essays_exam_id}?tab=grading` : undefined}
              tone="ember"
              label={t("admin_home_essays_to_grade")}
              value={a.essays_to_grade}
              icon={PenLine}
            />
            {a.shipment_failed > 0 && (
              <AttentionCard
                testId="attention-shipment-failed"
                href="/admin/orders?queue=shipment_failed"
                tone="danger"
                label={t("admin_home_shipment_failed")}
                value={a.shipment_failed}
                icon={AlertTriangle}
              />
            )}
          </div>
        )
      )}

      <PeriodBar
        preset={preset}
        range={range}
        onChange={handlePeriodChange}
        resolvedFrom={d.period.from || undefined}
        resolvedTo={d.period.to || undefined}
      />

      {dashboardError ? (
        <>
          <Card>
            <CardContent className="py-12 text-center text-ink-600">
              <p className="mb-4">{t("dash_load_failed")}</p>
              <Button variant="outline" onClick={() => refetchDashboard()}>
                {t("admin_home_reload")}
              </Button>
            </CardContent>
          </Card>
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
            <div className="lg:col-span-2">{purchaseFeed}</div>
            {quickActionsCard}
          </div>
        </>
      ) : dashboardLoading ? (
        <>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
            {[...Array(4)].map((_, i) => <Skeleton key={i} className="h-28 w-full" />)}
          </div>
          <Skeleton className="h-64 w-full" />
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <Skeleton className="h-64 w-full" />
            <Skeleton className="h-64 w-full" />
          </div>
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
            <Skeleton className="h-48 w-full lg:col-span-2" />
            <Skeleton className="h-48 w-full" />
          </div>
        </>
      ) : (
        <>
          <div data-testid="kpi-row" className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
            <KpiCard
              tone="brand"
              icon={Coins}
              label={t("admin_home_kpi_revenue")}
              value={formatRupiah(d.kpi.revenue.value)}
              delta={computeDelta(d.kpi.revenue.value, d.kpi.revenue.prev)}
            />
            <KpiCard
              tone="ember"
              icon={Receipt}
              label={t("admin_home_kpi_orders")}
              value={String(d.kpi.order_count.value)}
              delta={computeDelta(d.kpi.order_count.value, d.kpi.order_count.prev)}
            />
            <KpiCard
              tone="teal"
              icon={UserPlus}
              label={t("admin_home_kpi_new_students")}
              value={String(d.kpi.new_students.value)}
              delta={computeDelta(d.kpi.new_students.value, d.kpi.new_students.prev)}
            />
            <KpiCard
              tone="sky"
              icon={Store}
              label={t("admin_home_kpi_schools")}
              value={String(d.kpi.schools.value)}
              delta={computeDelta(d.kpi.schools.value, d.kpi.schools.prev)}
              sub={t("admin_home_kpi_students_total").replace("{count}", String(d.kpi.students_total.value))}
            />
          </div>

          <Card>
            <CardHeader>
              <CardTitle>{t("admin_home_chart_revenue_title")}</CardTitle>
              <CardDescription>{t("admin_home_chart_revenue_sub")}</CardDescription>
            </CardHeader>
            <CardContent>
              <AreaLineChart
                labels={labels}
                area={{ values: d.series.map((p) => p.revenue), color: CHART_BLUE, label: t("admin_home_series_revenue"), format: formatRupiah }}
                line={{ values: d.series.map((p) => p.order_count), color: CHART_ORANGE, label: t("admin_home_series_orders") }}
                emptyLabel={t("admin_home_chart_empty")}
              />
            </CardContent>
          </Card>

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>{t("admin_home_chart_students_title")}</CardTitle>
                <CardDescription>{t("admin_home_chart_students_sub")}</CardDescription>
              </CardHeader>
              <CardContent>
                <MultiLineChart
                  labels={labels}
                  series={[
                    { values: d.series.map((p) => p.new_students), color: CHART_PURPLE, label: t("admin_home_series_new") },
                    { values: d.series.map((p) => p.exam_students), color: CHART_TEAL, label: t("admin_home_series_exam") },
                    { values: d.series.map((p) => p.buying_students), color: CHART_AMBER, label: t("admin_home_series_buy") },
                  ]}
                  emptyLabel={t("admin_home_chart_empty")}
                />
              </CardContent>
            </Card>
            <RevenueMixCard digital={d.kpi.revenue_digital} physical={d.kpi.revenue_physical} />
          </div>

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
            <Card className="lg:col-span-2">
              <CardHeader>
                <div className="flex items-center justify-between gap-3">
                  <CardTitle>{t("admin_home_upcoming_exams")}</CardTitle>
                  {a.active_sessions > 0 && (
                    <Link href="/admin/exam/monitor">
                      <Badge variant="ember">
                        {t("admin_home_sessions_chip").replace("{count}", String(a.active_sessions))}
                      </Badge>
                    </Link>
                  )}
                </div>
              </CardHeader>
              <CardContent>
                {d.upcoming_exams.length === 0 ? (
                  <div className="py-12 text-center text-ink-600">{t("admin_home_upcoming_empty")}</div>
                ) : (
                  <div className="flex flex-col gap-2">
                    {d.upcoming_exams.map((exam) => (
                      <Link
                        key={exam.id}
                        href={`/admin/exam/packages/${exam.id}`}
                        className="flex items-center justify-between gap-3 rounded-[var(--radius-md)] bg-surface-sunken p-3 transition-colors hover:bg-brand-50"
                      >
                        <div className="min-w-0">
                          <div className="truncate type-label">{exam.title}</div>
                          <div className="type-caption text-ink-600">{jakarta.dateTime(exam.scheduled_at)}</div>
                        </div>
                        <div className="shrink-0 type-label-sm text-brand-700">
                          {t("admin_home_registrants").replace("{count}", String(exam.registrant_count))}
                        </div>
                      </Link>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
            {purchaseFeed}
          </div>

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
            <Card className="lg:col-span-2">
              <CardHeader>
                <div className="flex items-center justify-between gap-3">
                  <CardTitle>{t("admin_home_top_products")}</CardTitle>
                  <Button asChild variant="ghost" size="sm">
                    <Link href="/admin/store">{t("admin_home_sales_summary")}</Link>
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                {d.top_products.length === 0 ? (
                  <div className="py-12 text-center text-ink-600">{t("admin_home_chart_empty")}</div>
                ) : (
                  <ol className="flex flex-col gap-4">
                    {d.top_products.map((p, i) => (
                      <li key={p.product_id} data-testid="top-product" className="flex items-center gap-3">
                        <span className="w-5 shrink-0 type-label text-ink-600">{i + 1}</span>
                        <div className="min-w-0 flex-1">
                          <div className="truncate type-label">{p.name}</div>
                          <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-surface-sunken">
                            <div
                              data-testid="top-product-bar"
                              className="h-full rounded-full bg-brand-600"
                              style={{ width: `${(p.qty_sold / topMax) * 100}%` }}
                            />
                          </div>
                        </div>
                        <div className="shrink-0 text-right">
                          <div className="type-label tabular-nums">{t("admin_home_qty_sold").replace("{count}", String(p.qty_sold))}</div>
                          <div className="type-caption text-ink-600">{formatRupiah(p.product_revenue)}</div>
                        </div>
                      </li>
                    ))}
                  </ol>
                )}
              </CardContent>
            </Card>
            {quickActionsCard}
          </div>
        </>
      )}
    </div>
  );
}
