"use client";

import Link from "next/link";
import { Play } from "lucide-react";
import type { CourseSession, EventIllustration } from "@/lib/types";
import { useCourse } from "@/lib/hooks/courses";
import { Badge } from "@/components/ui/badge";
import { AbakAvatar } from "@/components/ui/abak-avatar";
import { useTranslation } from "@/lib/i18n";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { Spot } from "@/components/dashboard/Spot";

export interface CourseCardProps {
  session: CourseSession;
  gradient: string;
}

// Prototype spotFor(subject).
const SUBJECT_SPOT: Record<string, EventIllustration | undefined> = {
  Matematika: "math",
  Penalaran: "math",
  Sains: "science",
  Fisika: "science",
  Kimia: "science",
  "Bahasa Inggris": "language",
  Literasi: "language",
  "Bahasa Indonesia": "language",
  Coding: "coding",
  AI: "coding",
};

export const subjectSpot = (subject?: string) => (subject ? SUBJECT_SPOT[subject] : undefined);

// Prototype continueCard (.ccard).
export function CourseCard({ session, gradient }: CourseCardProps) {
  const { t } = useTranslation();
  const { data: course, isLoading } = useCourse(session.course_id);
  const lessons = course?.sections?.flatMap((s) => s.lessons ?? []) ?? [];
  const done = session.completed_lessons ?? {};
  const totalLessons = lessons.length;
  const doneLessons = lessons.filter((l) => l.id in done).length;
  const pct = totalLessons > 0 ? Math.round((doneLessons / totalLessons) * 100) : 0;
  const finished = totalLessons > 0 && doneLessons >= totalLessons;
  const next = lessons.find((l) => !(l.id in done)) ?? lessons[0];

  return (
    <Link
      href={`/courses/${session.course_id}`}
      className="group flex flex-col overflow-hidden rounded-[var(--radius-card)] border border-line bg-card shadow-card transition-all duration-200 hover:-translate-y-[5px] hover:shadow-pop motion-reduce:hover:translate-y-0"
    >
      <div
        data-testid="course-cover"
        className="relative grid h-[150px] place-items-center overflow-hidden"
        style={{ background: gradient }}
      >
        <Spot
          illustration={subjectSpot(course?.subject)}
          className="relative z-[1] size-[110px] drop-shadow-[0_10px_14px_rgba(0,0,0,0.18)]"
        />
        {course?.title && (
          <span
            aria-hidden
            className="absolute right-[-10px] bottom-[-34px] font-[family-name:var(--font-display)] text-[150px] leading-none font-extrabold tracking-[-0.06em] text-white/16"
          >
            {course.title.trim().charAt(0).toUpperCase()}
          </span>
        )}
        {totalLessons > 0 && (
          <Badge className="absolute top-3 left-3 z-[2] bg-white/85 text-[#14202b] backdrop-blur-[8px]">
            {finished
              ? t("course_card_done")
              : t("stu_home_learn_lesson_of")
                  .replace("{x}", String(lessons.indexOf(next!) + 1))
                  .replace("{y}", String(totalLessons))}
          </Badge>
        )}
        <span className="absolute right-3 bottom-3 z-[2] grid size-10 place-items-center rounded-full bg-white text-[#14202b] shadow-pop">
          <Play className="size-4" aria-hidden />
        </span>
      </div>
      <div className="flex flex-1 flex-col gap-[9px] px-[18px] pt-4 pb-[18px]">
        {isLoading || !course ? (
          <Skeleton className="h-5 w-3/4" />
        ) : (
          <h3 className="font-[family-name:var(--font-display)] text-base leading-[1.35] font-bold tracking-[-0.02em] text-ink-900">
            {course.title}
          </h3>
        )}
        {course && totalLessons > 0 && (
          <div className="flex items-center gap-2 text-[13px] text-ink-600">
            <AbakAvatar name={course.instructor_name ?? course.title} size="xs" ring={false} />
            <span className="min-w-0 truncate">
              {finished ? t("stu_home_learn_done") : t("course_card_next").replace("{title}", next?.title ?? "")}
            </span>
          </div>
        )}
        <div className="flex flex-wrap items-center justify-between gap-2 text-[13px] text-ink-600">
          <span>{t("course_card_progress")}</span>
          <b className="text-teal-700">{pct}%</b>
        </div>
        <Progress value={pct} />
      </div>
    </Link>
  );
}
