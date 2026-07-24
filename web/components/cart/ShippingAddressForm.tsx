"use client";

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import {
  useCitiesByProvince,
  useDistrictsByCity,
  useProvinces,
} from "@/lib/hooks/regions";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { User } from "@/lib/types";

export interface ShippingAddressFormState {
  penerima: string;
  telepon: string;
  alamat: string;
  provinsi_id: string;
  kota_id: string;
  kecamatan_id: string;
  kode_pos: string;
}

const FIELD_CLASS =
  "h-11 w-full rounded-md border border-line bg-surface px-3.5 text-sm text-ink-900 shadow-none transition-[border-color,box-shadow] outline-none placeholder:text-ink-400 focus-visible:border-brand-400 focus-visible:ring-[3px] focus-visible:ring-brand-50 disabled:cursor-not-allowed disabled:bg-surface-2/60 disabled:text-ink-500";

interface ShippingAddressFormProps {
  profile: User | undefined;
  initialAddress?: Partial<ShippingAddressFormState>;
  onAddressChange: (state: ShippingAddressFormState) => void;
  onCheckShipping: () => void;
  isCheckingShipping: boolean;
}

function hasAddressValues(a?: Partial<ShippingAddressFormState>): boolean {
  return Boolean(
    a && (a.penerima || a.telepon || a.alamat || a.provinsi_id || a.kota_id || a.kecamatan_id || a.kode_pos),
  );
}

