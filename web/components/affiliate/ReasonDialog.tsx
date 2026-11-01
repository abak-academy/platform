"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useTranslation } from "@/lib/i18n";

interface ReasonDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  isPending?: boolean;
  onConfirm: (reason: string) => void;
}

export function ReasonDialog({ open, onOpenChange, title, description, isPending, onConfirm }: ReasonDialogProps) {
  const { t } = useTranslation();
  const [reason, setReason] = useState("");
  const [missing, setMissing] = useState(false);

  useEffect(() => {
    if (open) {
      setReason("");
      setMissing(false);
    }
  }, [open]);

  const submit = () => {
    if (!reason.trim()) {
      setMissing(true);
      return;
    }
    onConfirm(reason.trim());
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>
        <label className="space-y-1 text-sm">
          <span className="font-medium text-ink-700">{t("aff_reason_label")}</span>
          <textarea
            value={reason}
            onChange={(e) => {
              setReason(e.target.value);
              setMissing(false);
            }}
            rows={3}
            className="w-full rounded-md border border-line bg-surface p-2"
          />
        </label>
        {missing && <p className="text-xs text-danger">{t("aff_reason_required")}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isPending}>
            {t("aff_cancel_btn")}
          </Button>
          <Button onClick={submit} disabled={isPending}>
            {t("aff_confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
