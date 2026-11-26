"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Book,
  Check,
  CreditCard,
  FileText,
  Loader2,
  PlayCircle,
  Trophy,
  X,
} from "lucide-react";
import { toast } from "sonner";

import { useOrder, useRetryPayment } from "@/lib/hooks/orders";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import { paymentMethodLabel } from "@/lib/payment-method";
import { payWithSnap } from "@/lib/snap";
import { ApiError } from "@/lib/api";
import { digitalConflictMessage } from "@/lib/digital-conflict";
import type { Order, OrderItem, OrderStatus } from "@/lib/types";

import { OrderStatusBadge } from "@/components/orders/OrderStatusBadge";
import { InvoiceDialog } from "@/components/orders/InvoiceDialog";
import { ShippingInfo } from "@/components/orders/ShippingInfo";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHead } from "@/components/shell/PageHead";
import { cn } from "@/lib/utils";
import { buildTimeline, type TimelineStep } from "./timeline";
import { PaymentCountdown } from "./payment-countdown";

const TYPE_ICON = {
  book: Book,
  course: PlayCircle,
  package: Trophy,
} as const;

const TYPE_TONE = {
  book: "bg-gold-50 text-gold-700",
  course: "bg-teal-50 text-teal-700",
  package: "bg-ember-50 text-ember-700",
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
    return <p className="text-sm text-ink-600">{t("order_no_items")}</p>;
  }
  return (
    <ul className="flex flex-col gap-3">
      {items.map((it) => {
        const Icon = TYPE_ICON[it.product_type as keyof typeof TYPE_ICON] ?? Book;
        const tone = TYPE_TONE[it.product_type as keyof typeof TYPE_TONE] ?? TYPE_TONE.book;
        const lineTotal = it.jumlah ?? it.unit_price * it.qty;
        return (
          <li key={it.id} className="flex items-center gap-3 text-sm">
            <span className={cn("grid size-[38px] shrink-0 place-items-center rounded-xl", tone)}>
              <Icon className="size-[18px]" />
            </span>
            <div className="flex min-w-0 flex-1 flex-col">
              <span className="truncate font-semibold text-ink-900">{it.name}</span>
              <span className="text-xs text-ink-600">
                {formatRupiah(it.unit_price)}
                {it.qty > 1 ? ` × ${it.qty}` : ""}
              </span>
            </div>
            <b className="font-bold text-ink-900 tabular-nums">{formatRupiah(lineTotal)}</b>
          </li>
        );
      })}
    </ul>
  );
}

function Timeline({ steps }: { steps: TimelineStep[] }) {
  return (
    <ol className="flex flex-col">
      {steps.map((s, i) => {
        const isLast = i === steps.length - 1;
        const Icon = s.cancelled ? X : Check;
        const nodeClass = s.cancelled
          ? "bg-danger-600 text-white"
          : isLast
            ? "bg-ember-600 text-white shadow-[0_0_0_5px_var(--color-ember-50)]"
            : "bg-teal-600 text-white";
        // The sequence runs in the order the order moved, so the eye arrives
        // at the newest step last — where the parcel is now.
        const delay = `${i * 90}ms`;
        return (
          <li
            key={s.key}
            className={cn("tl-step relative grid grid-cols-[34px_1fr_auto] gap-3", !isLast && "pb-[18px]")}
            style={{ animationDelay: delay }}
          >
            {!isLast && (
              <span
                className="tl-rail absolute top-[34px] bottom-0 left-4 w-0.5 bg-teal-600"
                style={{ animationDelay: delay }}
                aria-hidden
              />
            )}
            <span
              className={cn("tl-dot z-10 grid size-[34px] place-items-center rounded-xl", nodeClass)}
              style={{ animationDelay: delay }}
            >
              <Icon className="size-4" strokeWidth={2.5} />
            </span>
            <div className="flex min-w-0 flex-col justify-center">
              <b className={cn("text-sm font-bold", s.cancelled ? "text-danger" : "text-ink-900")}>{s.label}</b>
            </div>
            {s.at && <time className="self-center text-xs whitespace-nowrap text-ink-600">{formatDate(s.at)}</time>}
          </li>
        );
      })}
    </ol>
  );
}

