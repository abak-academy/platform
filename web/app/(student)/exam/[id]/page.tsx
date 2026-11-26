"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { AlertCircle, Download, Eye, EyeOff } from "lucide-react";
import { toast } from "sonner";

import {
  useCheckIn,
  useRegistration,
  useStartSession,
} from "@/lib/hooks/exam";
import { ApiError } from "@/lib/api";
import { computeCardState } from "@/lib/exam-card-state";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ExamBriefing } from "@/components/exam/ExamBriefing";

export default function ExamDetailPage() {
  const { t } = useTranslation();
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const id = params?.id ?? "";

  const { data: reg, isLoading, isError, error, refetch } = useRegistration(id);

  const [showToken, setShowToken] = useState(false);

  // ── Check-in state (FR27) ───────────────────────────────────────────────
  const [token, setToken] = useState("");
  const checkInMutation = useCheckIn();

  const handleCheckIn = () => {
    if (!id || !token.trim()) return;
    checkInMutation.mutate(
      { token: token.trim() },
      {
        onSuccess: () => {
          toast.success(t("checkin_success"));
          setToken("");
        },
        onError: (err) => {
          const msg =
            err instanceof ApiError
              ? err.message
              : err instanceof Error
                ? err.message
                : t("invalid_token");
          toast.error(msg);
        },
      },
    );
  };

  // ── Start gate state (FR28) ─────────────────────────────────────────────
  const startSessionMutation = useStartSession();

  const handleStart = async () => {
    if (!reg) return;
    try {
      const result = await startSessionMutation.mutateAsync(reg.id);
      router.push(`/exam/sessions/${result.session_id}`);
    } catch (err) {
      if (err instanceof ApiError && err.code === "exam_not_ready") {
        toast.error(t("exam_not_ready_start"));
        return;
      }
      const msg =
        err instanceof Error ? err.message : t("competition_error");
      toast.error(msg);
    }
  };

  // ── Card handler — open the printable card page ──────────────────────────
  const handleDownload = () => {
    if (!id) return;
    router.push(`/exam/${id}/card`);
  };

  // ── Window / gate computation ────────────────────────────────────────────
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 1_000);
    return () => clearInterval(id);
  }, []);

  const scheduledAt = reg?.exam.scheduled_at
    ? new Date(reg.exam.scheduled_at)
    : null;
  const windowMin = reg?.exam.check_in_window_minutes ?? 15;
  const windowOpen =
    scheduledAt
      ? new Date(scheduledAt.getTime() - windowMin * 60 * 1_000)
      : null;
  const windowEnd = reg?.exam.scheduled_end_at
    ? new Date(reg.exam.scheduled_end_at)
    : scheduledAt;
  const inWindow =
    windowOpen !== null &&
    windowEnd !== null &&
    now >= windowOpen &&
    now < windowEnd;
  const isBeforeWindow =
    windowOpen !== null && scheduledAt !== null && now < windowOpen;
  const isAfterWindow = windowEnd !== null && now >= windowEnd;

  const showCheckInForm =
    reg?.exam.requires_checkin && reg?.status === "registered";
  const showStartGate =
    reg?.status === "checked_in" ||
    reg?.status === "in_progress" ||
    !reg?.exam.requires_checkin;
  const isResume = reg?.status === "in_progress";
  const startDisabled = Boolean(
    reg?.exam.requires_checkin &&
      reg?.status === "checked_in" &&
      scheduledAt &&
      now < scheduledAt,
  );

  // ── Loading / error / empty states ───────────────────────────────────────

  if (isLoading) {
    return <DetailSkeleton />;
  }

  if (isError) {
    return (
      <>
        <Card className="gap-0 border-danger/30 bg-danger-bg px-5 py-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="size-5 text-danger" />
            <div className="flex-1 text-sm text-ink-700">
              {t("competition_error")}
              {error instanceof Error && error.message
                ? ` ${error.message}`
                : ""}
            </div>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        </Card>
        <Button
          variant="ghost"
          size="sm"
          className="w-fit"
          onClick={() => router.push("/exam")}
        >
          {t("competition_detail_back")}
        </Button>
      </>
    );
  }

  if (!reg) {
    return (
      <>
        <p className="text-sm text-ink-600">
          {t("competition_detail_not_found")}
        </p>
        <Button
          variant="ghost"
          size="sm"
          className="w-fit"
          onClick={() => router.push("/exam")}
        >
          {t("competition_detail_back")}
        </Button>
      </>
    );
  }

  const exam = reg.exam;
  const cardState = computeCardState(
    {
      status: reg.status,
      max_attempts: exam.max_attempts,
      attempts_used: reg.attempts_used,
      requires_checkin: exam.requires_checkin,
      scheduled_at: exam.scheduled_at,
      scheduled_end_at: exam.scheduled_end_at,
      check_in_window_minutes: exam.check_in_window_minutes,
    },
    now.getTime(),
  );
  const briefingStatus =
    cardState.kind === "start"
      ? "ready"
      : cardState.kind === "locked"
        ? "upcoming"
        : cardState.kind === "submitted"
          ? "done"
          : cardState.kind === "in_progress"
            ? "ready"
            : cardState.kind;

  const afterHowTo = (
    <Card className="gap-4 p-6">
      <dl className="text-sm">
        <dt className="type-label-sm text-ink-900">
          {t("competition_detail_token_label")}
        </dt>
        <dd className="mt-1 flex items-center gap-2 font-mono text-base font-semibold text-ink-900">
          <span className="select-all">{showToken ? reg.token : "••••••••"}</span>
          <Button
            type="button"
            size="xs"
            variant="ghost"
            onClick={() => setShowToken((v) => !v)}
            aria-label={
              showToken ? t("competition_detail_hide_token") : t("competition_detail_show_token")
            }
          >
            {showToken ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
            <span className="ml-1 text-xs">
              {showToken ? t("competition_detail_hide_token") : t("competition_detail_show_token")}
            </span>
          </Button>
        </dd>
      </dl>
      {exam.card_enabled && (
        <div>
          <Button type="button" size="sm" onClick={handleDownload}>
            <Download className="size-4" />
            {t("competition_detail_download_card")}
          </Button>
        </div>
      )}
    </Card>
  );

  const checkIn = showCheckInForm ? (
    <Card className="gap-0 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="type-h2 text-ink-900">{t("exam_checkin_title")}</h2>
        <Badge variant="gold">{t("exam_required_badge")}</Badge>
      </div>

      {isBeforeWindow && <p className="mt-2 text-sm text-ink-600">{t("window_closed_early")}</p>}
      {isAfterWindow && <p className="mt-2 text-sm text-ink-600">{t("window_closed_late")}</p>}

      <div className="mt-3.5 space-y-3">
        <div>
          <label htmlFor="checkin-token" className="mb-[7px] block type-label-sm text-ink-900">
            {t("token_label")}
          </label>
          <Input
            id="checkin-token"
            type="text"
            value={token}
            onChange={(e) => setToken(e.target.value.toUpperCase())}
            placeholder={t("token_placeholder")}
            disabled={!inWindow}
            className="font-mono uppercase"
          />
        </div>
        <p className="text-xs text-ink-600">{t("token_hint")}</p>
        <Button
          type="button"
          variant="secondary"
          onClick={handleCheckIn}
          disabled={!inWindow || !token.trim() || checkInMutation.isPending}
        >
          {checkInMutation.isPending ? t("sys_loading") : t("check_in")}
        </Button>
      </div>
    </Card>
  ) : null;

  return (
    <ExamBriefing
      exam={exam}
      backHref="/exam"
      backLabel={t("competition_detail_back")}
      status={briefingStatus}
      afterHowTo={afterHowTo}
      checkIn={checkIn}
      gate={
        showStartGate
          ? {
              kind: isResume ? "resume" : "start",
              disabled: startDisabled,
              hint: startDisabled && scheduledAt ? t("not_started") : null,
              pending: startSessionMutation.isPending,
              onStart: handleStart,
            }
          : null
      }
    />
  );
}

function DetailSkeleton() {
  return (
    <>
      <Skeleton className="h-4 w-32" />
      <Skeleton className="h-8 w-2/3" />
      <Skeleton className="h-64 w-full rounded-[22px]" />
    </>
  );
}
