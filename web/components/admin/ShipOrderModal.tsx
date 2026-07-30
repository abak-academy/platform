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
  onBook: () => void;
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

  useEffect(() => {
    if (open) {
      setManualMode(false);
      setTrackingNumber("");
    }
  }, [open]);

  const canSubmitManual = trackingNumber.trim() !== "";

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
            {t("orders_ship_subtitle").replace("{order}", orderNumber)}
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
          <DialogFooter className="sm:justify-between">
            <Button type="button" variant="ghost" onClick={() => setManualMode(true)} disabled={isPending}>
              {t("orders_ship_manual_choice")}
            </Button>
            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isPending}>
                {t("cancel")}
              </Button>
              <Button type="button" onClick={onBook} disabled={isPending}>
                {isPending ? t("orders_ship_pending") : t("orders_ship_book_biteship")}
              </Button>
            </div>
          </DialogFooter>
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
