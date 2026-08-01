"use client";

import { useEffect, useRef, useState, type ChangeEvent } from "react";
import { toast } from "sonner";
import { Upload } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { fileUrl } from "@/lib/api";
import { usePresignUpload } from "@/lib/hooks/students";

export interface ConfirmOrderModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  orderNumber: string;
  onConfirm: (paymentProofUrl: string) => void;
  isPending: boolean;
  error?: string | null;
}

export function ConfirmOrderModal({
  open,
  onOpenChange,
  orderNumber,
  onConfirm,
  isPending,
  error,
}: ConfirmOrderModalProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const presign = usePresignUpload();
  const [proofKey, setProofKey] = useState("");

  useEffect(() => {
    if (open) setProofKey("");
  }, [open]);

  async function handleFileSelected(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    try {
      const presigned = await presign.mutateAsync({
        filename: file.name,
        content_type: file.type,
        kind: "payment_proof",
      });

      const uploadRes = await fetch(presigned.url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": file.type },
      });

      if (!uploadRes.ok) {
        throw new Error(`Upload failed: ${uploadRes.status}`);
      }

      setProofKey(presigned.key);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Unggah bukti gagal");
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  }

  function handleSubmit() {
    if (!proofKey || isPending) return;
    onConfirm(proofKey);
  }

  const uploading = presign.isPending;
  const previewUrl = fileUrl(proofKey);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Konfirmasi Pembayaran Manual</DialogTitle>
          <DialogDescription>{`Pesanan ${orderNumber}`}</DialogDescription>
        </DialogHeader>

        {error && (
          <div
            role="alert"
            className="rounded-md border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive"
          >
            {error}
          </div>
        )}

        <div className="flex flex-col gap-3">
          <Button
            type="button"
            variant="outline"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading || isPending}
          >
            <Upload className="mr-2 size-4" />
            {uploading ? "Mengunggah..." : "Unggah bukti pembayaran"}
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            hidden
            data-testid="confirm-order-proof-input"
            onChange={handleFileSelected}
          />
          {previewUrl && (
            <a href={previewUrl} target="_blank" rel="noreferrer" className="text-sm text-primary underline">
              Lihat bukti yang diunggah
            </a>
          )}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isPending}>
            Batal
          </Button>
          <Button type="button" onClick={handleSubmit} disabled={!proofKey || isPending}>
            {isPending ? "Mengonfirmasi..." : "Konfirmasi"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
