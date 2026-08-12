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
import type { PromoCode, AdminCreatePromoCodeInput, AdminUpdatePromoCodeInput } from "@/lib/types";

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
    } else {
      setCode("");
      setDiscountType("percent");
      setDiscountValue("");
      setMaxDiscountAmount("");
      setMinOrderAmount("");
      setMaxUses("");
      setExpiresAt("");
      setIsPublic(false);
    }
  }, [open, promo]);

  const canSubmit =
    code.trim() !== "" &&
    discountValue !== "" &&
    Number(discountValue) >= 0 &&
    !isPending;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit || isPending) return;

    if (isEdit) {
      // is_public is a plain bool on the wire (not a pointer), so an omitted
      // key would reset it to false server-side — it must always be sent.
      const input: AdminUpdatePromoCodeInput = {
        ...(maxUses !== "" ? { max_uses: Number(maxUses) } : {}),
        ...(expiresAt ? { expires_at: expiryIso(expiresAt) } : {}),
        is_public: isPublic,
      };
      onSubmit(input);
      return;
    }

    const base: AdminCreatePromoCodeInput = {
      code: code.trim(),
      ...(maxUses !== "" ? { max_uses: Number(maxUses) } : {}),
      ...(minOrderAmount !== "" ? { min_order_amount: Number(minOrderAmount) } : {}),
      ...(expiresAt ? { expires_at: expiryIso(expiresAt) } : {}),
      is_public: isPublic,
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
                disabled={isPending}
              />
              Tampilkan di checkout
            </label>
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
