"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  AlertCircle,
  CalendarDays,
  Clock,
  Download,
  KeyRound,
  Play,
  Trophy,
  XCircle,
} from "lucide-react";
import { toast } from "sonner";

import { useCheckIn, useRegistrations, useStartSession } from "@/lib/hooks/exam";
import { useTranslation } from "@/lib/i18n";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import type { RegistrationListItem } from "@/lib/types";

function formatScheduled(scheduledAt: string | null): string {
  if (!scheduledAt) return "—";
  const d = new Date(scheduledAt);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat("id-ID", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

// "6 hari 7 jam" / "45 menit" style countdown, matching the mockup's PkgCard.
function formatCountdown(ms: number, lang: "id" | "en"): string {
  if (ms <= 0) return lang === "id" ? "Segera" : "Any moment";
  const totalMinutes = Math.floor(ms / 60_000);
  const days = Math.floor(totalMinutes / (60 * 24));
  const hours = Math.floor((totalMinutes % (60 * 24)) / 60);
  const minutes = totalMinutes % 60;
  const dayLabel = lang === "id" ? "hari" : "d";
  const hourLabel = lang === "id" ? "jam" : "h";
  const minLabel = lang === "id" ? "menit" : "m";
  if (days > 0) return `${days} ${dayLabel} ${hours} ${hourLabel}`;
  if (hours > 0) return `${hours} ${hourLabel} ${minutes} ${minLabel}`;
  return `${minutes} ${minLabel}`;
}

type CardState =
  | { kind: "start" }
  | { kind: "locked"; opensAt: Date }
  | { kind: "checkin" }
  | { kind: "in_progress" }
  | { kind: "expired" };

// Mirrors design-app-abak's PkgCard state machine (stateMeta: free/locked/
// checkin/checkedin/inprogress/expired), driven by real fields instead of
// mock data. "submitted" isn't derivable here — exam_registration.status
// never advances past 'in_progress' on submit (score lives on exam_session,
// which this list endpoint doesn't join) — see registration detail page for
// the same gap. in_progress renders as a link to the detail page instead of
// a fabricated "view result" action.
function computeCardState(reg: RegistrationListItem, now: number): CardState {
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
  const [token, setToken] = useState("");
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(id);
  }, []);

  const checkInMutation = useCheckIn();
  const startSessionMutation = useStartSession();

  const state = computeCardState(reg, now);

  const handleCheckIn = () => {
    if (!token.trim()) return;
    checkInMutation.mutate(
      { token: token.trim() },
      {
        onSuccess: () => {
          toast.success(t("checkin_success"));
          setToken("");
        },
        onError: (err) => {
          toast.error(err instanceof Error ? err.message : t("invalid_token"));
        },
      },
    );
  };

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

  return (
    <Card className="gap-0 p-[18px]">
      <div className="flex items-start justify-between gap-3">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-[10px] bg-brand-50 text-brand-600">
          <Trophy className="size-5" />
        </div>
        {reg.is_free ? (
          <Badge className="rounded-full">{t("free")}</Badge>
        ) : (
          <StateBadge state={state} />
        )}
      </div>

      <h3 className="mt-3 text-[15px] leading-snug font-semibold text-ink-900">
        {reg.exam_title}
      </h3>
      <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-500">
        {reg.duration_minutes != null && (
          <span className="inline-flex items-center gap-1">
            <Clock className="size-3" />
            {reg.duration_minutes} {t("minutes")}
          </span>
        )}
        {reg.scheduled_at && (
          <span className="inline-flex items-center gap-1">
            <CalendarDays className="size-3" />
            {formatScheduled(reg.scheduled_at)}
            {reg.scheduled_end_at && ` – ${formatScheduled(reg.scheduled_end_at)}`}
          </span>
        )}
      </div>

      <div className="mt-3.5 border-t border-line pt-3.5">
        {state.kind === "start" && (
          <div className="flex items-center gap-3">
            <div className="min-w-0 flex-1 text-xs text-ink-500">
              {t("attempts").replace("{n}", String(reg.attempts_used))}
            </div>
            <Button
              size="sm"
              className="shrink-0 rounded-full"
              onClick={handleStart}
              disabled={startSessionMutation.isPending}
            >
              <Play className="mr-1 size-3.5" />
              {startSessionMutation.isPending ? t("sys_loading") : t("start_exam")}
            </Button>
          </div>
        )}

        {state.kind === "locked" && (
          <div className="flex items-center gap-3">
            <div className="min-w-0 flex-1">
              <div className="text-xs text-ink-500">
                {t("checkin_opens")} {formatScheduled(state.opensAt.toISOString())}
              </div>
              <div className="mt-0.5 flex items-center gap-1 text-xs font-semibold text-info">
                <Clock className="size-3" />
                {formatCountdown(state.opensAt.getTime() - now, lang)}
              </div>
            </div>
            <Button
              size="sm"
              variant="outline"
              className="shrink-0 rounded-full"
              onClick={handleDownload}
            >
              <Download className="mr-1 size-3.5" />
              {t("download_card")}
            </Button>
          </div>
        )}

        {state.kind === "checkin" && (
          <div>
            <label className="text-xs font-semibold text-ink-600">{t("enter_token")}</label>
            <div className="mt-1.5 flex gap-2">
              <Input
                value={token}
                onChange={(e) => setToken(e.target.value.toUpperCase())}
                placeholder={t("token_placeholder")}
                className="font-mono uppercase"
              />
              <Button
                size="sm"
                className="shrink-0 rounded-full"
                onClick={handleCheckIn}
                disabled={!token.trim() || checkInMutation.isPending}
              >
                {checkInMutation.isPending ? t("sys_loading") : t("check_in")}
              </Button>
            </div>
            <div className="mt-1.5 flex items-center gap-1 text-xs text-ink-500">
              <KeyRound className="size-3" />
              {t("token_hint")}
            </div>
          </div>
        )}

        {state.kind === "in_progress" && (
          <Button asChild size="sm" variant="outline" className="w-full rounded-full">
            <Link href={`/exam/${reg.id}`}>{t("competition_view_detail")}</Link>
          </Button>
        )}

        {state.kind === "expired" && (
          <div className="flex items-center gap-1.5 text-xs text-ink-500">
            <XCircle className="size-3.5 text-danger" />
            {t("st_expired")}
          </div>
        )}
      </div>
    </Card>
  );
}

function StateBadge({ state }: { state: CardState }) {
  const { t } = useTranslation();
  const labelKey =
    state.kind === "locked"
      ? "st_locked"
      : state.kind === "checkin"
        ? "st_checkin"
        : state.kind === "in_progress"
          ? "st_inprogress"
          : state.kind === "expired"
            ? "st_expired"
            : "st_checkedin";
  return (
    <Badge variant="outline" className="rounded-full">
      {t(labelKey)}
    </Badge>
  );
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
