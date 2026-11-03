"use client";

import { useParams } from "next/navigation";
import { AlertCircle } from "lucide-react";

import { ExamBriefing } from "@/components/exam/ExamBriefing";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useExamPreview } from "@/lib/hooks/admin-exam-preview";
import { useTranslation } from "@/lib/i18n";

export default function ExamPreviewPage() {
  const { t } = useTranslation();
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const { data, isLoading, isError, error, refetch } = useExamPreview(id);

  if (isLoading) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-6 md:px-6 md:py-8">
        <Skeleton className="mb-6 h-8 w-2/3" />
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-8 md:px-6">
        <Card className="border-danger/30 bg-danger-bg px-5 py-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="size-5 text-danger" />
            <div className="flex-1 text-sm text-ink-700">
              {t("competition_error")}
              {error instanceof Error && error.message ? ` ${error.message}` : ""}
            </div>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <ExamBriefing
      exam={data}
      preview
      backHref={`/admin/exam/packages/${id}`}
      backLabel={t("admin_exam_preview_back")}
      gate={null}
    />
  );
}
