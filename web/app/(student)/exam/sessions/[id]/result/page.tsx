"use client";

import { useParams } from "next/navigation";
import type { LucideIcon } from "lucide-react";
import { AlertCircle, Award, Clock, EyeOff, Lock } from "lucide-react";

import { useSessionResult } from "@/lib/hooks/exam";
import { useTranslation, DICT } from "@/lib/i18n";
type I18nKey = keyof typeof DICT.id;
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { RichContent } from "@/components/admin/RichContent";
import type { ResultPembahasanItem } from "@/lib/types";

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat("id-ID", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

export default function SessionResultPage() {
  const { t } = useTranslation();
  const params = useParams<{ id: string }>();
  const sessionId = params?.id ?? "";

  const {
    data: result,
    isLoading,
    isError,
    error,
    refetch,
  } = useSessionResult(sessionId);

  // ── Error state ────────────────────────────────────────────────────────

  if (isError) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-8">
        <Card className="border-danger/30 bg-danger-bg px-5 py-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="size-5 text-danger" />
            <div className="flex-1 text-sm text-ink-700">
              {t("sys_error_load")}
              {error instanceof Error && error.message
                ? ` ${error.message}`
                : ""}
            </div>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  // ── Loading state ──────────────────────────────────────────────────────

  if (isLoading || !result) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-8">
        <p className="mb-4 text-sm text-ink-500">{t("sys_loading")}</p>
        <Skeleton className="mb-4 h-40 w-full rounded-lg" />
        <Skeleton className="h-24 w-full rounded-lg" />
      </div>
    );
  }

  // ── Configured end screen (FR-38) ──────────────────────────────────────
  // Shown after submit whenever an admin has set both an image and a promo
  // text for this exam — replaces the plain result screen below regardless
  // of result-visibility state (FR-39: falls back to plain when unset).

  if (result.end_screen_image_url && result.end_screen_promo_text) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-12 text-center">
        <img
          src={result.end_screen_image_url}
          alt={t("result_end_screen_image_alt")}
          data-testid="end-screen-image"
          className="mx-auto mb-6 max-h-80 w-full rounded-lg object-contain"
        />
        <p className="whitespace-pre-wrap text-ink-800">
          {result.end_screen_promo_text}
        </p>
      </div>
    );
  }

  // ── Non-result states (hidden / grading / locked) ─────────────────────

  if (result.state === "hidden") {
    return <NonResultCard icon={EyeOff} title={t("result_hidden")} certificateUrl={result.certificate_url} />;
  }

  if (result.state === "grading") {
    return <NonResultCard icon={Clock} title={t("result_grading")} certificateUrl={result.certificate_url} />;
  }

  if (result.state === "locked") {
    return (
      <NonResultCard
        icon={Lock}
        title={t("result_locked").replace(
          "{t}",
          formatDate(result.result_release_at),
        )}
        certificateUrl={result.certificate_url}
      />
    );
  }

  // ── Result state ────────────────────────────────────────────────────────

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      <Card className="mb-4 p-5 text-center">
        <Award className="mx-auto mb-3 size-10 text-brand-600" />
        <p className="text-sm text-ink-500">{t("result_your_score")}</p>
        <p className="mb-4 text-4xl font-bold text-ink-900">{result.score}</p>
        <div className="grid grid-cols-3 gap-3 text-sm">
          <div>
            <p className="font-semibold text-success">
              {result.correct_count}
            </p>
            <p className="text-ink-500">{t("result_correct")}</p>
          </div>
          <div>
            <p className="font-semibold text-danger">{result.wrong_count}</p>
            <p className="text-ink-500">{t("result_incorrect")}</p>
          </div>
          <div>
            <p className="font-semibold text-ink-700">
              {result.empty_count}
            </p>
            <p className="text-ink-500">{t("result_empty")}</p>
          </div>
        </div>
        <div className="mt-4 border-t border-line pt-4">
          <p className="text-sm text-ink-500">{t("result_rank")}</p>
          <p className="text-2xl font-bold text-ink-900">#{result.rank}</p>
        </div>
      </Card>

      {result.result_config === "score_pembahasan" && (
        <>
          <Card className="mb-4 p-5">
            <h2 className="mb-3 text-sm font-semibold text-ink-900">
              {t("result_by_topic")}
            </h2>
            <div className="space-y-3">
              {result.breakdown.map((row) => (
                <div key={row.test_id}>
                  <div className="mb-1 flex items-center justify-between text-sm">
                    <span className="flex items-center gap-2 text-ink-800">
                      {row.title}
                      {row.section_type && (
                        <span className="rounded bg-ink-100 px-1.5 py-0.5 text-[11px] font-medium text-ink-500">
                          {t(
                            `section_type_${row.section_type}` as I18nKey,
                          )}
                        </span>
                      )}
                    </span>
                    <span className="text-ink-500">
                      {row.earned}/{row.max}
                    </span>
                  </div>
                  <Progress
                    value={
                      row.max > 0
                        ? Math.min(100, Math.max(0, (row.earned / row.max) * 100))
                        : 0
                    }
                  />
                </div>
              ))}
            </div>
          </Card>

          <Card className="mb-4 p-5">
            <h2 className="mb-3 text-sm font-semibold text-ink-900">
              {t("result_pembahasan")}
            </h2>
            <div className="space-y-2">
              {result.pembahasan.map((item, i) => (
                <PembahasanItem
                  key={item.question_id}
                  index={i}
                  item={item}
                  t={t}
                />
              ))}
            </div>
          </Card>
        </>
      )}

      {result.certificate_url && (
        <Button variant="outline" className="w-full" asChild>
          <a href={result.certificate_url} target="_blank" rel="noreferrer">
            <Award className="size-4" />
            {t("certificate")}
          </a>
        </Button>
      )}

      <Button variant="outline" className="w-full" asChild>
        <a href={`/exam/sessions/${sessionId}/leaderboard`}>
          {t("view_leaderboard")}
        </a>
      </Button>
    </div>
  );
}

