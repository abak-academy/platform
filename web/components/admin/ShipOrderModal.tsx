"use client";

import { useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "@/lib/i18n";

export interface ShipOrderModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  orderNumber: string;
  onBook: (schedule?: { deliveryDate: string; deliveryTime: string }) => void;
  onSubmitManual: (trackingNumber: string) => void;
  isPending: boolean;
  error?: string | null;
}

export function ShipOrderModal({
  open,
  onOpenChange,
  orderNumber,
  onBook,
  onSubmitManual,
  isPending,
  error,
}: ShipOrderModalProps) {
  const { t } = useTranslation();
  const [manualMode, setManualMode] = useState(false);
  const [trackingNumber, setTrackingNumber] = useState("");
  const [scheduling, setScheduling] = useState(false);
  const [deliveryDate, setDeliveryDate] = useState("");
  const [deliveryTime, setDeliveryTime] = useState("");

  useEffect(() => {
    if (open) {
      setManualMode(false);
      setTrackingNumber("");
      setScheduling(false);
      setDeliveryDate("");
      setDeliveryTime("");
    }
  }, [open]);

  const canSubmitManual = trackingNumber.trim() !== "";
  // Half a schedule is not a schedule. Booking would otherwise fall back to an
  // immediate pickup, turning an unfinished form into a courier dispatched
  // today for a parcel the admin meant to send later.
  const scheduleIncomplete = scheduling && !(deliveryDate && deliveryTime);

  function handleManualSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmitManual || isPending) return;
    onSubmitManual(trackingNumber.trim());
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("orders_ship_title")}</DialogTitle>
          <DialogDescription>
            {t(manualMode ? "orders_ship_subtitle" : "orders_ship_choice_subtitle").replace(
              "{order}",
              orderNumber,
            )}
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div
            role="alert"
            className="rounded-md border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive"
          >
            {error}
          </div>
        )}

        {!manualMode && (
          <>
            <div className="py-2">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  data-testid="ship-schedule-toggle"
                  checked={scheduling}
                  onChange={(e) => setScheduling(e.target.checked)}
                  disabled={isPending}
                />
                {t("orders_ship_schedule_toggle")}
              </label>

              {scheduling && (
                <div className="mt-2 grid grid-cols-2 gap-2">
                  <div className="grid gap-1">
                    <Label htmlFor="delivery-date">{t("orders_ship_schedule_date")}</Label>
                    <Input
                      id="delivery-date"
                      type="date"
                      value={deliveryDate}
                      onChange={(e) => setDeliveryDate(e.target.value)}
                      disabled={isPending}
                    />
                  </div>
                  <div className="grid gap-1">
                    <Label htmlFor="delivery-time">{t("orders_ship_schedule_time")}</Label>
                    <Input
                      id="delivery-time"
                      type="time"
                      value={deliveryTime}
                      onChange={(e) => setDeliveryTime(e.target.value)}
                      disabled={isPending}
                    />
                  </div>
                  <p className="col-span-2 text-xs text-muted-foreground">
                    {scheduleIncomplete
                      ? t("orders_ship_schedule_incomplete")
                      : t("orders_ship_schedule_hint")}
                  </p>
                </div>
              )}
            </div>

            <DialogFooter className="flex-wrap sm:justify-end">
              <Button type="button" variant="outline" onClick={() => setManualMode(true)} disabled={isPending}>
                {t("orders_ship_manual_choice")}
              </Button>
              <Button
                type="button"
                onClick={() =>
                  onBook(scheduling ? { deliveryDate, deliveryTime } : undefined)
                }
                disabled={isPending || scheduleIncomplete}
              >
                {isPending ? t("orders_ship_booking_pending") : t("orders_ship_book_courier")}
              </Button>
            </DialogFooter>
          </>
        )}

        {manualMode && (
          <form onSubmit={handleManualSubmit}>
            <div className="grid gap-2 py-2">
              <Label htmlFor="tracking-number">{t("order_tracking")}</Label>
              <Input
                id="tracking-number"
                value={trackingNumber}
                onChange={(e) => setTrackingNumber(e.target.value)}
                placeholder={t("orders_ship_placeholder")}
                disabled={isPending}
                autoFocus
              />
            </div>

            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => setManualMode(false)} disabled={isPending}>
                {t("orders_ship_manual_back")}
              </Button>
              <Button type="submit" disabled={!canSubmitManual || isPending}>
                {isPending ? t("orders_ship_pending") : t("action_ship")}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
