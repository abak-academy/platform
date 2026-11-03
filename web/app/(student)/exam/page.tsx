"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  AlertCircle,
  ArrowRight,
  CalendarDays,
  ClipboardList,
  Download,
  Lock,
  Play,
  Trophy,
} from "lucide-react";
import { toast } from "sonner";

import { useRegistrations, useStartSession } from "@/lib/hooks/exam";
import { accessWindow, countdownParts, workingMinutes } from "@/lib/exam-schedule";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { RegistrationListItem } from "@/lib/types";

type CardState =
  | { kind: "start" }
  | { kind: "locked"; opensAt: Date }
  | { kind: "checkin" }
  | { kind: "in_progress" }
  | { kind: "expired" }
  | { kind: "submitted"; sessionId: string; hasAttemptsLeft: boolean };

// Mirrors design-app-abak's PkgCard state machine (stateMeta: free/locked/
// checkin/checkedin/inprogress/expired/submitted), driven by real fields
// instead of mock data.
function computeCardState(reg: RegistrationListItem, now: number): CardState {
  if (reg.status === "submitted") {
    // session_id is nullable (list join picks the latest attempt) — without
    // it there's nowhere valid to link, so fall back to in_progress's detail link.
    if (reg.session_id) {
      // nil max_attempts is unlimited; 0 or 1 is a single sitting.
      const maxAttempts = reg.max_attempts;
      const unlimited = maxAttempts == null;
      const ceiling = maxAttempts != null && maxAttempts > 0 ? maxAttempts : 1;
      return {
        kind: "submitted",
        sessionId: reg.session_id,
        hasAttemptsLeft: unlimited || reg.attempts_used < ceiling,
      };
    }
    return { kind: "in_progress" };
  }
  if (reg.status === "in_progress") return { kind: "in_progress" };
  if (reg.status === "checked_in") return { kind: "start" };
  if (!reg.requires_checkin) {
    // Even without check-in, a set scheduled_end_at turns scheduled_at into an
    // availability window (backend gates StartSession on it regardless of
    // requires_checkin) — mirror that here so "Start" isn't shown outside it.
    if (reg.scheduled_end_at) {
      const start = reg.scheduled_at ? new Date(reg.scheduled_at).getTime() : null;
      const end = new Date(reg.scheduled_end_at).getTime();
      if (start !== null && now < start) return { kind: "locked", opensAt: new Date(start) };
      if (now > end) return { kind: "expired" };
    }
    return { kind: "start" };
  }
  if (!reg.scheduled_at) return { kind: "checkin" };
  const scheduledAt = new Date(reg.scheduled_at).getTime();
  const windowMin = reg.check_in_window_minutes ?? 15;
  const opensAt = scheduledAt - windowMin * 60_000;
  if (now < opensAt) return { kind: "locked", opensAt: new Date(opensAt) };
  // Without an availability window, check-in closes the instant scheduled_at
  // is reached (windowEnd collapses to scheduledAt) — with one, it stays open
  // through scheduled_end_at, matching the backend CheckIn windowing.
  const windowEnd = reg.scheduled_end_at ? new Date(reg.scheduled_end_at).getTime() : scheduledAt;
  if (now < windowEnd) return { kind: "checkin" };
  return { kind: "expired" };
}

export default function ExamPage() {
  const { t } = useTranslation();
  const { data, isLoading, isError, error, refetch } = useRegistrations();

  const free = data?.filter((r) => r.is_free) ?? [];
  const mine = data?.filter((r) => !r.is_free) ?? [];

  return (
    <div>
      <header className="mb-8">
        <h1 className="font-serif text-3xl font-bold text-ink-900 md:text-4xl">
          {t("comp_title")}
        </h1>
        <p className="mt-2 text-sm text-ink-500">{t("comp_sub")}</p>
      </header>

      {isError && (
        <Card className="mb-8 border-danger/30 bg-danger-bg px-5 py-4">
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
          <PkgSection title={t("my_packages")} items={mine} className="mt-9" />
        </>
      ) : (
        !isError && <EmptyExam />
      )}
    </div>
  );
}

