"use client";

import Link from "next/link";
import { AlertCircle, BookOpen } from "lucide-react";
import { useMyCourses } from "@/lib/hooks/courses";
import { useTranslation } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHead } from "@/components/shell/PageHead";
import { CourseCard } from "@/components/courses/CourseCard";

const COURSE_COVERS = [
  "radial-gradient(120% 110% at 100% 0%, #6e7cee, #3d4ddb)",
  "radial-gradient(120% 110% at 100% 0%, #4cc3b2, #1e978a)",
  "radial-gradient(120% 110% at 100% 0%, #6a9bef, #2f6fde)",
  "radial-gradient(120% 110% at 100% 0%, #e07aa0, #c23b6b)",
];

const GRID = "grid grid-cols-[repeat(auto-fill,minmax(255px,1fr))] gap-[18px]";

export default function CoursesPage() {
  const { t } = useTranslation();
  const { data: sessions, isLoading, isError, error, refetch } = useMyCourses();

  return (
    <>
      <PageHead
        title={t("nav_courses")}
        sub={t("course_subtitle")}
        actions={
          <Button asChild variant="dark">
            <Link href="/catalog">{t("stu_home_learn_add")}</Link>
          </Button>
        }
      />

      {isError && (
        <Card className="gap-0 border-danger/30 bg-danger-bg px-5 py-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="size-5 text-danger" />
            <div className="flex-1 text-sm text-ink-700">
              {t("course_load_failed")}
              {error instanceof Error && error.message ? ` ${error.message}` : ""}
            </div>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        </Card>
      )}

      {isLoading ? (
        <CoursesSkeleton />
      ) : sessions && sessions.length > 0 ? (
        <div className={GRID}>
          {sessions.map((session, i) => (
            <CourseCard
              key={session.id}
              session={session}
              gradient={COURSE_COVERS[i % COURSE_COVERS.length]}
            />
          ))}
        </div>
      ) : (
        !isError && <EmptyCourses />
      )}
    </>
  );
}

function EmptyCourses() {
  const { t } = useTranslation();
  return (
    <EmptyState
      framed
      icon={BookOpen}
      title={t("dash_no_courses")}
      text={t("dash_no_courses_desc")}
      action={
        <Button asChild>
          <Link href="/catalog">{t("dash_open_catalog")}</Link>
        </Button>
      }
    />
  );
}

function CoursesSkeleton() {
  return (
    <div className={GRID}>
      {Array.from({ length: 3 }).map((_, i) => (
        <Skeleton key={i} className="h-64 rounded-[22px]" />
      ))}
    </div>
  );
}
