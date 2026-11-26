"use client";

import Link from "next/link";
import { Play } from "lucide-react";
import type { CourseSession } from "@/lib/types";
import { useCourse } from "@/lib/hooks/courses";
import { Badge } from "@/components/ui/badge";
import { useTranslation } from "@/lib/i18n";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";

export interface CourseCardProps {
  session: CourseSession;
  gradient: string;
}

function countLessons(
  sections?: { lessons?: { id: string }[] }[]
): number {
  if (!sections) return 0;
  return sections.reduce(
    (acc, s) => acc + (s.lessons?.length ?? 0),
    0
  );
}

export function CourseCard({ session, gradient }: CourseCardProps) {
  const { t } = useTranslation();
  const { data: course, isLoading } = useCourse(session.course_id);
  const totalLessons = countLessons(course?.sections);
  const doneLessons = Object.keys(session.completed_lessons ?? {}).length;
  const pct = totalLessons > 0 ? Math.round((doneLessons / totalLessons) * 100) : 0;
  const finished = totalLessons > 0 && doneLessons >= totalLessons;

  return (
    <Link
      href={`/courses/${session.course_id}`}
      className="group flex flex-col overflow-hidden rounded-[var(--radius-card)] border border-line bg-card shadow-card transition-all duration-200 hover:-translate-y-[5px] hover:shadow-pop motion-reduce:hover:translate-y-0"
    >
      <div data-testid="course-cover" className="relative h-[150px]" style={{ background: gradient }}>
        {totalLessons > 0 && (
          <Badge className="absolute top-3 left-3 bg-white/85 text-ink-900 backdrop-blur-sm">
            {finished
              ? t("stu_home_learn_done")
              : t("stu_home_learn_lesson_of")
                  .replace("{x}", String(Math.min(doneLessons + 1, totalLessons)))
                  .replace("{y}", String(totalLessons))}
          </Badge>
        )}
        <span className="absolute right-3 bottom-3 grid size-10 place-items-center rounded-full bg-white text-brand-600 shadow-pop">
          <Play className="size-4 fill-current" aria-hidden />
        </span>
      </div>
      <div className="flex flex-col gap-[9px] px-[18px] pt-4 pb-[18px]">
        {isLoading || !course ? (
          <Skeleton className="h-5 w-3/4" />
        ) : (
          <h3 className="text-base font-bold text-ink-900">{course.title}</h3>
        )}
        <div className="flex items-center justify-between text-[13px] text-ink-600">
          <span className="font-mono font-bold">
            {String(doneLessons).padStart(2, "0")}
            <span className="text-ink-600/70">/{totalLessons}</span>
          </span>
          <b className="text-teal-700">{pct}%</b>
        </div>
        <Progress value={pct} />
      </div>
    </Link>
  );
}
