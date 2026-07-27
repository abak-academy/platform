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
  onSubmit: (trackingNumber: string) => void;
  isPending: boolean;
}

export function ShipOrderModal({
  open,
  onOpenChange,
  orderNumber,
  onSubmit,
  isPending,
}: ShipOrderModalProps) {
  const { t } = useTranslation();
  const [trackingNumber, setTrackingNumber] = useState("");

  useEffect(() => {
    if (open) setTrackingNumber("");
  }, [open]);

  const canSubmit = trackingNumber.trim() !== "";

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit || isPending) return;
    onSubmit(trackingNumber.trim());
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("orders_ship_title")}</DialogTitle>
            <DialogDescription>
              {t("orders_ship_subtitle").replace("{order}", orderNumber)}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-2 py-4">
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
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
            >
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={!canSubmit || isPending}>
              {isPending ? t("orders_ship_pending") : t("action_ship")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
