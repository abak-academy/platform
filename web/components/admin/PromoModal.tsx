"use client";

import { useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PartnerPicker } from "@/components/admin/PartnerPicker";
import { useHasCapability } from "@/lib/hooks/use-capability";
import type { PromoCode, AdminCreatePromoCodeInput, AdminUpdatePromoCodeInput, PartnerQuotaUnit } from "@/lib/types";

type DiscountType = "percent" | "fixed";

interface PromoModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  promo?: PromoCode | null;
  onSubmit: (input: AdminCreatePromoCodeInput | AdminUpdatePromoCodeInput) => void;
  isPending: boolean;
}

function dateInputValue(iso?: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toISOString().slice(0, 10);
}

function expiryIso(date: string): string | undefined {
  if (!date) return undefined;
  return new Date(date).toISOString();
}

export function PromoModal({ open, onOpenChange, promo, onSubmit, isPending }: PromoModalProps) {
  const isEdit = Boolean(promo);
  const [code, setCode] = useState("");
  const [discountType, setDiscountType] = useState<DiscountType>("percent");
  const [discountValue, setDiscountValue] = useState("");
  const [maxDiscountAmount, setMaxDiscountAmount] = useState("");
  const [minOrderAmount, setMinOrderAmount] = useState("");
  const [maxUses, setMaxUses] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [isPublic, setIsPublic] = useState(false);
  const [affiliatorOnly, setAffiliatorOnly] = useState(false);
  const canManagePartners = useHasCapability("affiliates:*");
  const [partnerOn, setPartnerOn] = useState(false);
  const [partnerIds, setPartnerIds] = useState<string[]>([]);
  // The quota unit has no default: the admin has to choose what max_uses counts.
  const [partnerUnit, setPartnerUnit] = useState<PartnerQuotaUnit | "">("");

  useEffect(() => {
    if (!open) return;
    if (promo) {
      setCode(promo.code ?? "");
      const isPercent = promo.discount_percent != null;
      setDiscountType(isPercent ? "percent" : "fixed");
      setDiscountValue(
        isPercent
          ? String(promo.discount_percent ?? "")
          : String(promo.discount_amount ?? "")
      );
      setMaxDiscountAmount(promo.max_discount_amount != null ? String(promo.max_discount_amount) : "");
      setMinOrderAmount(promo.min_order_amount != null ? String(promo.min_order_amount) : "");
      setMaxUses(promo.max_uses != null ? String(promo.max_uses) : "");
      setExpiresAt(dateInputValue(promo.expires_at));
      setIsPublic(Boolean(promo.is_public));
      setAffiliatorOnly(Boolean(promo.affiliator_only));
      setPartnerOn(Boolean(promo.partner));
      setPartnerIds(promo.partner?.ids ?? []);
      setPartnerUnit(promo.partner?.quota_unit ?? "");
    } else {
      setCode("");
      setDiscountType("percent");
      setDiscountValue("");
      setMaxDiscountAmount("");
      setMinOrderAmount("");
      setMaxUses("");
      setExpiresAt("");
      setIsPublic(false);
      setAffiliatorOnly(false);
      setPartnerOn(false);
      setPartnerIds([]);
      setPartnerUnit("");
    }
  }, [open, promo]);

  const canSubmit =
    code.trim() !== "" &&
    discountValue !== "" &&
    Number(discountValue) >= 0 &&
    (!partnerOn || partnerUnit !== "") &&
    !isPending;

  // The Mitra section is sent only to an admin who can manage Mitra; on an edit it
  // is also left out when the promo was never a Mitra voucher and still is not.
  function partnerVoucher(): { partner_voucher?: { enabled: boolean; ids?: string[]; quota_unit?: PartnerQuotaUnit } } {
    if (!canManagePartners) return {};
    if (partnerOn && partnerUnit !== "") return { partner_voucher: { enabled: true, ids: partnerIds, quota_unit: partnerUnit } };
    if (isEdit && promo?.partner) return { partner_voucher: { enabled: false } };
    return {};
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit || isPending) return;

    if (isEdit) {
      // is_public is a plain bool on the wire (not a pointer), so an omitted
      // key would reset it to false server-side — it must always be sent.
      const input: AdminUpdatePromoCodeInput = {
        ...(maxUses !== "" ? { max_uses: Number(maxUses) } : {}),
        ...(expiresAt ? { expires_at: expiryIso(expiresAt) } : {}),
        is_public: isPublic && !affiliatorOnly && !partnerOn,
        affiliator_only: affiliatorOnly,
        ...partnerVoucher(),
      };
      onSubmit(input);
      return;
    }

    const base: AdminCreatePromoCodeInput = {
      code: code.trim(),
      ...(maxUses !== "" ? { max_uses: Number(maxUses) } : {}),
      ...(minOrderAmount !== "" ? { min_order_amount: Number(minOrderAmount) } : {}),
      ...(expiresAt ? { expires_at: expiryIso(expiresAt) } : {}),
      is_public: isPublic && !affiliatorOnly && !partnerOn,
      affiliator_only: affiliatorOnly,
      ...partnerVoucher(),
    };

    if (discountType === "percent") {
      base.discount_percent = Number(discountValue);
      if (maxDiscountAmount !== "") {
        base.max_discount_amount = Number(maxDiscountAmount);
      }
    } else {
      base.discount_amount = Number(discountValue);
    }

    onSubmit(base);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{isEdit ? "Edit kode promo" : "Buat kode promo"}</DialogTitle>
            <DialogDescription>
              {isEdit
                ? "Perbarui batas penggunaan dan tanggal kadaluarsa kode promo."
                : "Tambahkan kode promo baru ke katalog."}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="promo-code">Kode</Label>
              <Input
                id="promo-code"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder="contoh: DISKON10"
                disabled={isEdit || isPending}
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="promo-discount-type">Jenis diskon</Label>
                <select
                  id="promo-discount-type"
                  value={discountType}
                  onChange={(e) => setDiscountType(e.target.value as DiscountType)}
                  disabled={isEdit || isPending}
                  className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
                >
                  <option value="percent">Persen</option>
                  <option value="fixed">Nominal tetap</option>
                </select>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="promo-discount-value">Nilai diskon</Label>
                <Input
                  id="promo-discount-value"
                  type="number"
                  min={0}
                  step={discountType === "percent" ? 0.01 : 1}
                  value={discountValue}
                  onChange={(e) => setDiscountValue(e.target.value)}
                  placeholder={discountType === "percent" ? "10" : "20000"}
                  disabled={isEdit || isPending}
                />
              </div>
            </div>

            {discountType === "percent" && !isEdit && (
              <div className="grid gap-2">
                <Label htmlFor="promo-max-discount">Maksimal potongan (IDR)</Label>
                <Input
                  id="promo-max-discount"
                  type="number"
                  min={0}
                  value={maxDiscountAmount}
                  onChange={(e) => setMaxDiscountAmount(e.target.value)}
                  placeholder="0"
                  disabled={isPending}
                />
              </div>
            )}

            {!isEdit && (
              <div className="grid gap-2">
                <Label htmlFor="promo-min-order">Minimal pembelian (IDR)</Label>
                <Input
                  id="promo-min-order"
                  type="number"
                  min={0}
                  value={minOrderAmount}
                  onChange={(e) => setMinOrderAmount(e.target.value)}
                  placeholder="0"
                  disabled={isPending}
                />
              </div>
            )}

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="promo-max-uses">Maksimal penggunaan</Label>
                <Input
                  id="promo-max-uses"
                  type="number"
                  min={0}
                  value={maxUses}
                  onChange={(e) => setMaxUses(e.target.value)}
                  placeholder="Tanpa batas"
                  disabled={isPending}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="promo-expires">Kadaluarsa</Label>
                <Input
                  id="promo-expires"
                  type="date"
                  value={expiresAt}
                  onChange={(e) => setExpiresAt(e.target.value)}
                  disabled={isPending}
                />
              </div>
            </div>

            <label htmlFor="promo-is-public" className="flex items-center gap-2 text-sm text-ink-900">
              <input
                id="promo-is-public"
                type="checkbox"
                checked={isPublic}
                onChange={(e) => setIsPublic(e.target.checked)}
                disabled={isPending || affiliatorOnly || partnerOn}
              />
              Tampilkan di checkout
            </label>
            <label htmlFor="promo-affiliator-only" className="flex items-center gap-2 text-sm text-ink-900">
              <input
                id="promo-affiliator-only"
                type="checkbox"
                checked={affiliatorOnly}
                onChange={(e) => setAffiliatorOnly(e.target.checked)}
                disabled={isPending || partnerOn}
              />
              Khusus affiliator (tidak tampil ke siswa)
            </label>

            {canManagePartners && (
              <fieldset className="grid gap-3 rounded-lg border border-line p-3" aria-label="Voucher Mitra">
                <label htmlFor="promo-partner" className="flex items-center gap-2 text-sm font-semibold text-ink-900">
                  <input
                    id="promo-partner"
                    type="checkbox"
                    checked={partnerOn}
                    onChange={(e) => setPartnerOn(e.target.checked)}
                    disabled={isPending || affiliatorOnly}
                  />
                  Voucher Mitra (hanya untuk pembelian Mitra)
                </label>
                {partnerOn && (
                  <>
                    <div className="grid gap-1.5">
                      <Label htmlFor="promo-partner-unit">Kuota dihitung</Label>
                      <select
                        id="promo-partner-unit"
                        value={partnerUnit}
                        onChange={(e) => setPartnerUnit(e.target.value as PartnerQuotaUnit | "")}
                        disabled={isPending}
                        className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
                      >
                        <option value="">Pilih satuan kuota</option>
                        <option value="order">Per order: satu order memakai satu kuota</option>
                        <option value="student">Per siswa: tiap siswa memakai satu kuota</option>
                      </select>
                      <p className="text-xs text-ink-500">
                        Per siswa juga membuat maksimal potongan berlaku untuk tiap siswa. Per order menerapkannya sekali untuk seluruh order.
                      </p>
                    </div>
                    <div className="grid gap-1.5">
                      <Label>Mitra yang boleh memakai</Label>
                      <PartnerPicker
                        value={partnerIds}
                        onChange={setPartnerIds}
                        disabled={isPending}
                        emptyLabel="Semua Mitra"
                        searchLabel="Cari Mitra"
                      />
                    </div>
                    <p className="text-xs text-warn">
                      Voucher hanya bisa dipakai pada produk yang dibuka untuk Mitra itu. Atur di form produk.
                    </p>
                  </>
                )}
              </fieldset>
            )}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
            >
              Batal
            </Button>
            <Button type="submit" disabled={!canSubmit || isPending}>
              {isPending ? "Menyimpan..." : "Simpan"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
