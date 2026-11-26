"use client";

import {
  Activity, Library, ClipboardList, Calendar, BookOpen, AlertTriangle, ShieldAlert,
} from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/lib/hooks/auth";
import { useExamDashboard } from "@/lib/hooks/admin-dashboard";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { NoAccess } from "@/components/admin/NoAccess";
import { MonitorCard } from "@/components/admin/MonitorCard";
import { DashboardHero } from "@/components/admin/DashboardHero";
import { QuickActionTiles, type QuickAction } from "@/components/admin/QuickActionTiles";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useTranslation } from "@/lib/i18n";
import type { UserRole } from "@/lib/nav-config";

export default function ExamDashboardPage() {
  const { t, lang } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const storeRole = user?.role as UserRole | undefined;
  const me = useMe({ enabled: !storeRole });
  const name = user?.name ?? me.data?.name ?? t("exam_home_default_name");
  const canRead = useHasCapability("sessions:read");

  const { data, isLoading, isError, refetch } = useExamDashboard();

  const quickActions: QuickAction[] = [
    { icon: Library, label: t("exam_action_new_question"), href: "/admin/exam/questions" },
    { icon: ClipboardList, label: t("exam_action_new_test"), href: "/admin/exam/tests" },
    { icon: Calendar, label: t("exam_action_new_package"), href: "/admin/exam/packages" },
    { icon: Activity, label: t("exam_action_monitor"), href: "/admin/exam/monitor" },
  ];

  if (!canRead) return <NoAccess />;

  const formatDateTime = (iso: string) =>
    new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", {
      day: "numeric", month: "short", hour: "2-digit", minute: "2-digit", timeZone: "Asia/Jakarta",
    }).format(new Date(iso));

  return (
    <div className="fade-in flex flex-col gap-5">
      <DashboardHero
        icon={ShieldAlert}
        badge={t("exam_home_badge")}
        name={name}
        subtitle={t("exam_home_subtitle")}
      />

      {isError ? (
        <div className="md-card-outlined">
          <div className="py-12 text-center text-ink-600">
            <p className="mb-4">{t("dash_load_failed")}</p>
            <Button variant="outline" onClick={() => refetch()}>
              {t("admin_home_reload")}
            </Button>
          </div>
        </div>
      ) : isLoading ? (
        <div className="grid grid-cols-2 gap-3 min-[1200px]:grid-cols-5">
          {[...Array(5)].map((_, i) => <Skeleton key={i} className="h-28 w-full" />)}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 min-[1200px]:grid-cols-5">
            <MonitorCard
              testId="exam-active-sessions"
              href="/admin/exam/monitor"
              label={t("exam_home_active_sessions")}
              value={data?.active_sessions ?? 0}
              icon={Activity}
              tone={(data?.active_sessions ?? 0) > 0 ? "ember" : "teal"}
            />
            <MonitorCard
              testId="exam-questions"
              href="/admin/exam/questions"
              label={t("exam_home_questions")}
              value={data?.counts.questions ?? 0}
              icon={Library}
              tone="brand"
            />
            <MonitorCard
              testId="exam-tests"
              href="/admin/exam/tests"
              label={t("exam_home_tests")}
              value={data?.counts.tests ?? 0}
              icon={ClipboardList}
              tone="sky"
            />
            <MonitorCard
              testId="exam-exams"
              href="/admin/exam/packages"
              label={t("exam_home_exams")}
              value={data?.counts.exams ?? 0}
              icon={Calendar}
              tone="gold"
            />
            {/* admin_exam also holds products(course):* and sections:* in rbac.go */}
            <MonitorCard
              testId="exam-courses"
              href="/admin/courses"
              label={t("exam_home_courses")}
              value={data?.counts.courses ?? 0}
              icon={BookOpen}
              tone="berry"
            />
          </div>

          <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
            <section className="md-card-outlined">
              <div className="mb-3.5 flex items-center justify-between gap-3">
                <h2 className="type-h2">{t("admin_home_upcoming_exams")}</h2>
              </div>
              {!data?.upcoming_exams.length ? (
                <div className="py-10 text-center text-ink-600">{t("admin_home_upcoming_empty")}</div>
              ) : (
                <div className="flex flex-col">
                  {data.upcoming_exams.map((exam) => (
                    <div key={exam.id} className="flex items-center justify-between gap-3 rounded-2xl p-2.5 hover:bg-brand-50">
                      <div className="min-w-0">
                        <b className="block font-[family-name:var(--font-display)] text-sm font-bold text-ink-900">{exam.title}</b>
                        <small className="text-xs text-ink-600">{formatDateTime(exam.scheduled_at)}</small>
                      </div>
                      <div className="text-right text-xs font-bold text-ink-700">
                        {t("admin_home_registrants").replace("{count}", String(exam.registrant_count))}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>

            <section className="md-card-outlined">
              <div className="mb-3.5 flex items-center justify-between gap-3">
                <h2 className="type-h2">{t("exam_home_violations")}</h2>
              </div>
              {!data?.recent_violations.length ? (
                <div className="py-10 text-center text-ink-600">{t("exam_home_violations_empty")}</div>
              ) : (
                <div className="flex flex-col">
                  {data.recent_violations.map((v) => (
                    <div key={v.session_id + v.occurred_at} className="flex items-center gap-3 rounded-2xl p-2.5 hover:bg-brand-50">
                      <div className="grid size-[38px] shrink-0 place-items-center rounded-xl bg-danger-50 text-danger-600">
                        <AlertTriangle size={16} />
                      </div>
                      <div className="min-w-0">
                        <b className="block font-[family-name:var(--font-display)] text-sm font-bold text-ink-900">{v.student_name}</b>
                        <small className="text-xs text-ink-600">
                          {v.exam_title} · {v.violation_type} · {formatDateTime(v.occurred_at)}
                        </small>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </div>
        </>
      )}

      <QuickActionTiles title={t("admin_home_quick_actions")} actions={quickActions} />
    </div>
  );
}
