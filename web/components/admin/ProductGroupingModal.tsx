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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type {
  AdminCreateProductGroupingInput,
  AdminUpdateProductGroupingInput,
  ProductGrouping,
  ProductGroupingStatus,
} from "@/lib/types";

interface ProductGroupingModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  grouping?: ProductGrouping | null;
  onSubmit: (input: AdminCreateProductGroupingInput | AdminUpdateProductGroupingInput) => void;
  isPending: boolean;
}

export function ProductGroupingModal({ open, onOpenChange, grouping, onSubmit, isPending }: ProductGroupingModalProps) {
  const isEdit = Boolean(grouping);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState<ProductGroupingStatus>("draft");

  useEffect(() => {
    if (!open) return;
    setName(grouping?.name ?? "");
    setDescription(grouping?.description ?? "");
    setStatus(grouping?.status ?? "draft");
  }, [open, grouping]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (isEdit) {
      onSubmit({ name, description, status } satisfies AdminUpdateProductGroupingInput);
    } else {
      onSubmit({ name, description } satisfies AdminCreateProductGroupingInput);
    }
  }

  const canSubmit = name.trim().length > 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{isEdit ? "Edit grouping produk" : "Buat grouping produk"}</DialogTitle>
            <DialogDescription>
              {isEdit
                ? "Perbarui nama, deskripsi, atau status grouping ini."
                : "Grouping baru dibuat sebagai draft; produk ditambahkan setelah grouping dibuat."}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="grouping-name">Nama</Label>
              <Input
                id="grouping-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="contoh: Paket UTBK"
                disabled={isPending}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="grouping-description">Deskripsi (opsional)</Label>
              <Input
                id="grouping-description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="contoh: Buku dan paket latihan untuk UTBK"
                disabled={isPending}
              />
            </div>

            {isEdit && (
              <div className="grid gap-2">
                <Label htmlFor="grouping-status">Status</Label>
                <Select value={status} onValueChange={(v) => setStatus(v as ProductGroupingStatus)} disabled={isPending}>
                  <SelectTrigger id="grouping-status" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="draft">Draft</SelectItem>
                    <SelectItem value="published">Published</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isPending}>
              Batal
            </Button>
            <Button type="submit" disabled={!canSubmit || isPending}>
              {isPending ? "Menyimpan..." : "Simpan"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
