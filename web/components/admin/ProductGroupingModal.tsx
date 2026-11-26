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
import type { AdminCreateProductGroupingInput } from "@/lib/types";

interface ProductGroupingModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (input: AdminCreateProductGroupingInput) => void;
  isPending: boolean;
}

export function ProductGroupingModal({ open, onOpenChange, onSubmit, isPending }: ProductGroupingModalProps) {
  const [name, setName] = useState("");
  const [shortName, setShortName] = useState("");
  const [description, setDescription] = useState("");

  useEffect(() => {
    if (!open) return;
    setName("");
    setShortName("");
    setDescription("");
  }, [open]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    onSubmit({
      name: name.trim(),
      ...(shortName.trim() ? { short_name: shortName.trim() } : {}),
      ...(description.trim() ? { description: description.trim() } : {}),
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Tambah event</DialogTitle>
            <DialogDescription>
              Event baru dibuat sebagai draft dan disembunyikan dari landing page. Narasi, jadwal, dan edisi diisi di halaman event.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="grouping-name">Nama event</Label>
              <Input
                id="grouping-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="contoh: ESOLIO"
                disabled={isPending}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="grouping-short-name">Nama pendek (opsional)</Label>
              <Input
                id="grouping-short-name"
                value={shortName}
                onChange={(e) => setShortName(e.target.value)}
                placeholder="contoh: Einstein"
                disabled={isPending}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="grouping-description">Deskripsi (opsional)</Label>
              <Input
                id="grouping-description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Dipakai pada filter Event di Toko"
                disabled={isPending}
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isPending}>
              Batal
            </Button>
            <Button type="submit" disabled={name.trim().length === 0 || isPending}>
              {isPending ? "Menyimpan..." : "Simpan"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
