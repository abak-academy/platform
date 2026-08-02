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
import { usePresignUpload } from "@/lib/hooks/students";

export interface RefundOrderModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  orderNumber: string;
  onRefund: (refundProofUrl: string) => void;
  isPending: boolean;
  error?: string | null;
}

export function RefundOrderModal({
  open,
  onOpenChange,
  orderNumber,
  onRefund,
  isPending,
  error,
}: RefundOrderModalProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const presign = usePresignUpload();
  const [proofKey, setProofKey] = useState("");
  // Previewed from the local File: refund_proof is not served by the public
  // /files/* proxy, so a key-based URL would 404.
  const [previewUrl, setPreviewUrl] = useState("");

  useEffect(() => {
    if (open) {
      setProofKey("");
      setPreviewUrl((prev) => {
        if (prev) URL.revokeObjectURL(prev);
        return "";
      });
    }
  }, [open]);

  async function handleFileSelected(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    try {
      const presigned = await presign.mutateAsync({
        filename: file.name,
        content_type: file.type,
        kind: "refund_proof",
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
      setPreviewUrl((prev) => {
        if (prev) URL.revokeObjectURL(prev);
        return URL.createObjectURL(file);
      });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Unggah bukti gagal");
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  }

  function handleSubmit() {
    if (!proofKey || isPending) return;
    onRefund(proofKey);
  }

  const uploading = presign.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Tandai Sudah Direfund</DialogTitle>
          <DialogDescription>{`Pesanan ${orderNumber}`}</DialogDescription>
        </DialogHeader>

        {/* The system does not move money — saying so here is the difference
            between an honest record and an order that merely looks settled. */}
        <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
          Sistem <strong>tidak</strong> mengembalikan uang secara otomatis. Transfer dulu ke pembeli,
          lalu unggah bukti transfernya di sini. Stok barang juga <strong>tidak</strong> dikembalikan
          otomatis — sesuaikan manual bila perlu.
        </div>

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
            {uploading ? "Mengunggah..." : "Unggah bukti transfer"}
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            hidden
            data-testid="refund-order-proof-input"
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
          <Button
            type="button"
            variant="destructive"
            onClick={handleSubmit}
            disabled={!proofKey || isPending}
          >
            {isPending ? "Menyimpan..." : "Tandai sudah direfund"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
