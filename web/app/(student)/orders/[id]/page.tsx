"use client";

import { use, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Book,
  CheckCircle2,
  Clock,
  CreditCard,
  Loader2,
  PlayCircle,
  Receipt,
  Trophy,
  XCircle,
} from "lucide-react";
import { toast } from "sonner";

import { useOrder, useRetryPayment } from "@/lib/hooks/orders";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import { ApiError } from "@/lib/api";
import type { Order, OrderItem, OrderStatus } from "@/lib/types";

import { OrderStatusBadge } from "@/components/orders/OrderStatusBadge";
import { ShippingInfo } from "@/components/orders/ShippingInfo";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { buildTimeline, type TimelineStep } from "./timeline";

const TYPE_ICON = {
  book: Book,
  course: PlayCircle,
  package: Trophy,
} as const;

function formatDate(iso?: string): string {
  if (!iso) return "—";
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


function OrderItems({ items, t }: { items: OrderItem[]; t: (key: any) => string }) {
  if (items.length === 0) {
    return <p className="text-sm text-ink-500">{t("order_no_items")}</p>;
  }
  return (
    <ul className="flex flex-col divide-y divide-line">
      {items.map((it) => {
        const Icon = TYPE_ICON[it.product_type as keyof typeof TYPE_ICON] ?? Book;
        const lineTotal = it.jumlah ?? it.unit_price * it.qty;
        return (
          <li key={it.id} className="flex items-center gap-4 py-3 first:pt-0 last:pb-0">
            <div className="flex size-10 shrink-0 items-center justify-center rounded-md bg-paper">
              <Icon className="size-5 text-ink-400" strokeWidth={1.5} />
            </div>
            <div className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-sm font-semibold text-ink-900">{it.name}</span>
              <span className="text-xs text-ink-500">
                {formatRupiah(it.unit_price)}
                {it.qty > 1 ? ` × ${it.qty}` : ""}
              </span>
            </div>
            <span className="font-serif text-sm font-bold text-ink-900">
              {formatRupiah(lineTotal)}
            </span>
          </li>
        );
      })}
    </ul>
  );
}

function Timeline({ steps }: { steps: TimelineStep[] }) {
  return (
    <ol className="relative flex flex-col gap-4 pl-6">
      {steps.map((s, i) => {
        const isLast = i === steps.length - 1;
        const Icon = s.cancelled ? XCircle : CheckCircle2;
        const dotClass = s.cancelled
          ? "text-danger bg-danger-bg"
          : s.reached
            ? "text-success bg-success-bg"
            : "text-ink-400 bg-line-2";
        // The sequence runs in the order the order moved, so the eye arrives
        // at the newest step last — where the parcel is now.
        const delay = `${i * 90}ms`;
        return (
          <li key={s.key} className="tl-step relative" style={{ animationDelay: delay }}>
            {!isLast && (
              <span
                className="tl-rail absolute -left-[17px] top-4 h-[calc(100%+2px)] w-px bg-line"
                style={{ animationDelay: delay }}
                aria-hidden
              />
            )}
            <span
              className={cn(
                "tl-dot absolute -left-6 top-0.5 flex size-3.5 items-center justify-center rounded-full",
                isLast && !s.cancelled && "tl-current",
                dotClass,
              )}
              style={{ animationDelay: delay }}
            >
              <Icon className="size-3.5" strokeWidth={2.5} />
            </span>
            <div className="flex flex-col">
              <span
                className={cn(
                  "text-sm font-medium",
                  s.cancelled ? "text-danger" : s.reached ? "text-ink-900" : "text-ink-500",
                )}
              >
                {s.label}
              </span>
              {s.at && <span className="text-xs text-ink-500">{formatDate(s.at)}</span>}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

function PaymentInfo({ order, t }: { order: Order; t: (key: any) => string }) {
  const rows: { labelKey: string; value: string }[] = [];
  if (order.payment_method) {
    rows.push({ labelKey: "order_payment_method", value: order.payment_method });
  }
  if (order.gateway_ref) {
    rows.push({ labelKey: "order_gateway_ref", value: order.gateway_ref });
  }
  if (order.payment_expires_at) {
    rows.push({ labelKey: "order_valid_until", value: formatDate(order.payment_expires_at) });
  }
  if (order.invoice_url) {
    rows.push({ labelKey: "order_invoice", value: order.invoice_url });
  }
  if (rows.length === 0) return null;
  return (
    <dl className="flex flex-col gap-2 text-sm">
      {rows.map((r) => (
        <div key={r.labelKey} className="flex items-start justify-between gap-3">
          <dt className="text-ink-500">{t(r.labelKey)}</dt>
          <dd className="text-right font-medium text-ink-900 break-all">
            {r.labelKey === "order_invoice" && r.value.startsWith("http") ? (
              <a
                href={r.value}
                target="_blank"
                rel="noreferrer"
                className="text-info underline underline-offset-2"
              >
                {t("order_view_invoice")}
              </a>
            ) : (
              r.value
            )}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function summaryRows(t: (key: any) => string): { key: keyof Order; labelKey: any }[] {
  return [
    { key: "subtotal", labelKey: "order_subtotal" },
    { key: "discount", labelKey: "order_discount" },
    { key: "shipping_cost", labelKey: "order_shipping" },
  ];
}

export default function OrderDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { t } = useTranslation();
  const { id } = use(params);
  const router = useRouter();
  const { data: order, isLoading, isError, error, refetch } = useOrder(id);
  const retry = useRetryPayment();
  const [retrying, setRetrying] = useState(false);

  const handleRetry = () => {
    if (!order) return;
    setRetrying(true);
    retry.mutate(order.id, {
      onSuccess: (res) => {
        setRetrying(false);
        if (res.payment_url) {
          window.open(res.payment_url, "_blank");
          toast.info("Selesaikan pembayaran di tab baru, lalu refresh halaman ini.");
        } else {
          toast.error(t("order_snap_unavailable"));
        }
      },
      onError: (err) => {
        setRetrying(false);
        const msg = err instanceof ApiError ? err.message : t("order_retry_failed_desc");
        toast.error(t("order_retry_failed_title"), { description: msg });
      },
    });
  };

  if (isLoading) return <DetailSkeleton />;

  if (isError || !order) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 md:px-6">
        <div className="rounded-lg border border-danger/30 bg-danger-bg px-5 py-4 text-sm text-danger">
          <p>{t("orders_load_failed")} {(error as Error)?.message}</p>
          <button onClick={() => refetch()} className="mt-2 underline">
            {t("retry")}
          </button>
        </div>
      </div>
    );
  }

  const isPaymentPending = order.status === "payment_pending";
  const timeline = buildTimeline(order, t);

  return (
    <div className="mx-auto max-w-5xl px-4 py-6 md:px-6 md:py-10">
      <Button asChild variant="ghost" size="sm" className="mb-4">
        <Link href="/orders">
          <ArrowLeft className="size-4" />
          {t("order_all_orders")}
        </Link>
      </Button>

      <header className="mb-6 flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <Receipt className="size-5 text-ink-400" />
            <h1 className="font-serif text-2xl font-bold text-ink-900 md:text-3xl">
              {t("order_title").replace("{id}", order.id.slice(-8))}
            </h1>
          </div>
          <span className="text-xs text-ink-500">{t("order_created_at").replace("{date}", formatDate(order.created_at))}</span>
        </div>
        <OrderStatusBadge status={order.status as OrderStatus} className="text-sm" />
      </header>

      {isPaymentPending && (
        <Card className="mb-6 border-warn/30 bg-warn-bg p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-start gap-3">
              <Clock className="mt-0.5 size-5 text-warn" />
              <div className="text-sm">
                <p className="font-semibold text-ink-900">{t("order_payment_pending_title")}</p>
                <p className="text-ink-600">
                  {t("order_payment_pending_desc").replace("{deadline}", order.payment_expires_at ? `${t("order_pay_before")}${formatDate(order.payment_expires_at)}` : "")}
                </p>
              </div>
            </div>
            <Button onClick={handleRetry} disabled={retrying || retry.isPending} className="shrink-0">
              {retrying || retry.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <CreditCard className="size-4" />
              )}
              {t("order_continue_payment")}
            </Button>
          </div>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[1fr_380px] md:gap-8">
        <div className="flex flex-col gap-6">
          <section>
            <h2 className="mb-3 font-serif text-lg font-semibold text-ink-900">{t("order_items_section")}</h2>
            <Card className="p-5">
              <OrderItems items={order.items ?? []} t={t} />
            </Card>
          </section>

          <section>
            <h2 className="mb-3 font-serif text-lg font-semibold text-ink-900">{t("order_status_history")}</h2>
            <Card className="p-5">
              <Timeline steps={timeline} />
            </Card>
          </section>
        </div>

        <aside className="md:sticky md:top-6 md:self-start">
          <Card className="p-5">
            <h2 className="mb-3 font-serif text-base font-semibold text-ink-900">{t("order_summary")}</h2>
            <dl className="flex flex-col gap-2 text-sm">
              {summaryRows(t).map(({ key, labelKey }) => {
                const val = (order[key] as number) ?? 0;
                return (
                  <div key={key} className="flex items-center justify-between gap-3">
                    <dt className="text-ink-500">{t(labelKey)}</dt>
                    <dd className="font-medium text-ink-900">{formatRupiah(val)}</dd>
                  </div>
                );
              })}
              <div className="my-1 h-px bg-line" />
              <div className="flex items-center justify-between gap-3">
                <dt className="font-semibold text-ink-900">{t("order_total")}</dt>
                <dd className="font-serif text-lg font-bold text-success">
                  {formatRupiah(order.total)}
                </dd>
              </div>
            </dl>

            <ShippingInfo order={order} />

            <div className="my-4 h-px bg-line" />

            <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-ink-500">
              {t("order_payment_info")}
            </h3>
            <PaymentInfo order={order} t={t} />
          </Card>
        </aside>
      </div>
    </div>
  );
}

function DetailSkeleton() {
  return (
    <div className="mx-auto max-w-5xl px-4 py-6 md:px-6 md:py-10">
      <Skeleton className="mb-4 h-8 w-28" />
      <div className="mb-6 flex items-center gap-3">
        <Skeleton className="h-7 w-40" />
        <Skeleton className="h-6 w-28" />
      </div>
      <div className="grid grid-cols-1 gap-6 md:grid-cols-[1fr_380px] md:gap-8">
        <div className="flex flex-col gap-6">
          <Skeleton className="h-40 w-full rounded-lg" />
          <Skeleton className="h-48 w-full rounded-lg" />
        </div>
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    </div>
  );
}