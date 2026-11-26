"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  AlertCircle,
  ArrowRight,
  CalendarDays,
  Download,
  Lock,
  Play,
  Plus,
  Trophy,
} from "lucide-react";
import { toast } from "sonner";

import { useRegistrations, useStartSession } from "@/lib/hooks/exam";
import { computeCardState } from "@/lib/exam-card-state";
import { accessWindow, countdownParts, workingMinutes } from "@/lib/exam-schedule";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHead } from "@/components/shell/PageHead";
import { Skeleton } from "@/components/ui/skeleton";
import type { CardState } from "@/lib/exam-card-state";
import type { RegistrationListItem } from "@/lib/types";

export default function ExamPage() {
  const { t } = useTranslation();
  const { data, isLoading, isError, error, refetch } = useRegistrations();

  const free = data?.filter((r) => r.is_free) ?? [];
  const mine = data?.filter((r) => !r.is_free) ?? [];

  return (
    <>
      <PageHead
        title={t("comp_title")}
        sub={t("comp_sub")}
        actions={
          <Button asChild variant="dark">
            <Link href="/event">
              <Plus />
              {t("stu_home_events_browse")}
            </Link>
          </Button>
        }
      />

      {isError && (
        <Card className="gap-0 border-danger/30 bg-danger-bg px-5 py-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="size-5 text-danger" />
            <div className="flex-1 text-sm text-ink-700">
              {t("competition_error")}
              {error instanceof Error && error.message ? ` ${error.message}` : ""}
            </div>
            <Button variant="outline" size="sm" className="rounded-full" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        </Card>
      )}

      {isLoading ? (
        <ExamSkeleton />
      ) : data && data.length > 0 ? (
        <>
          <PkgSection title={t("free_packages")} items={free} />
          <PkgSection title={t("my_packages")} items={mine} />
        </>
      ) : (
        !isError && <EmptyExam />
      )}
    </>
  );
}

function PkgSection({ title, items }: { title: string; items: RegistrationListItem[] }) {
  if (items.length === 0) return null;
  return (
    <section>
      <div className="mb-3.5 flex flex-wrap items-center justify-between gap-3">
        <h2 className="font-mono text-xs font-semibold tracking-[0.14em] text-ink-600 uppercase">{title}</h2>
        <Badge variant="mute">{items.length}</Badge>
      </div>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(310px,1fr))] gap-4 max-[360px]:grid-cols-1">
        {items.map((reg) => (
          <PkgCard key={reg.id} reg={reg} />
        ))}
      </div>
    </section>
  );
}

