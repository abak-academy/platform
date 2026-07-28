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
  // True when this is the buyer's own saved address rather than a one-off
  // destination. Derived by comparison against the profile, so it costs no
  // column — and it is the mark that tells addresses apart once there are several.
  isPrimary?: boolean;
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
export function ShippingAddressSummary({ address, isPrimary = false, onEdit }: ShippingAddressSummaryProps) {
  const { t } = useTranslation();
  const complete = isAddressComplete(address);

  return (
    <div className="flex items-start justify-between gap-4 rounded-lg border border-line bg-surface px-4 py-3.5 shadow-[var(--sh-sm)]">
      <div className="flex min-w-0 flex-col gap-1">
        <h2 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-500">
          {t("cart_address_heading" as any)}
          {isPrimary && (
            <span className="rounded bg-success-bg px-1.5 py-0.5 text-[10px] font-medium normal-case tracking-normal text-success">
              {t("cart_address_primary_badge" as any)}
            </span>
          )}
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
