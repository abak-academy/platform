"use client";

import { useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { AlertCircle, Check, Loader2 } from "lucide-react";
import { toast } from "sonner";

import { useCourse, useCompleteLesson } from "@/lib/hooks/courses";
import { useTranslation } from "@/lib/i18n";
import type { Lesson } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHead } from "@/components/shell/PageHead";
import { LessonList } from "@/components/courses/LessonList";
import { VideoPlayer } from "@/components/courses/VideoPlayer";

type SectionWithLessons = NonNullable<
  ReturnType<typeof useCourse>["data"]
>["sections"] extends infer S
  ? S extends Array<infer U>
    ? U
    : never
  : never;

function flattenLessons(
  sections: SectionWithLessons[] | undefined
): Lesson[] {
  if (!sections) return [];
  return sections.flatMap((s) => s.lessons ?? []);
}

export default function CourseDetailPage() {
  const { t } = useTranslation();
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const courseId = params?.id ?? "";

  const { data: course, isLoading, isError, error, refetch } = useCourse(courseId);
  const completeLesson = useCompleteLesson();

  const flatLessons = useMemo(
    () => flattenLessons(course?.sections),
    [course?.sections]
  );

  const firstIncompleteId = useMemo(
    () => flatLessons.find((l) => !l.completed)?.id ?? flatLessons[0]?.id,
    [flatLessons]
  );

  const [activeLessonId, setActiveLessonId] = useState<string | undefined>(
    undefined
  );

  const activeId = activeLessonId ?? firstIncompleteId;
  const activeLesson = flatLessons.find((l) => l.id === activeId);

  const totalCount = flatLessons.length;
  const doneCount = flatLessons.filter((l) => l.completed).length;
  const progressPct =
    totalCount > 0 ? Math.round((doneCount / totalCount) * 100) : 0;

  const handleToggleComplete = () => {
    if (!activeLesson) return;
    const wasDone = Boolean(activeLesson.completed);
    completeLesson.mutate(
      { courseId, lessonId: activeLesson.id },
      {
        onSuccess: () => {
          toast.success(
            wasDone ? t("course_lesson_undone") : t("course_lesson_done")
          );
        },
        onError: (err: unknown) => {
          const message =
            err instanceof Error ? err.message : t("course_update_failed");
          toast.error(message);
        },
      }
    );
  };

  if (isLoading) {
    return <CourseDetailSkeleton />;
  }

  if (isError) {
    return (
      <>
        <Card className="gap-0 border-danger/30 bg-danger-bg px-5 py-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="size-5 text-danger" />
            <div className="flex-1 text-sm text-ink-700">
              {t("course_load_failed")}
              {error instanceof Error && error.message
                ? ` ${error.message}`
                : ""}
            </div>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        </Card>
      </>
    );
  }

  if (!course) {
    return (
      <>
        <p className="text-sm text-ink-600">{t("course_not_found")}</p>
        <Button
          variant="ghost"
          size="sm"
          className="w-fit"
          onClick={() => router.push("/courses")}
        >
          {t("course_back")}
        </Button>
      </>
    );
  }

  const sections = course.sections ?? [];
  const activeSection = sections.find((s) =>
    s.lessons?.some((l) => l.id === activeId)
  );

  return (
    <>
      <PageHead
        back={{ href: "/courses", label: t("course_back") }}
        title={course.title}
        sub={course.instructor_name ? `${t("course_instructor")}: ${course.instructor_name}` : undefined}
      />

      <div className="grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(0,1fr)_330px]">
        <div className="flex min-w-0 flex-col gap-4">
          <VideoPlayer
            videoRef={activeLesson?.video_url}
            title={activeLesson?.title}
          />

          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="min-w-0">
              {activeSection && (
                <div className="font-mono text-[10.5px] font-semibold tracking-[0.1em] text-ink-600 uppercase">
                  {activeSection.title}
                </div>
              )}
              <h2 className="mt-1 text-[26px] leading-tight font-extrabold tracking-[-0.03em] text-ink-900 [font-family:var(--font-display)]">
                {activeLesson?.title ?? t("course_select_lesson")}
              </h2>
              {typeof activeLesson?.duration_seconds === "number" &&
                activeLesson.duration_seconds > 0 && (
                  <p className="mt-1 text-xs text-ink-600">
                    {t("course_duration").replace("{n}", String(Math.round(activeLesson.duration_seconds / 60)))}
                  </p>
                )}
            </div>
            <Button
              type="button"
              variant={activeLesson?.completed ? "outline" : "default"}
              disabled={!activeLesson || completeLesson.isPending}
              onClick={handleToggleComplete}
              className="shrink-0"
            >
              {completeLesson.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : activeLesson?.completed ? (
                <Check className="size-4" />
              ) : null}
              {activeLesson?.completed ? t("course_complete") : t("course_mark_complete")}
            </Button>
          </div>
        </div>

        <LessonList
          sections={sections}
          activeLessonId={activeId}
          onSelectLesson={setActiveLessonId}
          doneCount={doneCount}
          totalCount={totalCount}
          progressPct={progressPct}
        />
      </div>
    </>
  );
}

function CourseDetailSkeleton() {
  return (
    <>
      <Skeleton className="h-4 w-32" />
      <Skeleton className="h-8 w-2/3" />
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_330px]">
        <div>
          <Skeleton className="h-0 w-full pb-[56.25%]" />
          <Skeleton className="mt-4 h-24 w-full" />
        </div>
        <Skeleton className="h-80 w-full" />
      </div>
    </>
  );
}
