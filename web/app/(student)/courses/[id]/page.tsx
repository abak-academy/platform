"use client";

import { useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { AlertCircle, CheckCircle2, ChevronLeft, Loader2 } from "lucide-react";
import { toast } from "sonner";

import { useCourse, useCompleteLesson } from "@/lib/hooks/courses";
import { useTranslation } from "@/lib/i18n";
import type { Lesson } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { AbakAvatar } from "@/components/ui/abak-avatar";
import { LessonList } from "@/components/courses/LessonList";
import { VideoPlayer } from "@/components/courses/VideoPlayer";
import { subjectSpot } from "@/components/courses/CourseCard";

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
        <Card className="gap-0 border-danger/30 bg-danger-bg p-4">
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
  const activeIdx = activeLesson ? flatLessons.indexOf(activeLesson) : -1;
  const prevLesson = activeIdx > 0 ? flatLessons[activeIdx - 1] : undefined;
  const minutes =
    typeof activeLesson?.duration_seconds === "number" && activeLesson.duration_seconds > 0
      ? Math.round(activeLesson.duration_seconds / 60)
      : undefined;

  // Prototype .learn: main column + 330px curriculum, stacked at <=1024.
  return (
    <div className="grid grid-cols-1 items-start gap-5 min-[1025px]:grid-cols-[minmax(0,1fr)_330px]">
      <div className="flex min-w-0 flex-col gap-4">
        <Link
          href="/courses"
          className="mb-1 inline-flex items-center gap-1 font-[family-name:var(--font-display)] text-[13px] leading-[normal] font-semibold text-ink-600 hover:text-ink-900"
        >
          <ChevronLeft className="size-4" aria-hidden />
          {course.title}
        </Link>

        <VideoPlayer
          videoRef={activeLesson?.video_url}
          title={activeLesson?.title}
          spot={subjectSpot(course.subject)}
        />

        <div className="flex flex-wrap items-center gap-2">
          <div className="min-w-0 flex-1 max-narrow:basis-full">
            {activeLesson && (
              <span className="font-mono text-[10.5px] leading-[1.4] font-semibold tracking-[0.12em] text-ink-600 uppercase">
                {t("course_lesson_of").replace("{x}", String(activeIdx + 1)).replace("{y}", String(totalCount))}
                {minutes !== undefined && ` · ${t("course_minutes").replace("{n}", String(minutes))}`}
              </span>
            )}
            <h1 className="mt-0.5 font-[family-name:var(--font-display)] text-[26px] leading-[normal] font-extrabold tracking-[-0.02em] text-ink-900">
              {activeLesson?.title ?? t("course_select_lesson")}
            </h1>
          </div>
          {prevLesson && (
            <Button type="button" variant="outline" onClick={() => setActiveLessonId(prevLesson.id)}>
              <ChevronLeft aria-hidden />
              {t("course_prev")}
            </Button>
          )}
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
              <CheckCircle2 className="size-4" />
            ) : null}
            {activeLesson?.completed ? t("course_complete") : t("course_mark_complete")}
          </Button>
        </div>

        {course.instructor_name && (
          <Card className="gap-0 p-6">
            <div className="flex items-center gap-3">
              <AbakAvatar name={course.instructor_name} size="md" />
              <div>
                <b className="block text-[14px] text-ink-900">{course.instructor_name}</b>
                <small className="block text-[12px] text-ink-600">
                  {course.subject
                    ? t("course_teacher_of").replace("{subject}", course.subject)
                    : t("course_instructor")}
                </small>
              </div>
            </div>
          </Card>
        )}
      </div>

      <LessonList
        title={course.title}
        sections={sections}
        activeLessonId={activeId}
        onSelectLesson={setActiveLessonId}
        doneCount={doneCount}
        totalCount={totalCount}
        progressPct={progressPct}
      />
    </div>
  );
}

function CourseDetailSkeleton() {
  return (
    <>
      <Skeleton className="h-4 w-32" />
      <Skeleton className="h-8 w-2/3" />
      <div className="grid grid-cols-1 gap-5 min-[1025px]:grid-cols-[minmax(0,1fr)_330px]">
        <div>
          <Skeleton className="h-0 w-full pb-[56.25%]" />
          <Skeleton className="mt-4 h-24 w-full" />
        </div>
        <Skeleton className="h-80 w-full" />
      </div>
    </>
  );
}
