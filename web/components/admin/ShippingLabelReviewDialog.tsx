"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { downloadShippingLabel, downloadShippingLabels, type LabelNotPrintable, type LabelSize } from "@/lib/hooks/shipping-label";
import { useTranslation } from "@/lib/i18n";
import { isShipmentFailure } from "@/lib/shipment-status";
import type { Order } from "@/lib/types";

export function ShippingLabelReviewDialog({ orders, onClose, mode = "bulk", focusTarget }: { orders: Order[]; onClose: () => void; mode?: "bulk" | "single"; focusTarget?: HTMLElement | null }) {
  const { t } = useTranslation();
  const [size, setSize] = useState<LabelSize>("100x150");
  const [freshReasons, setFreshReasons] = useState<LabelNotPrintable[]>([]);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<"changed" | "download" | null>(null);
  const inFlight = useRef(false);
  const dialogRef = useRef<HTMLDivElement>(null);
  useEffect(() => { if (pending) dialogRef.current?.focus(); }, [pending]);
  const returnFocus = useRef(focusTarget ?? (typeof document === "undefined" ? null : document.activeElement as HTMLElement));
  const reasonLabels = { not_found: t("labels_not_found"), no_tracking_number: t("labels_no_tracking_number"), shipment_failed: t("labels_shipment_failed") };
  const skipped = orders.flatMap((order) => {
    const reason = freshReasons.find((entry) => entry.order_id === order.id)?.reason
      ?? (!order.tracking_number ? "no_tracking_number" : mode === "bulk" && isShipmentFailure(order.shipment_status) ? "shipment_failed" : null);
    return reason ? [{ order_id: order.id, reason }] : [];
  });
  const printableIds = orders.filter((order) => !skipped.some((entry) => entry.order_id === order.id)).map((order) => order.id);

  async function print() {
    if (inFlight.current || printableIds.length === 0) return;
    inFlight.current = true;
    setPending(true);
    setError(null);
    try {
      const reasons = mode === "single"
        ? (await downloadShippingLabel(printableIds[0], size), null)
        : await downloadShippingLabels(printableIds, size);
      if (reasons) {
        setFreshReasons((previous) => [...previous.filter((entry) => !reasons.some((fresh) => fresh.order_id === entry.order_id)), ...reasons]);
        setError("changed");
      } else {
        onClose();
      }
    } catch {
      setError("download");
    } finally {
      inFlight.current = false;
      setPending(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => { if (!open && !inFlight.current) onClose(); }}>
      <DialogContent ref={dialogRef} onKeyDown={(event) => { if (inFlight.current && event.key === "Tab") { event.preventDefault(); event.currentTarget.focus(); } }} className="admin-shell flex flex-col gap-0 overflow-hidden p-0 [--primary:var(--md-sys-color-primary)] [--ring:var(--md-sys-color-primary)] sm:max-w-2xl" showCloseButton={!pending} onCloseAutoFocus={(event) => { event.preventDefault(); returnFocus.current?.focus(); }}>
        <DialogHeader className="shrink-0 border-b px-5 py-4 pr-12">
          <DialogTitle>{t(mode === "single" ? "labels_review_single_title" : "labels_review_title")}</DialogTitle>
          <DialogDescription>{t("labels_review_description")}</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-4">
          <div className="flex flex-wrap gap-x-5 gap-y-2 rounded-md bg-muted p-3 text-sm" aria-live="polite">
            <span>{t("labels_selected")}: <strong>{orders.length}</strong></span>
            <span>{t("labels_printable")}: <strong>{printableIds.length}</strong></span>
            <span>{t("labels_skipped")}: <strong>{skipped.length}</strong></span>
          </div>
          {error && <p role="alert" className="rounded-md border border-destructive p-3 text-sm text-destructive">{t(error === "changed" ? "labels_changed" : "labels_download_error")}</p>}
          <fieldset className="space-y-2" disabled={pending}>
            <legend className="mb-2 text-sm font-semibold">{t("labels_size")}</legend>
            <div className="grid gap-2 sm:grid-cols-2">
              {(["100x150", "roll80"] as const).map((value) => (
                <label key={value} className={`flex min-w-0 cursor-pointer items-start gap-3 rounded-md border p-3 text-sm ${size === value ? "border-primary bg-muted" : "border-border"}`}>
                  <input type="radio" name="label-size" value={value} checked={size === value} onChange={() => setSize(value)} className="mt-1 size-4 shrink-0 accent-[var(--md-sys-color-primary)]" />
                  <span className="min-w-0">
                    <span className="block font-semibold">{t(value === "100x150" ? "labels_size_fixed" : "labels_size_roll")}</span>
                    <span className="mt-1 block text-muted-foreground">{t(value === "100x150" ? "labels_size_fixed_description" : "labels_size_roll_description")}</span>
                  </span>
                </label>
              ))}
            </div>
          </fieldset>
          <section aria-label={t("labels_review_orders")} className="space-y-2 text-sm">
            <h3 className="font-semibold">{t("labels_review_orders")}</h3>
            <ul className="divide-y rounded-md border">
              {[...orders.filter((order) => skipped.some((entry) => entry.order_id === order.id)), ...orders.filter((order) => printableIds.includes(order.id))].map((order) => {
                const reason = skipped.find((entry) => entry.order_id === order.id)?.reason;
                return <li key={order.id} className={`space-y-1 break-words p-3 ${reason ? "bg-muted" : ""}`}>
                  <p className="font-medium">{order.student_name?.trim() || order.student_id} · #{order.id.slice(-8)}</p>
                  {reason ? <p className="text-destructive">{t("labels_skipped")} — <span>{reasonLabels[reason]}</span></p> : <p className="break-all font-mono text-muted-foreground">{order.tracking_number}</p>}
                </li>;
              })}
            </ul>
          </section>
        </div>
        <DialogFooter className="shrink-0 border-t px-5 py-4">
          <Button variant="outline" disabled={pending} onClick={onClose}>{t("cancel")}</Button>
          <Button className="h-auto min-h-9 whitespace-normal py-2" disabled={pending || printableIds.length === 0} aria-busy={pending} onClick={print}>
            {pending ? t("labels_preparing") : `${error === "download" ? `${t("retry")} — ` : ""}${t("labels_download").replace("{count}", String(printableIds.length))}`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
