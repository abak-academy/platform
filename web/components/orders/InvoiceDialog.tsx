"use client";

import { useRef, useState } from "react";
import { Download, Loader2, Printer } from "lucide-react";
import { toast } from "sonner";

import { AbakLogo } from "@/components/brand/AbakLogo";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { savePageAsPdf } from "@/lib/capture-pdf";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { paymentMethodLabel } from "@/lib/payment-method";
import { calculateTotalPhysicalWeight, hasPhysicalItems, isPhysicalType } from "@/lib/shipping";
import type { Order, ProductType } from "@/lib/types";
import { useAuthStore } from "@/stores/auth";

const formatDate = (iso: string | undefined, lang: string) =>
  iso
    ? new Intl.DateTimeFormat(lang === "id" ? "id-ID" : "en-US", { day: "numeric", month: "short", year: "numeric", timeZone: "Asia/Jakarta" }).format(new Date(iso))
    : "—";

export interface InvoiceDialogProps {
  order: Order;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function InvoiceDialog({ order, open, onOpenChange }: InvoiceDialogProps) {
  const { t, lang } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const paperRef = useRef<HTMLElement>(null);
  const [generating, setGenerating] = useState(false);

  const items = order.items ?? [];
  const physical = hasPhysicalItems(items);
  const addr = order.shipping_address ?? {};
  const recipient = [addr.penerima, addr.telepon].filter(Boolean).join(" · ");
  const address = [addr.alamat, addr.kode_pos].filter(Boolean).join(" ");
  const courier = [order.selected_courier, order.selected_service].filter(Boolean).join(" · ");
  const goods = items.filter((it) => isPhysicalType(it.product_type as ProductType));
  const goodsQty = goods.reduce((n, it) => n + it.qty, 0);
  const grams = calculateTotalPhysicalWeight(items);
  const weight = grams
    ? `${(grams / 1000).toLocaleString(lang === "id" ? "id-ID" : "en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })} kg`
    : "—";
  const paid = Boolean(order.paid_at);

  const handleDownload = async () => {
    const node = paperRef.current;
    if (!node || generating) return;
    setGenerating(true);
    try {
      await savePageAsPdf(node, `Invoice-${order.id.slice(-8)}.pdf`, getComputedStyle(node).backgroundColor);
      toast.success(t("invoice_downloaded"));
    } catch {
      toast.error(t("invoice_failed"));
    } finally {
      setGenerating(false);
    }
  };

  const sumRow = (label: string, value: string) => (
    <div className="flex justify-between gap-6">
      <span>{label}</span>
      <b className="font-semibold">{value}</b>
    </div>
  );

  const shipRow = (label: string, value: string) => (
    <>
      <dt className="text-ink-500">{label}</dt>
      <dd>{value}</dd>
    </>
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="wide">
        <style>{"@media print{@page{size:A4;margin:0}}"}</style>
        <DialogTitle className="sr-only">{t("invoice_title")}</DialogTitle>
        <div className="invoice-stage -mx-2 rounded-2xl bg-surface-sunken p-3.5">
          {/* shortcut: paper is one fixed A4 page in print CSS, long orders clip; upgrade when orders exceed ~12 items */}
          <article
            ref={paperRef}
            className="invoice-paper mx-auto flex aspect-[210/297] w-full max-w-[640px] flex-col rounded-md p-8 text-[12.5px] leading-normal shadow-pop"
          >
            <header className="flex items-start justify-between border-b-2 border-ink-900 pb-4">
              <div className="flex items-center gap-2.5">
                <AbakLogo size={38} primary="currentColor" />
                <span className="flex flex-col">
                  <b className="text-[15px] font-extrabold">Abak Academy</b>
                  <small className="text-[11px] text-ink-500">{window.location.host}</small>
                </span>
              </div>
              <div className="text-right">
                <h3 className="text-[22px] font-extrabold leading-none tracking-[.08em]">{t("invoice_title")}</h3>
                <small className="font-mono text-[11px] text-ink-500">INV/{order.id}</small>
              </div>
            </header>

            <div className="flex justify-between gap-6 py-4">
              <div className="flex flex-col gap-0.5">
                <small className="text-[11px] text-ink-500">{t("invoice_bill_to")}</small>
                <b className="text-[13.5px]">{user?.name ?? order.student_name}</b>
                {user?.email && <span>{user.email}</span>}
              </div>
              <div className="flex flex-col items-end gap-2 text-right">
                <span className="flex flex-col items-end">
                  <small className="text-[11px] text-ink-500">{t("invoice_order_date")}</small>
                  {formatDate(order.created_at, lang)}
                </span>
                <b
                  className={`inline-block rounded-md border-2 px-2.5 py-0.5 text-[11px] font-extrabold tracking-[.1em] ${paid ? "border-success text-success" : "border-danger text-danger"}`}
                >
                  {paid ? t("invoice_paid") : t("invoice_unpaid")}
                </b>
              </div>
            </div>

            <table className="w-full table-fixed border-collapse">
              <colgroup>
                <col />
                <col className="w-12" />
                <col className="w-24" />
                <col className="w-28" />
              </colgroup>
              <thead>
                <tr className="bg-line-2 text-[11px] font-bold uppercase tracking-[.06em] text-ink-500">
                  <th className="p-1.5 text-left">{t("invoice_col_item")}</th>
                  <th className="p-1.5 text-center">{t("invoice_col_qty")}</th>
                  <th className="p-1.5 text-right">{t("invoice_col_price")}</th>
                  <th className="p-1.5 text-right">{t("invoice_col_amount")}</th>
                </tr>
              </thead>
              <tbody>
                {items.map((it) => (
                  <tr key={it.id} className="border-b border-line">
                    <td className="break-words p-1.5 py-2.5">{it.name}</td>
                    <td className="p-1.5 text-center">{it.qty}</td>
                    <td className="whitespace-nowrap p-1.5 text-right">{formatRupiah(it.unit_price)}</td>
                    <td className="whitespace-nowrap p-1.5 text-right">{formatRupiah(it.jumlah ?? it.unit_price * it.qty)}</td>
                  </tr>
                ))}
              </tbody>
            </table>

            <div className="mt-3.5 ml-auto flex w-64 flex-col gap-1.5">
              {sumRow(t("invoice_subtotal"), formatRupiah(order.subtotal))}
              {order.discount > 0 && sumRow(t("invoice_discount"), `−${formatRupiah(order.discount)}`)}
              {physical && sumRow(t("invoice_shipping"), formatRupiah(order.shipping_cost ?? 0))}
              <div className="mt-1 flex justify-between gap-6 border-t-2 border-ink-900 pt-2 text-[15px] font-extrabold">
                <span>{t("invoice_total")}</span>
                <span>{formatRupiah(order.total)}</span>
              </div>
            </div>

            {physical && (
              <section data-testid="invoice-shipping" className="mt-5 ml-auto w-full max-w-[340px] rounded-md border border-line p-3.5">
                <b className="text-[11px] font-bold uppercase tracking-[.06em] text-ink-500">{t("invoice_ship_title")}</b>
                <dl className="mt-2 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                  {shipRow(t("invoice_ship_recipient"), recipient || "—")}
                  {shipRow(t("invoice_ship_address"), address || "—")}
                  {shipRow(t("invoice_ship_courier"), courier || "—")}
                  {shipRow(t("invoice_ship_resi"), order.tracking_number || t("invoice_ship_resi_pending"))}
                  {shipRow(t("invoice_ship_goods"), goods.map((it) => it.name).join(", "))}
                  {shipRow(t("invoice_ship_qty"), String(goodsQty))}
                  {shipRow(t("invoice_ship_weight"), weight)}
                </dl>
              </section>
            )}

            <footer className="mt-auto flex flex-col gap-1 border-t border-line pt-3 text-[11px] text-ink-500">
              <span>
                {t("invoice_payment_method")}: <b className="font-semibold text-ink-900">{paymentMethodLabel(order.payment_method, lang)}</b>
                {order.paid_at ? ` · ${formatDate(order.paid_at, lang)}` : ""}
              </span>
              <span className="font-semibold text-ink-700">{t("invoice_proof")}</span>
            </footer>
          </article>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => window.print()}>
            <Printer className="size-4" />
            {t("invoice_print")}
          </Button>
          <Button onClick={handleDownload} disabled={generating}>
            {generating ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}
            {generating ? t("invoice_generating") : t("invoice_download")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
