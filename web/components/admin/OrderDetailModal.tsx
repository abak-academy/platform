"use client";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { OrderStatusBadge } from "@/components/orders/OrderStatusBadge";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { Order } from "@/lib/types";
import { hasPhysicalItems } from "@/lib/shipping";

// Mirrors FallbackCourier in backend/internal/service/shipping_rates.go, same as
// the storefront's ShippingInfo. The order row carries no is_estimate flag, so a
// flat rate can only be recognised by name — see docs/backlog/shipping-estimate-flag.md.
const ESTIMATE_COURIERS = new Set(["Ongkir Flat", "Flat"]);

export interface OrderDetailModalProps {
  order: Order | null;
  onOpenChange: (open: boolean) => void;
}

function formatDateTime(iso: string | undefined, lang: string): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  return new Intl.DateTimeFormat(lang === "id" ? "id-ID" : "en-GB", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(d);
}

export function OrderDetailModal({ order, onOpenChange }: OrderDetailModalProps) {
  const { t, lang } = useTranslation();
  if (!order) return null;

  const items = order.items ?? [];
  const addr = order.shipping_address ?? {};
  const hasPhysical = hasPhysicalItems(items);
  // Narrowest first, the way an Indonesian address is written.
  const region = [addr.kecamatan, addr.kota, addr.provinsi].filter(Boolean).join(", ");
  const courier = [order.selected_courier, order.selected_service].filter(Boolean).join(" — ");
  const isEstimate = ESTIMATE_COURIERS.has(order.selected_courier ?? "");
  const placed = formatDateTime(order.checked_out_at ?? order.created_at, lang);
  const paid = formatDateTime(order.paid_at, lang);
  const shipped = formatDateTime(order.shipped_at, lang);

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[90vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        <DialogHeader className="shrink-0 border-b px-6 py-5">
          <DialogTitle className="flex items-center gap-3">
            <span className="font-mono">#{order.id.slice(-8)}</span>
            <OrderStatusBadge status={order.status} />
          </DialogTitle>
          <DialogDescription>
            {[placed, order.payment_method].filter(Boolean).join(" · ") || order.id}
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-6 py-5">
          <Section title={t("order_items_section")}>
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-xs uppercase tracking-[0.06em] text-muted-foreground">
                  <th className="pb-2 font-medium">{t("th_product")}</th>
                  <th className="pb-2 text-right font-medium">{t("th_price")}</th>
                  <th className="pb-2 text-right font-medium">{t("th_total")}</th>
                </tr>
              </thead>
              <tbody>
                {items.map((it) => (
                  <tr key={it.id} className="border-b last:border-b-0">
                    <td className="py-2 pr-3">
                      <span className="font-medium">{it.name}</span>
                      <span className="ml-2 text-xs text-muted-foreground">×{it.qty}</span>
                    </td>
                    <td className="py-2 text-right tabular-nums">{formatRupiah(it.unit_price)}</td>
                    <td className="py-2 text-right font-medium tabular-nums">
                      {formatRupiah(it.jumlah)}
                    </td>
                  </tr>
                ))}
                {items.length === 0 && (
                  <tr>
                    <td colSpan={3} className="py-4 text-center text-muted-foreground">
                      {t("empty_orders")}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </Section>

          {hasPhysical && (
            <Section title={t("order_shipping_heading")}>
              <dl className="space-y-2 text-sm">
                <Field label={t("order_shipping_address")}>
                  {addr.penerima || addr.alamat ? (
                    <span className="flex flex-col items-end gap-0.5 text-right">
                      {addr.penerima && <span className="font-medium">{addr.penerima}</span>}
                      {addr.telepon && <span className="text-muted-foreground">{addr.telepon}</span>}
                      {addr.alamat && <span>{addr.alamat}</span>}
                      {/* Empty on orders placed before checkout began storing the
                          region names — those rows only ever held the IDs. */}
                      {region && <span>{region}</span>}
                      {addr.kode_pos && <span className="text-muted-foreground">{addr.kode_pos}</span>}
                    </span>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </Field>
                {/* Written by the buyer at checkout. It is the reason the packer
                    opens this modal at all, so it is not tucked behind a toggle. */}
                {addr.catatan && (
                  <Field label={t("order_courier_note")}>
                    <span className="text-right whitespace-pre-line">{addr.catatan}</span>
                  </Field>
                )}
                {courier && (
                  <Field label={t("order_shipping_courier")}>
                    <span className="text-right">
                      {courier}
                      {isEstimate && (
                        <span className="ml-2 rounded bg-warn-bg px-1.5 py-0.5 text-xs text-warn">
                          {t("order_shipping_estimate_note")}
                        </span>
                      )}
                    </span>
                  </Field>
                )}
                <Field label={t("order_tracking")}>
                  {order.tracking_number ? (
                    <span className="font-mono">{order.tracking_number}</span>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </Field>
                {shipped && <Field label={t("status_shipped")}>{shipped}</Field>}
              </dl>
            </Section>
          )}

          <Section title={t("order_payment_info")}>
            <dl className="space-y-2 text-sm">
              <Field label={t("th_buyer")}>
                <span className="font-mono text-xs">{order.student_id}</span>
              </Field>
              {order.gateway_ref && (
                <Field label={t("order_gateway_ref")}>
                  <span className="font-mono text-xs">{order.gateway_ref}</span>
                </Field>
              )}
              {paid && <Field label={t("filter_paid")}>{paid}</Field>}
              <Field label={t("order_subtotal")}>{formatRupiah(order.subtotal)}</Field>
              {order.discount > 0 && (
                <Field label={t("order_discount")}>−{formatRupiah(order.discount)}</Field>
              )}
              {hasPhysical && (
                <Field label={t("order_shipping")}>{formatRupiah(order.shipping_cost ?? 0)}</Field>
              )}
              <div className="flex items-center justify-between border-t pt-2 text-base font-semibold">
                <span>{t("order_total")}</span>
                <span className="tabular-nums">{formatRupiah(order.total)}</span>
              </div>
            </dl>
          </Section>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <h3 className="border-b pb-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">
        {title}
      </h3>
      {children}
    </section>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-6">
      <dt className="shrink-0 text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-right">{children}</dd>
    </div>
  );
}
