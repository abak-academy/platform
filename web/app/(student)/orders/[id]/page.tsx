"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Book,
  Check,
  CheckCircle2,
  Copy,
  CreditCard,
  FileText,
  Loader2,
  PlayCircle,
  Receipt,
  Trophy,
  Truck,
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
import { hasPhysicalItems } from "@/lib/shipping";

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
  exam: Trophy,
} as const;

const TYPE_TONE = {
  book: "bg-gold-50 text-gold-700",
  course: "bg-teal-50 text-teal-700",
  package: "bg-ember-50 text-ember-700",
  exam: "bg-teal-50 text-teal-700",
} as const;

function formatShort(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" }).format(d);
}

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


// Prototype .mini-lines.
function OrderItems({ items, t }: { items: OrderItem[]; t: (key: any) => string }) {
  if (items.length === 0) {
    return <p className="mb-3.5 text-sm text-ink-600">{t("order_no_items")}</p>;
  }
  return (
    <ul className="mb-3.5 flex flex-col gap-3">
      {items.map((it) => {
        const Icon = TYPE_ICON[it.product_type as keyof typeof TYPE_ICON] ?? Book;
        const tone = TYPE_TONE[it.product_type as keyof typeof TYPE_TONE] ?? TYPE_TONE.book;
        const lineTotal = it.jumlah ?? it.unit_price * it.qty;
        return (
          <li key={it.id} className="flex items-center gap-3 text-[14px]">
            <span className={cn("grid size-[38px] shrink-0 place-items-center rounded-[12px]", tone)}>
              <Icon className="size-4" />
            </span>
            <span className="min-w-0 flex-1 text-ink-900">
              {it.name}
              {it.qty > 1 ? ` ×${it.qty}` : ""}
            </span>
            <b className="font-bold text-ink-900 tabular-nums">{formatRupiah(lineTotal)}</b>
          </li>
        );
      })}
    </ul>
  );
}

const STEP_ICON = { created: Receipt, checkout: Receipt, paid: Check, shipped: Truck, completed: CheckCircle2, cancelled: X } as const;

// Prototype .tl: reached steps teal, the next one ember, the rest muted.
function Timeline({ steps, detail, t }: { steps: TimelineStep[]; detail: (key: string) => string | undefined; t: (key: any) => string }) {
  const nowKey = steps.find((s) => !s.reached)?.key;
  return (
    <ol className="mb-3.5 flex flex-col">
      {steps.map((s, i) => {
        const isLast = i === steps.length - 1;
        const now = s.key === nowKey;
        const Icon = STEP_ICON[s.key as keyof typeof STEP_ICON] ?? Check;
        const nodeClass = s.cancelled
          ? "bg-danger-600 text-white"
          : s.reached
            ? "bg-teal-600 text-white"
            : now
              ? "bg-ember-600 text-white shadow-[0_0_0_5px_var(--color-ember-50)]"
              : "bg-surface-sunken text-ink-600";
        const note = s.reached ? detail(s.key) : t(`order_tl_hint_${s.key}`);
        const delay = `${i * 90}ms`;
        return (
          <li
            key={s.key}
            className={cn("tl-step relative grid grid-cols-[34px_1fr_auto] gap-3", !isLast && "pb-[18px]")}
            style={{ animationDelay: delay }}
          >
            {!isLast && (
              <span
                className={cn("tl-rail absolute top-[34px] bottom-0 left-4 w-0.5", s.reached ? "bg-teal-600" : "bg-line")}
                style={{ animationDelay: delay }}
                aria-hidden
              />
            )}
            <span
              className={cn("tl-dot relative z-10 grid size-[34px] place-items-center rounded-[12px]", nodeClass)}
              style={{ animationDelay: delay }}
            >
              <Icon className="size-4" />
            </span>
            <div className="min-w-0">
              <b
                className={cn(
                  "font-[family-name:var(--font-display)] text-[14px] leading-[normal] font-bold",
                  s.cancelled ? "text-danger" : s.reached || now ? "text-ink-900" : "text-ink-600",
                )}
              >
                {s.label}
              </b>
              {note && <p className="mt-0.5 text-[12.5px] text-ink-600">{note}</p>}
            </div>
            <time className="text-[12px] whitespace-nowrap text-ink-600">{s.at ? formatShort(s.at) : "—"}</time>
          </li>
        );
      })}
    </ol>
  );
}

