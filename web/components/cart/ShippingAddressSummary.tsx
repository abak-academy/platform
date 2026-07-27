"use client";

import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";

export interface SavedAddress {
  penerima: string;
  telepon: string;
  alamat: string;
  provinsi_id: string;
  kota_id: string;
  kecamatan_id: string;
  kode_pos: string;
}

export interface ShippingAddressSummaryProps {
  address: SavedAddress;
  onEdit: () => void;
}

export function isAddressComplete(a: SavedAddress): boolean {
  return Boolean(
    a.penerima && a.telepon && a.alamat && a.provinsi_id && a.kota_id && a.kecamatan_id && a.kode_pos,
  );
}

// The address states where the goods are going, so it reads before them. The
// shipping choice that depends on it lives with the goods themselves, which
// leaves this card with one job and one control.
export function ShippingAddressSummary({ address, onEdit }: ShippingAddressSummaryProps) {
  const { t } = useTranslation();
  const complete = isAddressComplete(address);

  return (
    <div className="flex items-start justify-between gap-4 rounded-lg border border-line bg-surface px-4 py-3.5 shadow-[var(--sh-sm)]">
      <div className="flex min-w-0 flex-col gap-1">
        <h2 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-500">
          {t("cart_address_heading" as any)}
        </h2>
        {complete ? (
          <p className="text-sm text-ink-600">
            <span className="font-semibold text-ink-900">{address.penerima}</span>
            <span className="mx-1.5 text-ink-400">·</span>
            {address.telepon}
            <span className="mx-1.5 text-ink-400">·</span>
            {address.alamat} · {address.kode_pos}
          </p>
        ) : (
          <p className="text-sm text-ink-500">{t("cart_address_incomplete_profile" as any)}</p>
        )}
      </div>
      <Button type="button" variant="ghost" size="sm" onClick={onEdit} className="shrink-0">
        {t("cart_address_change" as any)}
      </Button>
    </div>
  );
}
