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
  onCheckShipping: () => void;
  isCheckingShipping?: boolean;
}

export function isAddressComplete(a: SavedAddress): boolean {
  return Boolean(
    a.penerima && a.telepon && a.alamat && a.provinsi_id && a.kota_id && a.kecamatan_id && a.kode_pos,
  );
}

export function ShippingAddressSummary({
  address,
  onEdit,
  onCheckShipping,
  isCheckingShipping,
}: ShippingAddressSummaryProps) {
  const { t } = useTranslation();
  const complete = isAddressComplete(address);

  return (
    <div className="rounded-lg border border-line bg-surface p-5">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="font-serif text-base font-semibold text-ink-900">
          {t("cart_address_heading" as any)}
        </h2>
        <Button type="button" variant="ghost" size="sm" onClick={onEdit}>
          {t("cart_address_change" as any)}
        </Button>
      </div>

      {complete ? (
        <div className="flex flex-col gap-0.5 text-sm">
          <span className="font-medium text-ink-900">{address.penerima}</span>
          <span className="text-ink-600">{address.telepon}</span>
          <span className="text-ink-600">
            {address.alamat} · {address.kode_pos}
          </span>
        </div>
      ) : (
        <p className="text-sm text-ink-500">{t("cart_address_incomplete_profile" as any)}</p>
      )}

      {complete && (
        <Button
          type="button"
          onClick={onCheckShipping}
          disabled={isCheckingShipping}
          className="mt-4 w-full"
        >
          {t("cart_check_shipping_cost") || "Check Shipping Cost"}
        </Button>
      )}
    </div>
  );
}
