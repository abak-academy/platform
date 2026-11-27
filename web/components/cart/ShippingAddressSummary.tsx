"use client";

import { useTranslation } from "@/lib/i18n";
import { MapPin, Pencil } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export interface SavedAddress {
  penerima: string;
  telepon: string;
  alamat: string;
  provinsi_id: string;
  kota_id: string;
  kecamatan_id: string;
  kode_pos: string;
  provinsi?: string;
  kota?: string;
  kecamatan?: string;
}

export interface ShippingAddressSummaryProps {
  address: SavedAddress;
  // True when this is the buyer's own saved address rather than a one-off
  // destination. Derived by comparison against the profile, so it costs no
  // column — and it is the mark that tells addresses apart once there are several.
  isPrimary?: boolean;
  onEdit: () => void;
  children?: React.ReactNode;
}

export function isAddressComplete(a: SavedAddress): boolean {
  return Boolean(
    a.penerima && a.telepon && a.alamat && a.provinsi_id && a.kota_id && a.kecamatan_id && a.kode_pos,
  );
}

// Prototype address card: `.block-head` + `.addr` box. Children (the courier
// note) sit under the box, as in the prototype.
export function ShippingAddressSummary({ address, isPrimary = false, onEdit, children }: ShippingAddressSummaryProps) {
  const { t } = useTranslation();
  const complete = isAddressComplete(address);
  const region = [address.alamat, address.kecamatan, address.kota, address.provinsi].filter(Boolean).join(", ");

  return (
    <Card className="gap-0 p-6">
      <div className="mb-3.5 flex flex-wrap items-center justify-between gap-3">
        <h2 className="type-h2 text-ink-900">{t("cart_address_heading" as any)}</h2>
        <Button type="button" variant="ghost" size="sm" onClick={onEdit} className="gap-2 has-[>svg]:px-[13px]">
          <Pencil />
          {t("cart_address_change" as any)}
        </Button>
      </div>
      {complete ? (
        <div className="mb-3.5 flex gap-3 rounded-[16px] bg-surface-sunken p-3.5">
          <MapPin className="size-5 shrink-0 text-ember-700" aria-hidden />
          <div className="min-w-0">
            <b className="font-bold text-ink-900">
              <span>{address.penerima}</span> · {address.telepon}
            </b>
            {isPrimary && (
              <Badge variant="teal" className="ml-1.5 align-middle">
                {t("cart_address_primary_badge" as any)}
              </Badge>
            )}
            <p className="mt-1 text-ink-600">
              {region} {address.kode_pos}
            </p>
          </div>
        </div>
      ) : (
        <p className="text-ink-600">{t("cart_address_incomplete_profile" as any)}</p>
      )}
      {children}
    </Card>
  );
}
