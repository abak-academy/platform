"use client";

import { Plus, Trash2 } from "lucide-react";
import type { ProductAgreementCheckbox } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export interface AgreementCheckboxEditorProps {
  value: ProductAgreementCheckbox[];
  onChange: (checkboxes: ProductAgreementCheckbox[]) => void;
}

// Mirrors ProductSpecsEditor's row-editor shape (title/body instead of
// label/value); unlike specs, there's no per-product-type seed — a product
// starts with zero checkboxes and the admin adds only what it needs.
export function AgreementCheckboxEditor({ value, onChange }: AgreementCheckboxEditorProps) {
  const setAt = (i: number, patch: Partial<ProductAgreementCheckbox>) =>
    onChange(value.map((row, idx) => (idx === i ? { ...row, ...patch } : row)));

  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between border-b pb-2">
        <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">
          Agreement checkbox (syarat checkout)
        </h3>
        <span className="text-[11px] text-muted-foreground">wajib dicentang buyer saat checkout</span>
      </div>

      {value.length === 0 && (
        <p className="text-xs text-muted-foreground">
          Belum ada checkbox — buyer tidak akan melihat syarat apapun saat checkout produk ini.
        </p>
      )}

      {value.map((row, i) => (
        <div key={row.id || `new-${i}`} className="relative rounded-md border border-input p-3">
          <Input
            aria-label={`Judul checkbox ${i + 1}`}
            placeholder="Judul checkbox"
            value={row.title}
            onChange={(e) => setAt(i, { title: e.target.value })}
            className="mb-2 h-8"
          />
          <textarea
            aria-label={`Isi checkbox ${i + 1}`}
            placeholder="Isi (ditampilkan ke pembeli)"
            value={row.body}
            onChange={(e) => setAt(i, { body: e.target.value })}
            rows={2}
            className="w-full resize-none rounded-md border border-input bg-transparent px-2 py-1.5 text-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          />
          <button
            type="button"
            aria-label={`Hapus checkbox ${i + 1}`}
            onClick={() => onChange(value.filter((_, idx) => idx !== i))}
            className="absolute right-2 top-2 flex size-6 items-center justify-center rounded text-ink-400 hover:bg-danger-bg hover:text-danger"
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
        onClick={() => onChange([...value, { id: "", title: "", body: "" }])}
      >
        <Plus className="mr-1 size-4" />
        Tambah checkbox
      </Button>
    </section>
  );
}