function PaymentInfo({ order, t, lang }: { order: Order; t: (key: any) => string; lang: string }) {
  return (
    <p className="text-ink-600">
      <span>{order.payment_method ? paymentMethodLabel(order.payment_method, lang) : t("order_method_unset")}</span> · Midtrans
      {order.gateway_ref && <span className="break-all"> · {order.gateway_ref}</span>}
    </p>
  );
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
      <Card className="gap-0 border-danger/30 bg-danger-bg p-4 text-sm text-danger">
        <p>{t("orders_load_failed")} {(error as Error)?.message}</p>
        <button onClick={() => refetch()} className="mt-2 w-fit underline">
          {t("retry")}
        </button>
      </Card>
    );
  }

  const timeline = buildTimeline(order, t);
  const physical = hasPhysicalItems(order.items ?? []);
  const stepDetail = (key: string) =>
    key === "paid"
      ? order.payment_method && paymentMethodLabel(order.payment_method, lang)
      : key === "shipped"
        ? [order.selected_courier?.toUpperCase(), order.tracking_number].filter(Boolean).join(" · ") || undefined
        : key === "completed"
          ? t(physical ? "order_tl_done_physical" : "order_tl_done_digital")
          : undefined;
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
          <div className="min-w-0 flex-1 basis-64">
            <p className="mb-2.5 font-mono text-[11.5px] leading-[1.4] font-semibold tracking-[0.14em] text-ember-700 uppercase">
              {t("order_payment_pending_title")}
            </p>
            <h2 className="mb-1 font-[family-name:var(--font-display)] text-[30px] leading-[1.3] font-extrabold tracking-[-0.02em] text-ink-900 max-narrow:text-[24px]">
              {formatRupiah(order.total)}
            </h2>
            <p className="text-ink-700">
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

      <div className="grid grid-cols-1 items-start gap-4 min-[1025px]:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <Card className="block p-6">
          <h2 className="mb-3 type-h2 text-ink-900">{t("cart_order_summary")}</h2>
          <OrderItems items={order.items ?? []} t={t} />

          <dl className="mt-1.5 mb-4 flex flex-col gap-2 text-[14px]">
            <div className="flex justify-between">
              <dt>{t("order_subtotal")}</dt>
              <dd className="font-bold">{formatRupiah(order.subtotal ?? 0)}</dd>
            </div>
            {order.discount > 0 && (
              <div className="flex justify-between text-teal-700">
                <dt>{t("order_discount")}</dt>
                <dd className="font-bold">−{formatRupiah(order.discount)}</dd>
              </div>
            )}
            {(physical || order.shipping_cost > 0) && (
              <div className="flex justify-between">
                <dt>{t("order_shipping")}</dt>
                <dd className="font-bold">{order.shipping_cost ? formatRupiah(order.shipping_cost) : "—"}</dd>
              </div>
            )}
            <div className="flex justify-between border-t border-dashed border-line pt-3 font-[family-name:var(--font-display)] text-[19px] leading-[normal] font-extrabold">
              <dt>{t("order_total")}</dt>
              <dd className="font-black">{formatRupiah(order.total)}</dd>
            </div>
          </dl>

          <ShippingInfo order={order} />

          <h3 className="font-[family-name:var(--font-display)] text-base leading-[1.35] font-bold tracking-[-0.02em] text-ink-900">
            {t("order_payment_heading")}
          </h3>
          <PaymentInfo order={order} t={t} lang={lang} />
        </Card>

        <Card className="block p-6">
          <div className="mb-3.5 flex flex-wrap items-center justify-between gap-3">
            <h2 className="type-h2 text-ink-900">{t("order_status_history")}</h2>
            <OrderStatusBadge status={order.status as OrderStatus} />
          </div>
          <Timeline steps={timeline} detail={stepDetail} t={t} />
          {order.tracking_number && (
            <div className="mb-3 flex items-center gap-2.5 rounded-[14px] bg-surface-sunken p-3 text-[13px]">
              <Truck className="size-4 shrink-0" aria-hidden />
              <span>
                {t("order_tracking")} <b className="tabular-nums">{order.tracking_number}</b>
              </span>
              <Button
                variant="ghost"
                size="sm"
                className="ml-auto"
                onClick={() => {
                  void navigator.clipboard?.writeText(order.tracking_number!);
                  toast.success(t("order_tracking_copied"));
                }}
              >
                <Copy aria-hidden />
                {t("action_copy")}
              </Button>
            </div>
          )}
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
      <div className="grid grid-cols-1 gap-4 min-[1025px]:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <Skeleton className="h-72 w-full rounded-[22px]" />
        <Skeleton className="h-48 w-full rounded-[22px]" />
      </div>
    </>
  );
}