function PkgSection({
  title,
  items,
  className,
}: {
  title: string;
  items: RegistrationListItem[];
  className?: string;
}) {
  if (items.length === 0) return null;
  return (
    <section className={className}>
      <h2 className="mb-3 text-[13px] font-bold tracking-wide text-ink-500 uppercase">
        {title}
      </h2>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
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
    <Card className="min-w-0 gap-0 p-5">
      <div className="flex items-center justify-between gap-3">
        <span className="flex items-center gap-1.5 text-[11px] font-bold tracking-[0.12em] whitespace-nowrap text-ink-500 uppercase">
          <ClipboardList className="size-4 text-brand-600" />
          {t("examcard_kicker")}
        </span>
        <StateBadge state={state} />
      </div>

      <h3 className="mt-3 text-lg leading-snug font-semibold [overflow-wrap:anywhere] text-ink-900">
        {reg.exam_title}
      </h3>

      <dl className="mt-4 grid grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)_minmax(0,1fr)] border-y border-line py-3 text-xs">
        <div className="min-w-0 pr-3">
          <dt className="text-ink-500">
            {reg.timer_mode === "per_test" ? t("examcard_time_total") : t("examcard_time_overall")}
          </dt>
          <dd className="mt-1 text-ink-900">
            {minutes != null ? (
              <>
                <span className="text-3xl leading-none font-bold">{minutes}</span>{" "}
                <span className="text-sm">{t("minutes")}</span>
              </>
            ) : (
              "—"
            )}
          </dd>
          <dd className="mt-1.5 text-ink-500">
            {reg.timer_mode === "per_test" ? t("examcard_timer_per_test") : t("examcard_timer_overall")}
          </dd>
        </div>
        <div className="min-w-0 border-l border-line px-3">
          <dt className="text-ink-500">{t("examcard_sections")}</dt>
          <dd className="mt-1 text-3xl leading-none font-bold text-ink-900">{reg.section_count}</dd>
        </div>
        <div className="min-w-0 border-l border-line pl-3">
          <dt className="text-ink-500">{t("examcard_questions")}</dt>
          <dd className="mt-1 text-3xl leading-none font-bold text-ink-900">{reg.question_count}</dd>
        </div>
      </dl>

      {access && (
        <div className="mt-3 flex items-center gap-3 rounded-lg border border-line bg-surface-2 px-3 py-2.5 text-xs">
          <CalendarDays className="size-8 shrink-0 text-brand-600" strokeWidth={1.5} />
          <div className="min-w-0 flex-1">
            <div className="text-ink-500">{t("examcard_access")}</div>
            {access.to && access.sameDay ? (
              <div className="mt-0.5 flex flex-wrap justify-between gap-x-3 text-sm font-semibold text-ink-900">
                <span>{access.from.date}</span>
                <span className="font-medium">
                  {access.from.time}–{access.to.time} WIB
                </span>
              </div>
            ) : (
              <div className="mt-0.5 text-sm font-semibold text-ink-900">
                <div>
                  {access.from.date}, {access.from.time} WIB
                </div>
                {access.to && (
                  <div>
                    {access.to.date}, {access.to.time} WIB
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      <div className="mt-3 flex items-start gap-3 text-xs">
        {sequential ? (
          <Lock className="mt-0.5 size-4 shrink-0 text-ink-500" />
        ) : (
          <ArrowRight className="mt-0.5 size-4 shrink-0 text-ink-500" />
        )}
        <div className="min-w-0">
          <div className="text-sm font-semibold text-ink-900">
            {sequential ? t("examcard_nav_sequential") : t("examcard_nav_free")}
          </div>
          <div className="text-ink-500">
            {sequential ? t("examcard_nav_sequential_hint") : t("examcard_nav_free_hint")}
          </div>
        </div>
      </div>

      <div className="mt-4 border-t border-line pt-4">
        <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-3">
          <div className="min-w-0 flex-1 basis-32 text-xs">
            <div className="text-sm font-semibold text-ink-900">{footerTitle}</div>
            {footerHint && <div className="mt-0.5 text-ink-500">{footerHint}</div>}
          </div>
          {resuming ? (
            <Button
              size="sm"
              className="shrink-0 rounded-full"
              onClick={handleStart}
              disabled={startSessionMutation.isPending}
            >
              <Play className="size-3.5" />
              {startSessionMutation.isPending ? t("sys_loading") : t("continue_exam")}
            </Button>
          ) : state.kind === "submitted" ? (
            <Button asChild size="sm" className="shrink-0 rounded-full">
              <Link href={`/exam/sessions/${state.sessionId}/result`}>{t("view_exam_result")}</Link>
            </Button>
          ) : (
            <Button asChild size="sm" className="shrink-0 rounded-full">
              <Link href={detailHref}>{detailLabel}</Link>
            </Button>
          )}
        </div>

        {/* FR-45: the card must stay reachable regardless of state, so a detail link
            is always rendered, as the primary CTA or as a secondary action. */}
        {(!detailIsPrimary || state.kind === "locked") && (
          <div className="mt-2.5 flex flex-wrap justify-end gap-2">
            {state.kind === "locked" && (
              <Button size="sm" variant="outline" className="rounded-full" onClick={handleDownload}>
                <Download className="size-3.5" />
                {t("download_card")}
              </Button>
            )}
            {state.kind === "submitted" && state.hasAttemptsLeft && (
              <Button
                size="sm"
                variant="outline"
                className="rounded-full"
                onClick={handleStart}
                disabled={startSessionMutation.isPending}
              >
                {startSessionMutation.isPending ? t("sys_loading") : t("retake_exam")}
              </Button>
            )}
            {!detailIsPrimary && (
              <Button asChild size="sm" variant="outline" className="rounded-full">
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
  return t("examcard_opens_in").replace("{time}", t(`examcard_countdown_${parts.unit}`).replace("{n}", String(parts.n)));
}

const BADGE_TONE = {
  success: "border-transparent bg-success-bg text-success",
  brand: "border-transparent bg-brand-50 text-brand-700",
  neutral: "border-line bg-surface-2 text-ink-700",
  warn: "border-transparent bg-warn-bg text-warn",
} as const;

function StateBadge({ state }: { state: CardState }) {
  const { t } = useTranslation();
  const [labelKey, tone]: [I18nKey, keyof typeof BADGE_TONE] =
    state.kind === "locked"
      ? ["examcard_upcoming_badge", "neutral"]
      : state.kind === "checkin"
        ? ["st_checkin", "brand"]
        : state.kind === "in_progress"
          ? ["st_inprogress", "brand"]
          : state.kind === "expired"
            ? ["st_expired", "warn"]
            : state.kind === "submitted"
              ? ["examcard_status_done", "success"]
              : ["examcard_status_ready", "success"];
  return <Badge className={`rounded-full ${BADGE_TONE[tone]}`}>{t(labelKey)}</Badge>;
}

function EmptyExam() {
  const { t } = useTranslation();
  return (
    <Card className="flex flex-col items-center justify-center gap-3 border-dashed border-line bg-surface-2 px-6 py-10 text-center">
      <div className="flex size-12 items-center justify-center rounded-full bg-brand-50 text-brand-600">
        <Trophy className="size-6" />
      </div>
      <p className="font-semibold text-ink-900">{t("competition_empty")}</p>
    </Card>
  );
}

function ExamSkeleton() {
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Skeleton className="h-9 w-48" />
        <Skeleton className="h-4 w-72" />
      </div>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-44 w-full rounded-lg" />
        ))}
      </div>
    </div>
  );
}
