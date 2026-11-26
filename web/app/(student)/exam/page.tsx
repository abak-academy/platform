"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  AlertCircle,
  ArrowRight,
  CalendarDays,
  Download,
  LayoutGrid,
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
import { Spot } from "@/components/dashboard/Spot";
import { cn } from "@/lib/utils";
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
          <Button asChild variant="secondary" className={cn(BTN, FILL)}>
            <Link href="/event">
              <Plus />
              {t("stu_home_events_browse")}
            </Link>
          </Button>
        }
      />

      {isError && (
        <Card className="gap-0 border-danger/30 bg-danger-bg p-4">
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
        <h2 className="font-mono text-xs leading-[normal] font-semibold tracking-[0.14em] text-ink-600 uppercase">{title}</h2>
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
    <Card className="min-w-0 gap-0 overflow-hidden py-0 transition-transform duration-300 ease-spring hover:-translate-y-1 motion-reduce:hover:translate-y-0">
      <div className="relative flex min-h-[58px] items-center justify-between gap-2 bg-[linear-gradient(110deg,#3d4ddb,#6e7cee)] py-3.5 pr-4 pl-[70px] text-white">
        <Spot className="absolute top-1 left-2.5 size-[52px] drop-shadow-[0_6px_8px_rgba(0,0,0,.2)]" />
        <span className="font-mono text-[11px] leading-[1.4] font-semibold tracking-[0.12em] whitespace-nowrap uppercase opacity-[.92]">
          {t("examcard_kicker")}
        </span>
        <StateBadge state={state} />
      </div>

      <div className="flex flex-1 flex-col gap-3 p-4">
        <h3 className="font-[family-name:var(--font-display)] text-[17px] leading-[1.35] font-bold tracking-[-0.02em] [overflow-wrap:anywhere]">
          <Link href={detailHref} className="text-ink-900">
            {reg.exam_title}
          </Link>
        </h3>

        <dl className="grid grid-cols-3 gap-2">
          <div className="min-w-0 rounded-[14px] bg-surface-sunken p-2.5">
            <dd className="font-[family-name:var(--font-display)] text-lg leading-[normal] font-extrabold text-ink-900">
              {minutes != null ? `${minutes}′` : "—"}
            </dd>
            <dt className="text-[11px] leading-[23.25px] text-ink-600">
              {reg.timer_mode === "per_test" ? t("examcard_timer_per_test") : t("examcard_timer_overall")}
            </dt>
          </div>
          <div className="min-w-0 rounded-[14px] bg-surface-sunken p-2.5">
            <dd className="font-[family-name:var(--font-display)] text-lg leading-[normal] font-extrabold text-ink-900">{reg.section_count}</dd>
            <dt className="text-[11px] leading-[23.25px] text-ink-600">{t("examcard_sections")}</dt>
          </div>
          <div className="min-w-0 rounded-[14px] bg-surface-sunken p-2.5">
            <dd className="font-[family-name:var(--font-display)] text-lg leading-[normal] font-extrabold text-ink-900">{reg.question_count}</dd>
            <dt className="text-[11px] leading-[23.25px] text-ink-600">{t("examcard_questions")}</dt>
          </div>
        </dl>

        {access && (
          <div className="flex flex-wrap items-center gap-2 text-[13px] font-semibold text-ink-900">
            <CalendarDays className="size-4 shrink-0" aria-hidden />
            <span className="font-mono text-[10px] leading-[1.4] font-semibold tracking-[0.12em] text-ink-600 uppercase">
              {t("examcard_access")}
            </span>
            {access.to && access.sameDay ? (
              <>
                <span>{access.from.date}</span>
                <span className="ml-auto text-ink-600">
                  {access.from.time}–{access.to.time} WIB
                </span>
              </>
            ) : (
              <>
                <span>
                  {access.from.date}, {access.from.time} WIB
                </span>
                {access.to && (
                  <span className="ml-auto text-ink-600">
                    {access.to.date}, {access.to.time} WIB
                  </span>
                )}
              </>
            )}
          </div>
        )}

        <p className="flex items-center gap-1.5 text-[13px] text-ink-600">
          {sequential ? (
            <ArrowRight className="size-4 shrink-0" aria-hidden />
          ) : (
            <LayoutGrid className="size-4 shrink-0" aria-hidden />
          )}
          <span className="min-w-0">
            {sequential ? t("examcard_nav_sequential") : t("examcard_nav_free")} —{" "}
            {sequential ? t("examcard_nav_sequential_hint") : t("examcard_nav_free_hint")}
          </span>
        </p>
      </div>

      <div className="border-t border-dashed border-line px-4 py-3">
        <div className="flex flex-wrap items-center gap-2">
          <div className="min-w-0 flex-1 basis-32">
            <b className="block font-[family-name:var(--font-display)] text-[13px] leading-[normal] font-bold text-ink-900">{footerTitle}</b>
            {footerHint && <small className="text-xs leading-[1.55] text-ink-600">{footerHint}</small>}
          </div>
          {resuming ? (
            <Button data-cta="main" className={cn("shrink-0", BTN, FILL)} onClick={handleStart} disabled={startSessionMutation.isPending}>
              <Play className="size-3.5" />
              {startSessionMutation.isPending ? t("sys_loading") : t("continue_exam")}
            </Button>
          ) : state.kind === "submitted" && state.sessionId ? (
            <Button asChild data-cta="main" variant="secondary" className={cn("shrink-0", BTN, FILL)}>
              <Link href={`/exam/sessions/${state.sessionId}/result`}>{t("view_exam_result")}</Link>
            </Button>
          ) : (
            <Button
              asChild
              data-cta="main"
              variant={state.kind === "start" || state.kind === "checkin" ? "default" : "outline"}
              className={cn("shrink-0", BTN, state.kind === "start" || state.kind === "checkin" ? FILL : undefined)}
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
              <Button size="sm" variant="outline" className={SM} onClick={handleDownload}>
                <Download className="size-3.5" />
                {t("download_card")}
              </Button>
            )}
            {state.kind === "submitted" && state.hasAttemptsLeft && (
              <Button
                size="sm"
                variant="outline"
                className={SM}
                onClick={handleStart}
                disabled={startSessionMutation.isPending}
              >
                {startSessionMutation.isPending ? t("sys_loading") : t("retake_exam")}
              </Button>
            )}
            {!detailIsPrimary && (
              <Button asChild size="sm" variant="outline" className={SM}>
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

const CHIP_ON_BAND = "bg-white/92 text-[#14202b]";
// Prototype .ab-btn: 18px (13px sm) side padding with or without an icon, 1.5px transparent border.
const BTN = "px-[18px] has-[>svg]:px-[18px]";
const FILL = "border-[1.5px] border-transparent";
const SM = "px-[13px] has-[>svg]:px-[13px]";

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
  return (
    <Card className="gap-0 p-6">
      <EmptyState
        icon={Trophy}
        title={t("competition_empty")}
        text={t("comp_empty_text")}
        action={
          <Button asChild className={cn(BTN, FILL)}>
            <Link href="/event">{t("comp_empty_cta")}</Link>
          </Button>
        }
      />
    </Card>
  );
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
