"use client";

import { useMemo, useState } from "react";
import { ChevronRight, Loader2, Ticket, X } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type { ActivePromoCode } from "@/lib/types";
import { PromoInput } from "@/components/cart/PromoInput";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export interface PromoSheetProps {
  promos: ActivePromoCode[];
  subtotal: number;
  // Applied state comes from the order (promo_code_id); the code string is only
  // known when this session applied it, so after a reload the row shows the
  // saving without a code rather than inventing one.
  applied: boolean;
  appliedCode?: string;
  discount: number;
  isApplying?: boolean;
  error?: string;
  onApply: (code: string) => void;
  onClear: () => void;
}

// Indonesian shorthand — the rail has room for "Rp25rb" but not "Rp25.000",
// and it is the form buyers already read on every voucher here.
function compactRupiah(n: number): string {
  if (n >= 1_000_000) return `Rp${(Math.round((n / 1_000_000) * 10) / 10).toLocaleString("id-ID")}jt`;
  if (n >= 1_000) return `Rp${(Math.round((n / 1_000) * 10) / 10).toLocaleString("id-ID")}rb`;
  return formatRupiah(n);
}

function daysUntil(iso: string): number {
  const ms = new Date(iso).getTime() - Date.now();
  return Math.ceil(ms / 86_400_000);
}

export function promoShortfall(promo: ActivePromoCode, subtotal: number): number {
  return Math.max(0, (promo.min_order_amount ?? 0) - subtotal);
}

/** The value the rail shows. Percent wins when a promo carries both. */
function promoValue(promo: ActivePromoCode): { text: string; percent: boolean } {
  if (promo.discount_percent != null) return { text: `${promo.discount_percent}%`, percent: true };
  return { text: compactRupiah(promo.discount_amount ?? 0), percent: false };
}

interface VoucherCardProps {
  promo: ActivePromoCode;
  subtotal: number;
  applied: boolean;
  disabled: boolean;
  onApply: (code: string) => void;
}

// A kupon, not a list row: an indigo value rail, a tear seam, and notches
// punched through both edges of it. The notches are painted in the sheet's own
// backdrop, so this card only reads correctly on `bg-paper`.
function VoucherCard({ promo, subtotal, applied, disabled, onApply }: VoucherCardProps) {
  const { t } = useTranslation();
  const shortfall = promoShortfall(promo, subtotal);
  const locked = shortfall > 0;
  const value = promoValue(promo);
  const days = promo.expires_at ? daysUntil(promo.expires_at) : null;
  const expiringSoon = days != null && days <= 7;

  return (
    <div
      className={`relative flex overflow-hidden rounded-[13px] ${
        applied ? "ring-2 ring-success" : "ring-1 ring-line"
      }`}
    >
      <div
        className={`flex w-[78px] shrink-0 flex-col items-center justify-center gap-0.5 px-1.5 py-4 text-center ${
          locked ? "bg-line-2 text-ink-600" : "bg-brand-500 text-white"
        }`}
      >
        <span
          className={`font-mono font-bold tabular-nums leading-none ${
            value.percent ? "text-[26px]" : "text-[15px]"
          }`}
        >
          {value.text}
        </span>
        <span className="text-[10px] font-semibold uppercase tracking-[0.14em] opacity-80">
          {t("promo_off")}
        </span>
      </div>

      <div
        className={`flex min-w-0 flex-1 items-center gap-2 border-l-2 border-dashed py-3 pl-3 pr-2.5 ${
          locked ? "border-line bg-surface-2" : "border-brand-200 bg-surface"
        }`}
      >
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <span
            className={`truncate font-mono text-[13px] font-bold tracking-wide ${
              locked ? "text-ink-600" : "text-ink-900"
            }`}
          >
            {promo.code}
          </span>

          <span className="text-xs leading-snug text-ink-600">
            {locked
              ? t("promo_shortfall").replace("{v}", formatRupiah(shortfall))
              : promo.min_order_amount
                ? t("promo_min_spend").replace("{v}", formatRupiah(promo.min_order_amount))
                : t("promo_no_minimum")}
          </span>

          {(expiringSoon || (promo.max_discount_amount != null && value.percent)) && (
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              {promo.max_discount_amount != null && value.percent && (
                <span className="text-[11px] text-ink-600">
                  {t("promo_max_discount").replace("{v}", formatRupiah(promo.max_discount_amount))}
                </span>
              )}
              {expiringSoon && (
                <span className="rounded bg-gold-bg px-1.5 py-0.5 text-[11px] font-semibold whitespace-nowrap text-gold-700">
                  {days! <= 0
                    ? t("promo_expires_today")
                    : t("promo_expires_in").replace("{n}", String(days))}
                </span>
              )}
            </div>
          )}
        </div>

        {/* No control on a voucher the cart cannot use — a dead button is five
            characters that do nothing, and the shortfall line already says what
            would unlock it. */}
        {applied ? (
          <span className="shrink-0 text-xs font-semibold text-success">{t("promo_sheet_applied")}</span>
        ) : locked ? null : (
          <Button
            type="button"
            size="sm"
            disabled={disabled}
            onClick={() => onApply(promo.code)}
            className="shrink-0"
          >
            {t("promo_sheet_use")}
          </Button>
        )}
      </div>

      {/* Clipped to half-circles by the wrapper's overflow-hidden. */}
      <span className="pointer-events-none absolute -top-2 left-[78px] size-4 -translate-x-1/2 rounded-full bg-paper" />
      <span className="pointer-events-none absolute -bottom-2 left-[78px] size-4 -translate-x-1/2 rounded-full bg-paper" />
    </div>
  );
}

