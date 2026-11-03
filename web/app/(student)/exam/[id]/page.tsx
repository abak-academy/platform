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
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
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
  const inWindow =
    windowOpen !== null &&
    scheduledAt !== null &&
    now >= windowOpen &&
    now < scheduledAt;
  const isBeforeWindow =
    windowOpen !== null && scheduledAt !== null && now < windowOpen;
  const isAfterWindow =
    scheduledAt !== null && now >= scheduledAt;

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
      <div className="mx-auto max-w-3xl px-4 py-8 md:px-6">
        <Card className="border-danger/30 bg-danger-bg px-5 py-4">
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
          className="mt-4"
          onClick={() => router.push("/exam")}
        >
          {t("competition_detail_back")}
        </Button>
      </div>
    );
  }

  if (!reg) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-8 md:px-6">
        <p className="text-sm text-ink-500">
          {t("competition_detail_not_found")}
        </p>
        <Button
          variant="ghost"
          size="sm"
          className="mt-3"
          onClick={() => router.push("/exam")}
        >
          {t("competition_detail_back")}
        </Button>
      </div>
    );
  }

  const exam = reg.exam;

  const afterInfo = (
    <Card className="mt-4 space-y-4 p-5">
      <dl className="text-sm">
        <dt className="text-xs uppercase tracking-wide text-ink-500">
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
    <Card className="mt-4 p-5">
      <h3 className="font-serif text-base font-semibold text-ink-900">{t("exam_checkin_title")}</h3>

      {isBeforeWindow && <p className="mt-2 text-sm text-ink-500">{t("window_closed_early")}</p>}
      {isAfterWindow && <p className="mt-2 text-sm text-ink-500">{t("window_closed_late")}</p>}

      <div className="mt-3 space-y-3">
        <div>
          <label htmlFor="checkin-token" className="text-xs uppercase tracking-wide text-ink-500">
            {t("token_label")}
          </label>
          <input
            id="checkin-token"
            type="text"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder={t("token_placeholder")}
            disabled={!inWindow}
            className="mt-1 block w-full rounded-md border border-ink-200 bg-white px-3 py-2 text-sm text-ink-900 placeholder:text-ink-400 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary disabled:cursor-not-allowed disabled:opacity-50"
          />
        </div>
        <p className="text-xs text-ink-400">{t("token_hint")}</p>
        <Button
          type="button"
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
      afterInfo={afterInfo}
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
    <div className="mx-auto max-w-3xl px-4 py-6 md:px-6 md:py-8">
      <Skeleton className="mb-4 h-4 w-32" />
      <Skeleton className="mb-6 h-8 w-2/3" />
      <Skeleton className="h-64 w-full rounded-lg" />
    </div>
  );
}
