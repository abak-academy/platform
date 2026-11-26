"use client";

import { useState } from "react";
import { CheckCircle2, Loader2 } from "lucide-react";
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
  label?: string;
  placeholder?: string;
  applyLabel?: string;
}

export function PromoInput({ onValidate, onClear, isValidating, applied, discount, error, label, placeholder, applyLabel }: PromoInputProps) {
  const [code, setCode] = useState("");

  return (
    <div>
      {label && (
        <label htmlFor="promo-code" className="mb-2 block text-xs font-semibold uppercase tracking-wide text-ink-600">
          {label}
        </label>
      )}
      {/* Prototype `.promo`: an uppercase code field and an outline Pakai. */}
      <div className="flex gap-2">
        <Input
          id="promo-code"
          value={code}
          onChange={(e) => setCode(e.target.value)}
          placeholder={placeholder ?? "Masukkan kode promo"}
          aria-label={label ? undefined : placeholder ?? "Kode promo"}
          className="h-[var(--spacing-input)] flex-1 rounded-[14px] px-[15px] py-0 uppercase"
          disabled={isValidating}
        />
        <Button
          type="button"
          variant="outline"
          className="h-[var(--spacing-control-md)] self-center border"
          onClick={() => {
            const c = code.trim();
            if (!c) return;
            onValidate(c);
          }}
          disabled={isValidating || !code.trim()}
        >
          {isValidating ? <Loader2 className="size-4 animate-spin" /> : applyLabel ?? "Pakai"}
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