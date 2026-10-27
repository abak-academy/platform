"use client";

import { useState } from "react";
import { Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { affLabel } from "@/lib/affiliate-labels";
import { useAffiliateOverrideProducts } from "@/lib/hooks/admin-affiliates";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateOverrideProduct } from "@/lib/types";

interface ProductPickerProps {
  id: string;
  value: AffiliateOverrideProduct | null;
  onChange: (product: AffiliateOverrideProduct | null) => void;
}

// Search products by name and pick one; replaces typing a product id by hand.
export function ProductPicker({ id, value, onChange }: ProductPickerProps) {
  const { t } = useTranslation();
  const [text, setText] = useState("");
  const [open, setOpen] = useState(false);
  const q = useDebouncedValue(text.trim());
  const results = useAffiliateOverrideProducts(q);

  if (value) {
    return (
      <div className="flex min-h-9 items-center justify-between gap-2 rounded-md border border-line px-3 py-1.5 text-sm">
        <span className="min-w-0 truncate">
          <span className="font-medium text-ink-900">{value.name}</span>{" "}
          <span className="text-xs text-ink-500">{affLabel(t, value.type)}</span>
        </span>
        <Button type="button" variant="ghost" size="icon" className="size-7" aria-label={t("admin_aff_ovr_clear")} onClick={() => onChange(null)}>
          <X className="size-4" aria-hidden />
        </Button>
      </div>
    );
  }

  return (
    <div className="relative">
      <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
      <Input
        id={id}
        role="combobox"
        aria-expanded={open}
        aria-controls={`${id}-list`}
        autoComplete="off"
        value={text}
        placeholder={t("admin_aff_ovr_search")}
        className="pl-9"
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          setText(e.target.value);
          setOpen(true);
        }}
      />
      {open && (
        <ul
          id={`${id}-list`}
          role="listbox"
          className="absolute z-20 mt-1 max-h-64 w-full overflow-y-auto rounded-md border border-line bg-surface p-1 shadow-lg"
        >
          {results.isLoading && <li className="px-3 py-2 text-sm text-ink-500">{t("aff_loading")}</li>}
          {!results.isLoading && (results.data ?? []).length === 0 && (
            <li className="px-3 py-2 text-sm text-ink-500">{t("admin_aff_ovr_no_product")}</li>
          )}
          {(results.data ?? []).map((product) => (
            <li key={product.id} role="option" aria-selected={false}>
              <button
                type="button"
                className="flex w-full items-center justify-between gap-3 rounded px-3 py-2 text-left text-sm hover:bg-surface-2"
                onClick={() => {
                  onChange(product);
                  setText("");
                  setOpen(false);
                }}
              >
                <span className="min-w-0 truncate">
                  {product.name} <span className="text-xs text-ink-500">{affLabel(t, product.type)}</span>
                </span>
                {product.percentage !== null && (
                  <span className="shrink-0 text-xs text-warn">
                    {t("admin_aff_ovr_current").replace("{pct}", String(product.percentage))}
                  </span>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