function PkgCard({ reg }: { reg: RegistrationListItem }) {
  const { t, lang } = useTranslation();
  const router = useRouter();
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(id);
  }, []);

  const startSessionMutation = useStartSession();

  const state = computeCardState(reg, now);
  const minutes = workingMinutes(reg);
  const sequential = reg.navigation_policy === "sequential";
  const access = accessWindow(reg.scheduled_at, reg.scheduled_end_at, lang);
  const attempts =
    reg.max_attempts == null
      ? t("examdetail_attempts_unlimited")
      : t("examdetail_attempts_max").replace("{n}", String(Math.max(1, reg.max_attempts)));

  const handleStart = async () => {
    try {
      const result = await startSessionMutation.mutateAsync(reg.id);
      router.push(`/exam/sessions/${result.session_id}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("competition_error"));
    }
  };

  const handleDownload = () => {
    router.push(`/exam/${reg.id}/card`);
  };

  const detailHref = `/exam/${reg.id}`;
  const resuming = state.kind === "in_progress" && reg.status === "in_progress";
  const detailIsPrimary =
    (state.kind === "submitted" && !state.sessionId) ||
    state.kind === "start" ||
    state.kind === "checkin" ||
    state.kind === "locked" ||
    state.kind === "expired" ||
    (state.kind === "in_progress" && !resuming);

  let footerTitle = attempts;
  let footerHint: string | null = null;
  if (state.kind === "start" || state.kind === "checkin") footerHint = t("examcard_hint_ready");
  else if (state.kind === "locked") {
    footerTitle = formatOpensIn(state.opensAt.getTime() - now, t);
    footerHint = attempts;
  } else if (state.kind === "in_progress") {
    footerTitle = t("examcard_progress_title");
    footerHint = t("examcard_progress_hint");
  } else if (state.kind === "submitted") {
    footerTitle = t("examcard_done_title");
    footerHint = t("examcard_done_hint");
  } else if (state.kind === "expired") {
    footerTitle = t("st_expired");
    footerHint = attempts;
  }

  const detailLabel =
    state.kind === "start" || state.kind === "checkin" ? (
      <>
        {t("examcard_rules_cta")} <ArrowRight className="size-3.5" />
      </>
    ) : (
      <>
        {t("examcard_detail_cta")} <ArrowRight className="size-3.5" />
      </>
    );

  return (
    <Card className="min-w-0 gap-0 overflow-hidden py-0 transition-transform duration-200 hover:-translate-y-1 motion-reduce:hover:translate-y-0">
      <div className="flex min-h-[58px] items-center justify-between gap-3 bg-[linear-gradient(110deg,var(--color-brand-600),var(--color-brand-400))] px-4 py-3.5 text-white">
        <span className="font-mono text-[11px] font-semibold tracking-[0.12em] whitespace-nowrap uppercase opacity-90">
          {t("examcard_kicker")}
        </span>
        <StateBadge state={state} />
      </div>

      <div className="flex flex-col gap-3 p-4">
        <h3 className="text-[17px] leading-snug font-bold [overflow-wrap:anywhere] text-ink-900">
          {reg.exam_title}
        </h3>

        <dl className="grid grid-cols-3 gap-2">
          <div className="min-w-0 rounded-[14px] bg-surface-sunken p-2.5">
            <dd className="[font-family:var(--font-display)] text-lg leading-tight font-extrabold text-ink-900">
              {minutes != null ? (
                <>
                  <span>{minutes}</span> <span className="text-xs font-bold">{t("minutes")}</span>
                </>
              ) : (
                "—"
              )}
            </dd>
            <dt className="text-[11px] text-ink-600">
              {reg.timer_mode === "per_test" ? t("examcard_time_total") : t("examcard_time_overall")}
            </dt>
            <dd className="text-[11px] text-ink-600">
              {reg.timer_mode === "per_test" ? t("examcard_timer_per_test") : t("examcard_timer_overall")}
            </dd>
          </div>
          <div className="min-w-0 rounded-[14px] bg-surface-sunken p-2.5">
            <dd className="[font-family:var(--font-display)] text-lg leading-tight font-extrabold text-ink-900">{reg.section_count}</dd>
            <dt className="text-[11px] text-ink-600">{t("examcard_sections")}</dt>
          </div>
          <div className="min-w-0 rounded-[14px] bg-surface-sunken p-2.5">
            <dd className="[font-family:var(--font-display)] text-lg leading-tight font-extrabold text-ink-900">{reg.question_count}</dd>
            <dt className="text-[11px] text-ink-600">{t("examcard_questions")}</dt>
          </div>
        </dl>

        {access && (
          <div className="flex items-start gap-2 text-[13px] font-semibold text-ink-900">
            <CalendarDays className="mt-0.5 size-4 shrink-0 text-ink-600" aria-hidden />
            <div className="min-w-0 flex-1">
              <span className="mr-1.5 font-mono text-[10px] font-semibold tracking-[0.1em] text-ink-600 uppercase">
                {t("examcard_access")}
              </span>
              {access.to && access.sameDay ? (
                <span className="flex flex-wrap justify-between gap-x-3">
                  <span>{access.from.date}</span>
                  <span className="font-medium text-ink-600">
                    {access.from.time}–{access.to.time} WIB
                  </span>
                </span>
              ) : (
                <span className="block">
                  <span className="block">
                    {access.from.date}, {access.from.time} WIB
                  </span>
                  {access.to && (
                    <span className="block">
                      {access.to.date}, {access.to.time} WIB
                    </span>
                  )}
                </span>
              )}
            </div>
          </div>
        )}

        <div className="flex items-start gap-2 text-[13px] text-ink-600">
          {sequential ? (
            <Lock className="mt-0.5 size-4 shrink-0" aria-hidden />
          ) : (
            <ArrowRight className="mt-0.5 size-4 shrink-0" aria-hidden />
          )}
          <div className="min-w-0">
            <span className="font-semibold text-ink-900">
              {sequential ? t("examcard_nav_sequential") : t("examcard_nav_free")}
            </span>{" "}
            {sequential ? t("examcard_nav_sequential_hint") : t("examcard_nav_free_hint")}
          </div>
        </div>
      </div>

      <div className="border-t border-dashed border-line px-4 py-3">
        <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-3">
          <div className="min-w-0 flex-1 basis-32">
            <div className="text-[13px] font-bold text-ink-900">{footerTitle}</div>
            {footerHint && <div className="text-xs text-ink-600">{footerHint}</div>}
          </div>
          {resuming ? (
            <Button data-cta="main" className="shrink-0" onClick={handleStart} disabled={startSessionMutation.isPending}>
              <Play className="size-3.5" />
              {startSessionMutation.isPending ? t("sys_loading") : t("continue_exam")}
            </Button>
          ) : state.kind === "submitted" && state.sessionId ? (
            <Button asChild data-cta="main" variant="secondary" className="shrink-0">
              <Link href={`/exam/sessions/${state.sessionId}/result`}>{t("view_exam_result")}</Link>
            </Button>
          ) : (
            <Button
              asChild
              data-cta="main"
              variant={state.kind === "start" || state.kind === "checkin" ? "default" : "outline"}
              className="shrink-0"
            >
              <Link href={detailHref}>{detailLabel}</Link>
            </Button>
          )}
        </div>

        {/* FR-45: the card must stay reachable regardless of state, so a detail link
            is always rendered, as the primary CTA or as a secondary action. */}
        {(!detailIsPrimary || state.kind === "locked") && (
          <div className="mt-2.5 flex flex-wrap justify-end gap-2">
            {state.kind === "locked" && (
              <Button size="sm" variant="outline" onClick={handleDownload}>
                <Download className="size-3.5" />
                {t("download_card")}
              </Button>
            )}
            {state.kind === "submitted" && state.hasAttemptsLeft && (
              <Button
                size="sm"
                variant="outline"
                onClick={handleStart}
                disabled={startSessionMutation.isPending}
              >
                {startSessionMutation.isPending ? t("sys_loading") : t("retake_exam")}
              </Button>
            )}
            {!detailIsPrimary && (
              <Button asChild size="sm" variant="outline">
                <Link href={detailHref}>{t("competition_view_detail")}</Link>
              </Button>
            )}
          </div>
        )}
      </div>
    </Card>
  );
}

function formatOpensIn(ms: number, t: (key: I18nKey) => string): string {
  const parts = countdownParts(ms);
  if (!parts) return t("examcard_opens_soon");
  const time = parts
    .map(({ unit, n }) => t(`examcard_countdown_${unit}${n === 1 ? "_one" : ""}`).replace("{n}", String(n)))
    .join(" ");
  return t("examcard_opens_in").replace("{time}", time);
}

const CHIP_ON_BAND = "bg-white/90 text-ink-900";

function StateBadge({ state }: { state: CardState }) {
  const { t } = useTranslation();
  const labelKey: I18nKey =
    state.kind === "locked"
      ? "examcard_upcoming_badge"
      : state.kind === "checkin"
        ? "st_checkin"
        : state.kind === "in_progress"
          ? "st_inprogress"
          : state.kind === "expired"
            ? "st_expired"
            : state.kind === "submitted"
              ? "examcard_status_done"
              : "examcard_status_ready";
  const live = state.kind === "in_progress";
  return (
    <Badge variant={live ? "live" : "default"} className={live ? undefined : CHIP_ON_BAND}>
      {t(labelKey)}
    </Badge>
  );
}

function EmptyExam() {
  const { t } = useTranslation();
  return <EmptyState framed icon={Trophy} title={t("competition_empty")} />;
}

function ExamSkeleton() {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(310px,1fr))] gap-4">
      {Array.from({ length: 3 }).map((_, i) => (
        <Skeleton key={i} className="h-72 w-full rounded-[22px]" />
      ))}
    </div>
  );
}