function PaymentInfo({ order, t, lang }: { order: Order; t: (key: any) => string; lang: string }) {
  const rows: { labelKey: string; value: string }[] = [];
  if (order.payment_method) {
    rows.push({ labelKey: "order_payment_method", value: paymentMethodLabel(order.payment_method, lang) });
  }
  if (order.gateway_ref) {
    rows.push({ labelKey: "order_gateway_ref", value: order.gateway_ref });
  }
  if (order.payment_expires_at) {
    rows.push({ labelKey: "order_valid_until", value: formatDate(order.payment_expires_at) });
  }
  if (rows.length === 0) return null;
  return (
    <dl className="flex flex-col gap-2 text-sm">
      {rows.map((r) => (
        <div key={r.labelKey} className="flex items-start justify-between gap-3">
          <dt className="text-ink-600">{t(r.labelKey)}</dt>
          <dd className="text-right font-medium text-ink-900 break-all">{r.value}</dd>
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

function retryErrorMessage(err: unknown, t: (key: I18nKey) => string): string {
  if (!(err instanceof ApiError)) return t("order_retry_failed_desc");
  if (err.code === "payment_expired") return t("order_retry_expired");
  if (err.code === "payment_deadline_too_close") return t("order_retry_deadline_too_close");
  // A stale retry hits the order lock guard, not the admin-confirm "not payable" code.
  if (err.code === "order_not_editable") return t("order_retry_not_editable");
  return err.message;
}

export default function OrderDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { t, lang } = useTranslation();
  const { id } = use(params);
  const router = useRouter();
  const { data: order, isLoading, isError, error, refetch } = useOrder(id);
  const retry = useRetryPayment();
  const [retrying, setRetrying] = useState(false);
  const [paymentUrl, setPaymentUrl] = useState<string | null>(null);
  const [expired, setExpired] = useState(false);
  const [invoiceOpen, setInvoiceOpen] = useState(false);
  const handleExpired = useCallback(() => setExpired(true), []);
  const isPaymentPending = order?.status === "payment_pending";

  useEffect(() => {
    if (!expired || !isPaymentPending) return;
    const id = setInterval(() => refetch(), 5000);
    return () => clearInterval(id);
  }, [expired, isPaymentPending, refetch]);

  const busy = useRef(false);
  const finish = () => {
    busy.current = false;
    setRetrying(false);
  };

  const handleRetry = () => {
    if (!order || busy.current || retry.isPending) return;
    busy.current = true;
    setRetrying(true);
    retry.mutate(order.id, {
      onSuccess: async (res) => {
        if (res.payment_url) setPaymentUrl(res.payment_url);
        try {
          const path = await payWithSnap({
            snapToken: res.snap_token,
            paymentUrl: res.payment_url,
            onFinished: (event) => {
              finish();
              if (event === "error") {
                toast.error(t("payment_failed"));
                return;
              }
              refetch();
              if (event === "success") toast.info(t("payment_received"));
              if (event === "pending") toast.info(t("payment_pending"));
            },
          });
          if (path !== "popup") finish();
          if (path === "new-tab") toast.info(t("payment_new_tab"));
        } catch {
          finish();
          toast.error(t("order_snap_unavailable"));
        }
      },
      onError: (err) => {
        finish();
        const msg = digitalConflictMessage(err, t) ?? retryErrorMessage(err, t);
        toast.error(t("order_retry_failed_title"), { description: msg });
      },
    });
  };

  if (isLoading) return <DetailSkeleton />;

  if (isError || !order) {
    return (
      <Card className="gap-0 border-danger/30 bg-danger-bg px-5 py-4 text-sm text-danger">
        <p>{t("orders_load_failed")} {(error as Error)?.message}</p>
        <button onClick={() => refetch()} className="mt-2 w-fit underline">
          {t("retry")}
        </button>
      </Card>
    );
  }

  const timeline = buildTimeline(order, t);
  const canViewInvoice = order.status !== "cancelled" && order.status !== "payment_expired";

  return (
    <>
      <PageHead
        back={{ href: "/orders", label: t("order_all_orders") }}
        title={t("order_title").replace("{id}", order.id.slice(-8))}
        sub={t("order_created_at").replace("{date}", formatDate(order.created_at))}
        actions={
          canViewInvoice && (
            <Button variant="outline" onClick={() => setInvoiceOpen(true)}>
              <FileText className="size-4" />
              {t("invoice_view")}
            </Button>
          )
        }
      />

      {isPaymentPending && (
        <section className="flex flex-wrap items-center gap-5 rounded-[30px] bg-gold-50 p-6 text-gold-700">
          <div className="min-w-0 flex-1 basis-64 text-sm">
            <p className="font-mono text-[11px] font-semibold tracking-[0.12em] uppercase">
              {t("order_payment_pending_title")}
            </p>
            <p className="mt-1 text-[30px] leading-tight font-extrabold text-ink-900 [font-family:var(--font-display)]">
              {formatRupiah(order.total)}
            </p>
            <p className="mt-1 text-ink-700">
              {t("order_payment_pending_desc").replace("{deadline}", order.payment_expires_at ? `${t("order_pay_before")}${formatDate(order.payment_expires_at)}` : "")}
            </p>
            {order.payment_expires_at && (
              <PaymentCountdown expiresAt={order.payment_expires_at} onExpired={handleExpired} />
            )}
            {paymentUrl && !expired && (
              <div className="mt-3">
                <p className="text-ink-700">{t("order_payment_link_hint")}</p>
                <a
                  href={paymentUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-info underline underline-offset-2"
                >
                  {t("order_payment_link")}
                </a>
              </div>
            )}
          </div>
          {!expired && (
            <Button size="lg" onClick={handleRetry} disabled={retrying || retry.isPending} className="shrink-0">
              {retrying || retry.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <CreditCard className="size-4" />
              )}
              {t("order_continue_payment")}
            </Button>
          )}
        </section>
      )}

      <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <Card className="gap-5 p-6">
          <h2 className="type-h2 text-ink-900">{t("cart_order_summary")}</h2>
          <OrderItems items={order.items ?? []} t={t} />

          <dl className="flex flex-col gap-2 text-sm">
            {summaryRows(t).map(({ key, labelKey }) => {
              const val = (order[key] as number) ?? 0;
              return (
                <div key={key} className="flex items-center justify-between gap-3">
                  <dt className="text-ink-600">{t(labelKey)}</dt>
                  <dd className="font-medium text-ink-900">{formatRupiah(val)}</dd>
                </div>
              );
            })}
            <div className="mt-1 flex items-center justify-between gap-3 border-t border-dashed border-line pt-3">
              <dt className="text-[19px] font-extrabold text-ink-900 [font-family:var(--font-display)]">{t("order_total")}</dt>
              <dd className="text-[19px] font-extrabold text-ink-900 [font-family:var(--font-display)]">{formatRupiah(order.total)}</dd>
            </div>
          </dl>

          <ShippingInfo order={order} />

          <div>
            <h3 className="mb-2 type-h3 text-ink-900">{t("order_payment_info")}</h3>
            <PaymentInfo order={order} t={t} lang={lang} />
          </div>
        </Card>

        <Card className="gap-0 p-6">
          <div className="mb-3.5 flex flex-wrap items-center justify-between gap-3">
            <h2 className="type-h2 text-ink-900">{t("order_status_history")}</h2>
            <OrderStatusBadge status={order.status as OrderStatus} />
          </div>
          <Timeline steps={timeline} />
        </Card>
      </div>
      <InvoiceDialog order={order} open={invoiceOpen} onOpenChange={setInvoiceOpen} />
    </>
  );
}

function DetailSkeleton() {
  return (
    <>
      <Skeleton className="h-4 w-28" />
      <Skeleton className="h-8 w-48" />
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <Skeleton className="h-72 w-full rounded-[22px]" />
        <Skeleton className="h-48 w-full rounded-[22px]" />
      </div>
    </>
  );
}
