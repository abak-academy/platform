"use client";

import { useEffect, useRef, useState, type ChangeEvent } from "react";
import { toast } from "sonner";
import { Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { usePresignUpload } from "@/lib/hooks/students";
import { useTranslation } from "@/lib/i18n";

interface PayoutProofDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  description?: string;
  isPending?: boolean;
  onConfirm: (proofKey: string) => void;
}

export function PayoutProofDialog({ open, onOpenChange, description, isPending, onConfirm }: PayoutProofDialogProps) {
  const { t } = useTranslation();
  const presign = usePresignUpload();
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [proofKey, setProofKey] = useState("");

  useEffect(() => {
    if (open) setProofKey("");
  }, [open]);

  const onFile = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const presigned = await presign.mutateAsync({
        filename: file.name,
        content_type: file.type,
        kind: "payment_proof",
      });
      const res = await fetch(presigned.url, { method: "PUT", body: file, headers: { "Content-Type": file.type } });
      if (!res.ok) throw new Error(String(res.status));
      setProofKey(presigned.key);
    } catch {
      toast.error(t("admin_aff_pay_upload_failed"));
    } finally {
      if (inputRef.current) inputRef.current.value = "";
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("admin_aff_pay_proof_title")}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>
        <div className="flex flex-col gap-2">
          <Button type="button" variant="outline" onClick={() => inputRef.current?.click()} disabled={presign.isPending || isPending}>
            <Upload className="mr-2 size-4" />
            {presign.isPending ? t("admin_aff_pay_uploading") : t("admin_aff_pay_upload")}
          </Button>
          <input
            ref={inputRef}
            type="file"
            accept="image/*"
            hidden
            data-testid="payout-proof-input"
            onChange={onFile}
          />
          {proofKey && <p className="text-sm text-success">{t("admin_aff_pay_uploaded")}</p>}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isPending}>
            {t("aff_cancel_btn")}
          </Button>
          <Button onClick={() => onConfirm(proofKey)} disabled={!proofKey || isPending}>
            {t("aff_confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
