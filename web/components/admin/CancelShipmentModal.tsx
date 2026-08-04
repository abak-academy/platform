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

export interface CancelShipmentModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  orderNumber: string;
  onCancelShipment: (reason: string) => void;
  isPending: boolean;
  error?: string | null;
}

/**
 * Cancels the courier booking only. The description says so explicitly:
 * cancelling a delivery and refunding an order are different actions with
 * different consequences, and conflating them is how paid deliveries get
 * quietly killed.
 */
export function CancelShipmentModal({
  open,
  onOpenChange,
  orderNumber,
  onCancelShipment,
  isPending,
  error,
}: CancelShipmentModalProps) {
  const { t } = useTranslation();
  const [reason, setReason] = useState("");

  useEffect(() => {
    if (open) setReason("");
  }, [open]);

  const canSubmit = reason.trim() !== "";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("shipment_cancel_title")}</DialogTitle>
          <DialogDescription>
            {t("shipment_cancel_desc").replace("{order}", orderNumber)}
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

        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (!canSubmit || isPending) return;
            onCancelShipment(reason.trim());
          }}
        >
          <div className="grid gap-2 py-2">
            <Label htmlFor="cancel-reason">{t("shipment_cancel_reason")}</Label>
            <Input
              id="cancel-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              disabled={isPending}
              autoFocus
            />
          </div>

          <DialogFooter>
            <Button type="submit" variant="destructive" disabled={!canSubmit || isPending}>
              {t("shipment_cancel_confirm")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