export function ShippingAddressForm({
  profile,
  initialAddress,
  onAddressChange,
  onCheckShipping,
  isCheckingShipping,
}: ShippingAddressFormProps) {
  const { t } = useTranslation();

  const [penerima, setPenerima] = useState("");
  const [telepon, setTelepon] = useState("");
  const [alamat, setAlamat] = useState("");
  const [provinsiId, setProvinsiId] = useState("");
  const [kotaId, setKotaId] = useState("");
  const [kecamatanId, setKecamatanId] = useState("");
  const [kodePos, setKodePos] = useState("");

  const { data: provinces, isLoading: provincesLoading } = useProvinces();
  const { data: cities, isLoading: citiesLoading } = useCitiesByProvince(
    provinsiId || null
  );
  const { data: districts, isLoading: districtsLoading } = useDistrictsByCity(
    kotaId || null
  );

  useEffect(() => {
    if (hasAddressValues(initialAddress)) {
      setPenerima(initialAddress?.penerima ?? "");
      setTelepon(initialAddress?.telepon ?? "");
      setAlamat(initialAddress?.alamat ?? "");
      setProvinsiId(initialAddress?.provinsi_id ?? "");
      setKotaId(initialAddress?.kota_id ?? "");
      setKecamatanId(initialAddress?.kecamatan_id ?? "");
      setKodePos(initialAddress?.kode_pos ?? "");
    } else if (profile) {
      setPenerima(profile.name ?? "");
      setTelepon(profile.phone ?? "");
      setAlamat(profile.alamat_domisili ?? "");
      setProvinsiId(profile.provinsi_id ?? "");
      setKotaId(profile.kota_id ?? "");
      setKecamatanId(profile.kecamatan_id ?? "");
      setKodePos(profile.kode_pos ?? "");
    }
  }, [profile, initialAddress]);

  useEffect(() => {
    onAddressChange({
      penerima,
      telepon,
      alamat,
      provinsi_id: provinsiId,
      kota_id: kotaId,
      kecamatan_id: kecamatanId,
      kode_pos: kodePos,
    });
  }, [penerima, telepon, alamat, provinsiId, kotaId, kecamatanId, kodePos, onAddressChange]);

  const handleProvinceChange = (value: string) => {
    setProvinsiId(value === "_empty_" ? "" : value);
    setKotaId("");
    setKecamatanId("");
  };

  const handleCityChange = (value: string) => {
    setKotaId(value === "_empty_" ? "" : value);
    setKecamatanId("");
  };

  const handleDistrictChange = (value: string) => {
    setKecamatanId(value === "_empty_" ? "" : value);
  };

  return (
    <div className="flex flex-col gap-4 rounded-lg border border-line bg-surface p-5">
      <h3 className="font-semibold text-ink-900">{t("cart_shipping_title") || "Shipping Address"}</h3>

      <div className="grid gap-3">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="penerima" className="text-xs font-semibold text-ink-600">
            {t("cart_address_recipient" as any)}
          </Label>
          <input id="penerima" className={FIELD_CLASS} value={penerima}
            onChange={(e) => setPenerima(e.target.value)} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="telepon" className="text-xs font-semibold text-ink-600">
            {t("cart_address_phone" as any)}
          </Label>
          <input id="telepon" className={FIELD_CLASS} value={telepon}
            onChange={(e) => setTelepon(e.target.value)} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="alamat" className="text-xs font-semibold text-ink-600">
            {t("cart_address_street" as any)}
          </Label>
          <input id="alamat" className={FIELD_CLASS} value={alamat}
            onChange={(e) => setAlamat(e.target.value)} />
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="provinsi" className="text-xs font-semibold text-ink-600">
            {t("students_field_provinsi") || "Province"}
          </Label>
          <Select
            value={provinsiId || "_empty_"}
            onValueChange={handleProvinceChange}
          >
            <SelectTrigger id="provinsi" className={FIELD_CLASS}>
              <SelectValue
                placeholder={t("select_province") || "Select Province"}
              />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_empty_">
                {t("select_province") || "Select Province"}
              </SelectItem>
              {provinces?.map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="kota" className="text-xs font-semibold text-ink-600">
            {t("students_field_kota") || "City"}
          </Label>
          <Select
            value={kotaId || "_empty_"}
            onValueChange={handleCityChange}
            disabled={!provinsiId || citiesLoading}
          >
            <SelectTrigger id="kota" className={FIELD_CLASS}>
              <SelectValue
                placeholder={t("select_city") || "Select City"}
              />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_empty_">
                {t("select_city") || "Select City"}
              </SelectItem>
              {cities?.map((c) => (
                <SelectItem key={c.id} value={c.id}>
                  {c.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="kecamatan" className="text-xs font-semibold text-ink-600">
            {t("students_field_kecamatan") || "District"}
          </Label>
          <Select
            value={kecamatanId || "_empty_"}
            onValueChange={handleDistrictChange}
            disabled={!kotaId || districtsLoading}
          >
            <SelectTrigger id="kecamatan" className={FIELD_CLASS}>
              <SelectValue
                placeholder={t("select_district") || "Select District"}
              />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_empty_">
                {t("select_district") || "Select District"}
              </SelectItem>
              {districts?.map((d) => (
                <SelectItem key={d.id} value={d.id}>
                  {d.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="kode-pos" className="text-xs font-semibold text-ink-600">
            {t("students_field_kode_pos") || "Postal Code"}
          </Label>
          <Input
            id="kode-pos"
            type="text"
            value={kodePos}
            onChange={(e) => setKodePos(e.target.value)}
            placeholder={t("students_field_kode_pos_placeholder") || "e.g., 40123"}
            className={FIELD_CLASS}
          />
        </div>
      </div>

      <Button
        onClick={onCheckShipping}
        disabled={!penerima || !telepon || !alamat || !provinsiId || !kotaId || !kecamatanId || !kodePos || isCheckingShipping}
        className="w-full"
      >
        {isCheckingShipping ? (
          <Loader2 className="mr-2 size-4 animate-spin" />
        ) : null}
        {t("cart_check_shipping_cost") || "Check Shipping Cost"}
      </Button>
      {(!penerima || !telepon || !alamat || !provinsiId || !kotaId || !kecamatanId) && (
        <p className="text-xs text-ink-500">
          {t("cart_shipping_address_incomplete") || "Select province, city, and district to check shipping cost."}
        </p>
      )}
    </div>
  );
}
