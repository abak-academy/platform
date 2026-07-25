"use client";

import { useEffect, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import type { ProductSpec, ProductType } from "@/lib/types";
import { specRowsForType } from "@/lib/product-specs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export interface ProductSpecsEditorProps {
  type: ProductType;
  value: ProductSpec[];
  onChange: (specs: ProductSpec[]) => void;
}

export function ProductSpecsEditor({ type, value, onChange }: ProductSpecsEditorProps) {
  const [rows, setRows] = useState<ProductSpec[]>(() => specRowsForType(type, value));

  useEffect(() => {
    setRows(specRowsForType(type, value));
    // Re-seed when the product type changes so the canonical field list follows it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [type]);

  const emit = (next: ProductSpec[]) => {
    setRows(next);
    onChange(next);
  };

  const setAt = (i: number, patch: Partial<ProductSpec>) =>
    emit(rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));

  return (
    <div className="flex flex-col gap-2">
      <Label>Spesifikasi Produk</Label>
      {rows.map((row, i) => (
        <div key={`${row.key}-${i}`} className="flex items-center gap-2">
          <Input
            aria-label={`Label baris ${i + 1}`}
            placeholder="Label"
            value={row.label}
            onChange={(e) => setAt(i, { label: e.target.value })}
            className="w-2/5"
          />
          <Input
            aria-label={`Nilai baris ${i + 1}`}
            placeholder="Nilai"
            value={row.value}
            onChange={(e) => setAt(i, { value: e.target.value })}
            className="flex-1"
          />
          <button
            type="button"
            aria-label={`Hapus baris ${i + 1}`}
            onClick={() => emit(rows.filter((_, idx) => idx !== i))}
            className="text-ink-400 hover:text-danger"
          >
            <Trash2 className="size-4" />
          </button>
        </div>
      ))}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="self-start"
        onClick={() =>
          emit([...rows, { key: `custom_${rows.length + 1}`, label: "", value: "" }])
        }
      >
        <Plus className="mr-1 size-4" />
        Tambah baris
      </Button>
    </div>
  );
}
