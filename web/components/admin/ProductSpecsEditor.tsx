"use client";

import { useEffect, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import type { ProductSpec, ProductType } from "@/lib/types";
import { specRowsForType } from "@/lib/product-specs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

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
    <section className="flex flex-col gap-3">
      <h3 className="border-b pb-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">
        Spesifikasi produk
      </h3>

      {/* Eight loose input pairs read as a pile; column headings and shared
          borders make the same rows read as one table. */}
      <div className="overflow-hidden rounded-md border border-input">
        <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2rem] sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)_2rem] items-center gap-2 border-b border-input bg-muted/40 px-2 py-1.5 text-[11px] font-medium uppercase tracking-[0.06em] text-muted-foreground">
          <span>Label</span>
          <span>Nilai</span>
          <span className="sr-only">Hapus</span>
        </div>
        {rows.map((row, i) => (
          <div
            key={`${row.key}-${i}`}
            className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2rem] sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)_2rem] items-center gap-2 border-b border-input px-2 py-1.5 last:border-b-0"
          >
            <Input
              aria-label={`Label baris ${i + 1}`}
              placeholder="Label"
              value={row.label}
              onChange={(e) => setAt(i, { label: e.target.value })}
              className="h-8 border-0 bg-transparent px-1 shadow-none focus-visible:ring-1"
            />
            <Input
              aria-label={`Nilai baris ${i + 1}`}
              placeholder="Nilai"
              value={row.value}
              onChange={(e) => setAt(i, { value: e.target.value })}
              className="h-8 border-0 bg-transparent px-1 shadow-none focus-visible:ring-1"
            />
            <button
              type="button"
              aria-label={`Hapus baris ${i + 1}`}
              onClick={() => emit(rows.filter((_, idx) => idx !== i))}
              className="flex size-7 items-center justify-center rounded text-ink-400 hover:bg-danger-bg hover:text-danger"
            >
              <Trash2 className="size-4" />
            </button>
          </div>
        ))}
      </div>

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
    </section>
  );
}
