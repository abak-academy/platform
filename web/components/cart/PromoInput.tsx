"use client";

import { useState } from "react";
import { CheckCircle2, Loader2, Tag } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export interface PromoInputProps {
  onValidate: (code: string) => void;
  onClear: () => void;
  isValidating?: boolean;
  // Applied state and the discount amount both come from the order, not from
  // the validate call — the order is what the buyer is actually charged.
  applied?: boolean;
  discount?: number;
  error?: string;
}

export function PromoInput({ onValidate, onClear, isValidating, applied, discount, error }: PromoInputProps) {
  const [code, setCode] = useState("");

  return (
    <div className="mt-4">
      <label htmlFor="promo-code" className="mb-2 block text-xs font-semibold uppercase tracking-wide text-ink-500">
        Kode Promo
      </label>
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Tag className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-ink-400" />
          <Input
            id="promo-code"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            placeholder="Masukkan kode promo"
            className="pl-9"
            disabled={isValidating}
          />
        </div>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={() => {
            const c = code.trim();
            if (!c) return;
            onValidate(c);
          }}
          disabled={isValidating || !code.trim()}
        >
          {isValidating ? <Loader2 className="size-4 animate-spin" /> : "Pakai"}
        </Button>
      </div>

      {applied && (
        <div className="mt-2 flex items-center gap-1.5 text-xs font-semibold text-success">
          <CheckCircle2 className="size-3.5" />
          Promo diterapkan −{discount && discount > 0 ? discount.toLocaleString("id-ID") : 0}
          <button
            type="button"
            onClick={onClear}
            className="font-normal text-ink-500 underline hover:text-ink-700"
          >
            Hapus
          </button>
        </div>
      )}
      {!applied && error && <p className="mt-2 text-xs font-medium text-danger">{error}</p>}
    </div>
  );
}