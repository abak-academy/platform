"use client";

import { AlertCircle } from "lucide-react";
import { useDashboard } from "@/lib/hooks/students";
import { useOrders } from "@/lib/hooks/orders";
import { useRegistrations } from "@/lib/hooks/exam";
import { useEventSeries } from "@/lib/hooks/event-series";
import { useProducts } from "@/lib/hooks/products";
import { useTranslation } from "@/lib/i18n";
import { heroEvents, pickShipment } from "@/lib/student-home";
import { useAuthStore } from "@/stores/auth";
import { Skeleton } from "@/components/ui/skeleton";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PageHead } from "@/components/shell/PageHead";
import { PendingBanner } from "@/components/dashboard/PendingBanner";
import { ShippingStrip } from "@/components/dashboard/ShippingStrip";
import { EventCarousel } from "@/components/dashboard/EventCarousel";
import { MyEventsCard } from "@/components/dashboard/MyEventsCard";
import { ContinueLearningCard } from "@/components/dashboard/ContinueLearningCard";
import { StudySummaryCard } from "@/components/dashboard/StudySummaryCard";
import { ShopPicks } from "@/components/dashboard/ShopPicks";
import { Landing } from "@/components/landing/Landing";
import { greetingKey } from "./home-greeting";

const ROW_A = "grid grid-cols-[minmax(0,1fr)] gap-4 min-[1200px]:grid-cols-[minmax(0,65fr)_minmax(0,35fr)]";
const ROW_B = "grid grid-cols-[minmax(0,1fr)] gap-4 min-[1200px]:grid-cols-[minmax(0,40fr)_minmax(0,60fr)]";

function firstName(name?: string): string {
  if (!name) return "";
  return name.trim().split(/\s+/)[0];
}

export default function Page() {
  const token = useAuthStore((s) => s.token);
  return token ? <Dashboard /> : <Landing />;
}

function Dashboard() {
  const { t, lang } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const { data, isLoading, isError, error, refetch } = useDashboard();
  const orders = useOrders();
  const registrations = useRegistrations();
  const eventSeries = useEventSeries(lang);
  const products = useProducts();
  const name = firstName(user?.name ?? user?.username ?? undefined);

  const now = Date.now();
  const regs = registrations.data ?? [];
  const shipment = pickShipment(orders.data ?? []);
  const slides = heroEvents(eventSeries.data ?? [], regs);
  const greeting = t(greetingKey(new Date(now).getHours()));
  const today = new Intl.DateTimeFormat(lang === "id" ? "id-ID" : "en-US", {
    weekday: "long",
    day: "numeric",
    month: "long",
  }).format(now);

  return (
    <>
      {isError && (
        <Card className="gap-0 border-danger/30 bg-danger-bg px-5 py-4">
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
        <div className="flex flex-col gap-5">
          <PageHead
            eyebrow={today}
            title={t("stu_home_greeting").replace("{greeting}", greeting).replace("{name}", name)}
            sub={t("stu_home_intro")}
          />

          {data.pending_order && (
            <PendingBanner
              id={data.pending_order.id}
              product={data.pending_order.product}
              amount={data.pending_order.amount}
            />
          )}

          {shipment && <ShippingStrip order={shipment} />}

          <div className={ROW_A}>
            {eventSeries.isLoading || registrations.isLoading ? (
              <Skeleton className="h-64 rounded-xl" data-testid="carousel-skeleton" />
            ) : (
              <EventCarousel slides={eventSeries.isError ? [] : slides} />
            )}
            <MyEventsCard registrations={regs} now={now} isLoading={registrations.isLoading} isError={registrations.isError} />
          </div>

          <div className={ROW_B}>
            <ContinueLearningCard courses={data.enrolled_courses} />
            <StudySummaryCard study={data.study_summary} />
          </div>

          {products.data && (
            <ShopPicks products={products.data} enrolledCourseIds={data.enrolled_courses.map((c) => c.id)} />
          )}
        </div>
      ) : null}
    </>
  );
}

function DashboardSkeleton() {
  return (
    <div className="flex flex-col gap-5" data-testid="dashboard-skeleton">
      <Skeleton className="h-16 w-72" />
      <div className={ROW_A}>
        <Skeleton className="h-64 rounded-xl" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
      <div className={ROW_B}>
        <Skeleton className="h-48 rounded-lg" />
        <Skeleton className="h-48 rounded-lg" />
      </div>
    </div>
  );
}