export function PromoSheet({
  promos,
  subtotal,
  applied,
  appliedCode,
  discount,
  isApplying,
  error,
  onApply,
  onClear,
}: PromoSheetProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  // Usable promos first, then the ones the cart has not earned yet — the same
  // order every Indonesian checkout puts them in.
  const { usable, locked } = useMemo(() => {
    const usable: ActivePromoCode[] = [];
    const locked: ActivePromoCode[] = [];
    for (const p of promos) (promoShortfall(p, subtotal) > 0 ? locked : usable).push(p);
    return { usable, locked };
  }, [promos, subtotal]);

  return (
    <>
      {applied ? (
        <div className="mt-4 flex items-center gap-3 rounded-[13px] border border-success/30 bg-success-bg px-3.5 py-3">
          <Ticket className="size-4 shrink-0 text-success" />
          <div className="flex min-w-0 flex-1 flex-col">
            <span
              className={`truncate text-sm font-bold text-ink-900 ${
                appliedCode ? "font-mono tracking-wide" : ""
              }`}
            >
              {appliedCode ?? t("promo_sheet_applied")}
            </span>
            <span className="text-xs font-semibold text-success">
              {t("promo_applied_saving").replace("{v}", formatRupiah(discount))}
            </span>
          </div>
          <button
            type="button"
            onClick={() => setOpen(true)}
            className="shrink-0 text-xs font-semibold text-ink-600 underline underline-offset-2 hover:text-ink-900"
          >
            {t("promo_change")}
          </button>
          <button
            type="button"
            onClick={onClear}
            aria-label={t("promo_remove")}
            className="shrink-0 rounded-md p-1 text-ink-500 transition-colors hover:bg-surface hover:text-ink-900"
          >
            <X className="size-4" />
          </button>
        </div>
      ) : (
        <button
          type="button"
          onClick={() => setOpen(true)}
          className="mt-4 flex w-full items-center gap-3 rounded-[13px] border border-line bg-surface-2 px-3.5 py-3 text-left transition-colors hover:border-brand-300 hover:bg-brand-50/60"
        >
          <Ticket className="size-4 shrink-0 text-brand-500" />
          <span className="flex-1 whitespace-nowrap text-sm font-semibold text-ink-900">
            {t("promo_sheet_trigger")}
          </span>
          {/* The count alone — spelling it out wrapped the label at 360px, and
              the row already names what is being counted. */}
          {usable.length > 0 && (
            <span
              aria-label={t("promo_sheet_trigger_count").replace("{n}", String(usable.length))}
              className="flex size-5 shrink-0 items-center justify-center rounded-full bg-brand-500 text-[11px] font-semibold tabular-nums text-white"
            >
              {usable.length}
            </span>
          )}
          <ChevronRight className="size-4 shrink-0 text-ink-400" />
        </button>
      )}

      {/* Only once the sheet is gone — while it is open the input owns the error,
          and the buyer would otherwise read the same rejection twice. */}
      {!applied && !open && error && <p className="mt-2 text-xs font-medium text-danger">{error}</p>}

      <Dialog open={open} onOpenChange={setOpen}>
        {/* A bottom sheet on a phone, a centred dialog on a desktop — the offers
            list is long and thumb-reachable is where it belongs. */}
        <DialogContent
          className="top-auto bottom-0 left-0 max-h-[88vh] w-full max-w-full translate-x-0 translate-y-0 gap-0 overflow-hidden rounded-t-[26px] rounded-b-none bg-paper p-0 sm:top-[50%] sm:bottom-auto sm:left-[50%] sm:max-w-md sm:translate-x-[-50%] sm:translate-y-[-50%] sm:rounded-[18px]"
        >
          <DialogHeader className="border-b border-line bg-surface px-5 pb-4 pt-5 text-left sm:text-left">
            <DialogTitle className="font-serif text-lg text-ink-900">{t("promo_sheet_title")}</DialogTitle>
            <DialogDescription className="text-xs text-ink-600">
              {t("promo_sheet_subtitle")}
            </DialogDescription>
            <PromoInput
              label={t("promo_sheet_manual_label")}
              onValidate={onApply}
              onClear={onClear}
              isValidating={isApplying}
              applied={applied}
              discount={discount}
              error={error}
            />
          </DialogHeader>

          <div className="flex flex-col gap-5 overflow-y-auto px-5 py-5">
            {promos.length === 0 ? (
              <p className="py-6 text-center text-sm text-ink-500">{t("promo_sheet_empty")}</p>
            ) : (
              <>
                {usable.length > 0 && (
                  <PromoGroup title={t("promo_sheet_eligible")} count={usable.length}>
                    {usable.map((promo) => (
                      <VoucherCard
                        key={promo.code}
                        promo={promo}
                        subtotal={subtotal}
                        applied={promo.code === appliedCode}
                        disabled={Boolean(isApplying)}
                        onApply={(code) => {
                          onApply(code);
                          setOpen(false);
                        }}
                      />
                    ))}
                  </PromoGroup>
                )}

                {locked.length > 0 && (
                  <PromoGroup title={t("promo_sheet_ineligible")} count={locked.length}>
                    {locked.map((promo) => (
                      <VoucherCard
                        key={promo.code}
                        promo={promo}
                        subtotal={subtotal}
                        applied={false}
                        disabled
                        onApply={onApply}
                      />
                    ))}
                  </PromoGroup>
                )}
              </>
            )}

            {isApplying && (
              <p className="flex items-center justify-center gap-2 text-xs text-ink-500">
                <Loader2 className="size-3.5 animate-spin" /> {t("promo_sheet_applying")}
              </p>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

function PromoGroup({
  title,
  count,
  children,
}: {
  title: string;
  count: number;
  children: React.ReactNode;
}) {
  return (
    <section className="flex flex-col gap-3">
      <h3 className="text-[11px] font-semibold uppercase tracking-[0.1em] text-ink-600">
        {title} <span className="text-ink-600">· {count}</span>
      </h3>
      {children}
    </section>
  );
}
