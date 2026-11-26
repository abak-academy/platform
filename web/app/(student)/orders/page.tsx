"use client";

import Link from "next/link";
import { BookOpen, ChevronRight, Package, Receipt, Trophy } from "lucide-react";
import { useOrders } from "@/lib/hooks/orders";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type { Order } from "@/lib/types";
import { OrderStatusBadge } from "@/components/orders/OrderStatusBadge";
import { PageHead } from "@/components/shell/PageHead";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";

function formatDate(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat("id-ID", {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(d);
}

function BillingEmpty() {
  const { t } = useTranslation();
  return (
    <EmptyState
      framed
      icon={Receipt}
      title={t("billing_no_tx_title")}
      text={t("billing_no_tx_desc")}
      action={
        <Button asChild>
          <Link href="/catalog">{t("billing_browse_store")}</Link>
        </Button>
      }
    />
  );
}

function thumbOf(type?: string) {
  const k = (type ?? "").toLowerCase();
  if (k.includes("course")) return { Icon: BookOpen, tone: "bg-teal-50 text-teal-700" };
  if (k.includes("exam") || k.includes("event")) return { Icon: Trophy, tone: "bg-ember-50 text-ember-700" };
  return { Icon: Package, tone: "bg-gold-50 text-gold-700" };
}

function OrdersList({ orders }: { orders: Order[] }) {
  const { t } = useTranslation();
  if (orders.length === 0) {
    return <BillingEmpty />;
  }

  const sorted = [...orders].sort((a, b) => {
    const ta = a.created_at ? Date.parse(a.created_at) : 0;
    const tb = b.created_at ? Date.parse(b.created_at) : 0;
    return tb - ta;
  });

  return (
    <ul className="flex flex-col gap-4">
      {sorted.map((o) => {
        const { Icon, tone } = thumbOf(o.items?.[0]?.product_type);
        const extra = (o.items?.length ?? 0) - 1;
        return (
          <li key={o.id}>
            <Link
              href={`/orders/${o.id}`}
              className="group flex items-center gap-3.5 rounded-[var(--radius-card)] border border-line bg-card px-[18px] py-3.5 shadow-card transition-transform duration-200 hover:translate-x-1 motion-reduce:hover:translate-x-0"
            >
              <span className={`grid size-[38px] shrink-0 place-items-center rounded-xl ${tone}`}>
                <Icon className="size-[18px]" aria-hidden />
              </span>
              <div className="flex min-w-0 flex-1 flex-col">
                <span className="truncate type-label text-ink-900">
                  {o.items?.[0]?.name ?? `#${o.id.slice(-8)}`}
                  {extra > 0 && <span className="font-medium text-ink-600"> +{extra}</span>}
                </span>
                <small className="truncate text-xs text-ink-600">
                  #{o.id.slice(-8)} · {formatDate(o.created_at)}
                </small>
              </div>
              <b className="shrink-0 font-extrabold text-ink-900 tabular-nums [font-family:var(--font-display)]">
                {formatRupiah(o.total)}
              </b>
              <OrderStatusBadge status={o.status} />
              <ChevronRight className="size-4 shrink-0 text-ink-600" aria-hidden />
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

function OrdersSkeleton() {
  return (
    <ul className="flex flex-col gap-4">
      {Array.from({ length: 5 }).map((_, i) => (
        <li key={i}>
          <Skeleton className="h-[68px] w-full rounded-[22px]" />
        </li>
      ))}
    </ul>
  );
}

export default function OrdersPage() {
  const { t } = useTranslation();
  const { data, isLoading, isError, error, refetch } = useOrders();

  return (
    <>
      <PageHead title={t("nav_billing")} sub={t("billing_subtitle")} />

      {isError ? (
        <Card className="gap-0 border-danger/30 bg-danger-bg p-4 text-sm text-danger">
          <p>{t("orders_load_failed")} {(error as Error)?.message}</p>
          <button onClick={() => refetch()} className="mt-2 w-fit underline">
            {t("retry")}
          </button>
        </Card>
      ) : isLoading ? (
        <OrdersSkeleton />
      ) : (
        <OrdersList orders={data ?? []} />
      )}
    </>
  );
}
