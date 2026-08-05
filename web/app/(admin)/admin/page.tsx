"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { Shield, DollarSign, Users, Eye, Store, Clipboard, BarChart3, UserPlus } from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/lib/hooks/auth";
import { useAdminAuditLog } from "@/lib/hooks/admin-audit";
import { useAdminRevenue } from "@/lib/hooks/admin-revenue";
import { useSchools } from "@/lib/hooks/students";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { adminHomeForRole } from "@/lib/auth-redirect";
import { NoAccess } from "@/components/admin/NoAccess";
import { StatCard } from "@/components/admin/StatCard";
import { Skeleton } from "@/components/ui/skeleton";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { UserRole } from "@/lib/nav-config";
import type { AuditLogEntry } from "@/lib/types";

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

export default function AdminIndexPage() {
  const router = useRouter();
  const { t } = useTranslation();
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

  const { data: auditEntries = [], isLoading: auditLoading, isError: auditError, refetch: refetchAudit } = useAdminAuditLog();
  const { data: revenue, isLoading: revenueLoading } = useAdminRevenue();
  const { data: schools, isLoading: schoolsLoading } = useSchools();
  const canReadRevenue = useHasCapability("revenue:read");

  useEffect(() => {
    if (!role) return;
    if (role !== "super_admin") router.replace(adminHomeForRole(role));
  }, [role, router]);

  if (role !== "super_admin") return null;
  if (!canReadRevenue) return <NoAccess />;

  const schoolCount = schools?.length ?? null;
  const schoolCountStr = schoolCount !== null ? String(schoolCount) : "—";
  const totalStudents = schools?.reduce((sum, s) => sum + (s.student_count ?? 0), 0) ?? null;
  const totalStudentsStr = totalStudents !== null ? String(totalStudents) : "—";

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

      {/* Stat grid */}
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-4 mb-8">
        {revenueLoading ? (
          <Skeleton className="h-28 w-full" />
        ) : (
          <StatCard
            label={t("admin_home_stat_revenue")}
            value={revenue ? formatRupiah(revenue.total) : "—"}
            accent="primary"
            icon={DollarSign}
          />
        )}
        {schoolsLoading ? (
          <Skeleton className="h-28 w-full" />
        ) : (
          <StatCard
            label={t("admin_home_stat_students")}
            value={totalStudentsStr}
            accent="secondary"
            icon={Users}
          />
        )}
        <StatCard
          label={t("admin_home_stat_exam_sessions")}
          value="—"
          trend={t("admin_home_stat_unavailable")}
          accent="error"
          icon={Eye}
        />
        {schoolsLoading ? (
          <Skeleton className="h-28 w-full" />
        ) : (
          <StatCard
            label={t("admin_home_stat_schools")}
            value={schoolCountStr}
            trend={schoolCount !== null ? t("admin_home_partner_trend") : undefined}
            accent="tertiary"
            icon={Store}
          />
        )}
      </div>

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
          <div className="grid gap-3">
            {quickActions.map((action, i) => (
              <button
                key={i}
                type="button"
                onClick={() => router.push(action.route)}
                className="flex items-center gap-3 p-3 rounded-[12px] border-none w-full text-left"
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
                <span className="text-body" style={{ fontWeight: 500 }}>
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
