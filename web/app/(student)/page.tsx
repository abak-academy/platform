"use client";

import Link from "next/link";
import { AlertCircle, ChevronRight, Plus, Trophy } from "lucide-react";
import { useDashboard } from "@/lib/hooks/students";
import { useTranslation } from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PendingBanner } from "@/components/dashboard/PendingBanner";
import { RankingCard } from "@/components/dashboard/RankingCard";
import { StudySummaryCard } from "@/components/dashboard/StudySummaryCard";
import { ExamProgressCard } from "@/components/dashboard/ExamProgressCard";
import { PopularLessonsSection } from "@/components/dashboard/PopularLessonsSection";

const COURSE_GRADIENTS = [
  "linear-gradient(135deg, #EEF0FE 0%, #F3E8FB 45%, #FDE8EE 100%)",
  "linear-gradient(135deg, #EAF0FE 0%, #E8F6FB 50%, #EEF0FE 100%)",
  "linear-gradient(135deg, #F0ECFC 0%, #EAF0FE 55%, #E8F7F2 100%)",
  "linear-gradient(135deg, #FDEEF0 0%, #EEF0FE 60%, #EAF6FB 100%)",
];

function firstName(name?: string): string {
  if (!name) return "";
  return name.trim().split(/\s+/)[0];
}

export default function DashboardPage() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const { data, isLoading, isError, error, refetch } = useDashboard();
  const name = firstName(user?.name ?? user?.username ?? undefined);

  const greeting = (() => {
    const h = new Date().getHours();
    if (h < 11) return t("greeting_morning");
    if (h < 16) return t("greeting_afternoon");
    return t("greeting_evening");
  })();

  return (
    <div>
      {isError && (
        <Card className="mb-8 border-danger/30 bg-danger-bg px-5 py-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="size-5 text-danger" />
            <div className="flex-1 text-sm text-ink-700">
              {t("dash_load_failed")}
              {error instanceof Error && error.message ? ` ${error.message}` : ""}
            </div>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        </Card>
      )}

      {isLoading ? (
        <DashboardSkeleton />
      ) : data ? (
        <>
          {/* (a) Hero + (b) Ranking */}
          <section className="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-5">
            <div
              className="relative overflow-hidden rounded-xl lg:col-span-3"
              style={{ background: "var(--gradient-hero)" }}
            >
              <div className="flex h-full flex-col justify-between p-6 md:p-8">
                <div>
                  <p className="text-sm font-medium text-white/70">
                    {greeting}{name ? `, ${name}` : ""}
                  </p>
                  <h1 className="mt-2 font-serif text-2xl font-bold text-white md:text-3xl">
                    {t("dash_hero_title")}
                  </h1>
                  <p className="mt-2 max-w-xs text-sm text-white/80">
                    {t("dash_hero_sub")}
                  </p>
                </div>
                <Button
                  asChild
                  size="sm"
                  className="mt-6 w-fit border-white/30 bg-white/15 text-white hover:bg-white/25"
                  variant="outline"
                >
                  <Link href="/catalog">
                    {t("dash_explore_btn")} <ChevronRight className="size-4" />
                  </Link>
                </Button>
              </div>
              <Trophy className="pointer-events-none absolute bottom-4 right-6 size-28 text-white opacity-10" />
            </div>

            <div className="lg:col-span-2">
              <RankingCard ranking={data.ranking} />
            </div>
          </section>

          {/* (c) Pending order banner */}
          {data.pending_order && (
            <div className="mb-6">
              <PendingBanner
                id={data.pending_order.id}
                product={data.pending_order.product}
                amount={data.pending_order.amount}
              />
            </div>
          )}

          {/* (d) My Courses */}
          <section className="mb-8">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="font-serif text-xl font-semibold text-ink-900">
                {t("nav_courses")}
              </h2>
              <Link
                href="/courses"
                className="inline-flex items-center gap-1 text-sm font-medium text-brand-700 hover:text-brand-800"
              >
                {t("dash_view_all")} <ChevronRight className="size-4" />
              </Link>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {data.enrolled_courses.map((course, i) => (
                <CourseCard
                  key={course.id}
                  course={course}
                  gradient={COURSE_GRADIENTS[i % COURSE_GRADIENTS.length]}
                />
              ))}
              <AddCourseCard />
            </div>
          </section>

          {/* (e) Study Summary + (f) Exam Progress */}
          <section className="mb-8 grid grid-cols-1 gap-4 md:grid-cols-2">
            <StudySummaryCard study={data.study_summary} />
            <ExamProgressCard examProgress={data.exam_progress} />
          </section>

          {/* (g) Popular Lessons */}
          <PopularLessonsSection lessons={data.popular_lessons} />
        </>
      ) : null}
    </div>
  );
}

function AddCourseCard() {
  const { t } = useTranslation();
  return (
    <Link
      href="/catalog"
      className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-line bg-surface-2 p-6 text-center transition-colors hover:border-brand-400 hover:bg-brand-50/50"
    >
      <div className="flex size-10 items-center justify-center rounded-full bg-brand-50 text-brand-600">
        <Plus className="size-5" />
      </div>
      <span className="text-sm font-medium text-ink-500">{t("add_course")}</span>
    </Link>
  );
}

function CourseCard({
  course,
  gradient,
}: {
  course: {
    id: string;
    title: string;
    progress: number;
    total_lessons: number;
    done_lessons: number;
  };
  gradient: string;
}) {
  const pct = Math.round((course.progress ?? 0) * 100);
  return (
    <Link
      href={`/courses/${course.id}`}
      className="group flex flex-col rounded-lg border border-line p-5 transition-all hover:-translate-y-0.5 hover:shadow-md"
      style={{ background: gradient }}
    >
      <h3 className="text-base font-bold text-ink-900">{course.title}</h3>
      <div className="mt-auto flex items-center justify-between pt-4">
        <span className="font-mono text-sm font-bold text-brand-700">
          {String(course.done_lessons).padStart(2, "0")}
          <span className="text-ink-400">/{course.total_lessons}</span>
        </span>
        <span className="flex size-7 items-center justify-center rounded-full bg-white shadow-sm text-brand-600 transition-transform group-hover:translate-x-0.5">
          <ChevronRight className="size-4" />
        </span>
      </div>
      <Progress
        value={pct}
        className="mt-3 bg-ink-900/10 [&>[data-slot=progress-indicator]]:bg-brand-600"
      />
    </Link>
  );
}

function DashboardSkeleton() {
  return (
    <div className="space-y-6" data-testid="dashboard-skeleton">
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-5">
        <Skeleton className="h-52 rounded-xl lg:col-span-3" />
        <Skeleton className="h-52 rounded-xl lg:col-span-2" />
      </div>
      <div>
        <Skeleton className="mb-4 h-6 w-32" />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-36 rounded-lg" />
          ))}
        </div>
      </div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Skeleton className="h-48 rounded-lg" />
        <Skeleton className="h-48 rounded-lg" />
      </div>
    </div>
  );
}