function NonResultCard({
  icon: Icon,
  title,
  certificateUrl,
}: {
  icon: LucideIcon;
  title: string;
  certificateUrl?: string | null;
}) {
  const { t } = useTranslation();
  return (
    <div className="mx-auto flex max-w-md flex-col items-center justify-center px-4 py-24 text-center">
      <Icon className="mb-4 size-12 text-ink-400" />
      <h1 className="text-xl font-bold text-ink-900">{title}</h1>
      {certificateUrl && (
        <Button variant="outline" className="mt-6 w-full" asChild>
          <a href={certificateUrl} target="_blank" rel="noreferrer">
            <Award className="size-4" />
            {t("certificate")}
          </a>
        </Button>
      )}
    </div>
  );
}

function PembahasanItem({
  index,
  item,
  t,
}: {
  index: number;
  item: ResultPembahasanItem;
  t: (key: I18nKey) => string;
}) {
  return (
    <details className="rounded-lg border border-line p-3">
      <summary className="cursor-pointer text-sm font-medium text-ink-800">
        {index + 1}. <RichContent html={item.body} />
      </summary>
      <div className="mt-3 space-y-2 text-sm">
        <p>
          <span className="text-ink-500">{t("result_your_answer")}: </span>
          <span className={item.is_correct ? "text-success" : "text-danger"}>
            {item.your_answer || "—"}
          </span>
        </p>
        {!item.is_correct && item.correct_answer != null && (
          <p>
            <span className="text-ink-500">
              {t("result_correct_answer")}:{" "}
            </span>
            <span className="text-success">{item.correct_answer}</span>
          </p>
        )}
        {item.explanation && (
          <p className="whitespace-pre-wrap text-ink-700">
            {item.explanation}
          </p>
        )}
      </div>
    </details>
  );
}
