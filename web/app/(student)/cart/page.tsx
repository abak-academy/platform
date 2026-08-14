"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { ArrowLeft, ShoppingCart, X } from "lucide-react";
import { useCart, useRemoveCartItem, useUpdateCartItemQty, useValidatePromo, useShippingRates, usePatchCart, useActivePromoCodes } from "@/lib/hooks/orders";
import { useProfile, useUpdateProfile } from "@/lib/hooks/students";
import { useCitiesByProvince, useDistrictsByCity, useProvinces } from "@/lib/hooks/regions";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type { OrderItem } from "@/lib/types";
import { CartLineItem } from "@/components/cart/CartLineItem";
import { PromoSheet } from "@/components/cart/PromoSheet";
import { SnapCheckout } from "@/components/cart/SnapCheckout";
import { ShippingAddressForm, type ShippingAddressFormState } from "@/components/cart/ShippingAddressForm";
import { ShippingAddressSummary, isAddressComplete } from "@/components/cart/ShippingAddressSummary";
import { CourierRateList, courierRateKey } from "@/components/cart/CourierRateList";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { hasPhysicalItems, calculateTotalPhysicalWeight } from "@/lib/shipping";

export default function CartPage() {
  const { t } = useTranslation();
  const { data: cart, isLoading, isError, error, refetch } = useCart();
  const { data: profile } = useProfile();
  const removeItem = useRemoveCartItem();
  const updateQty = useUpdateCartItemQty();
  const validatePromo = useValidatePromo();
  const shippingRates = useShippingRates();
  const patchCart = usePatchCart();
  const updateProfile = useUpdateProfile();
  // FR-14: additive next to the manual input. isError/data undefined just
  // means the list renders nothing — manual entry must keep working either way.
  const { data: activePromos } = useActivePromoCodes();

  const [shippingAddress, setShippingAddress] = useState<ShippingAddressFormState>({
    penerima: "",
    telepon: "",
    alamat: "",
    provinsi_id: "",
    kota_id: "",
    kecamatan_id: "",
    kode_pos: "",
  });
  const [selectedRateKey, setSelectedRateKey] = useState<string | null>(null);
  const [promoError, setPromoError] = useState<string | undefined>(undefined);
  const [appliedPromoCode, setAppliedPromoCode] = useState<string | undefined>(undefined);
  // Once open, the form closes when the buyer closes it, never because it looks
  // full. Deriving this from "every field is non-empty" swapped the form out on
  // the first character of the last field — which is how a one-character
  // postcode got saved. The *initial* value is a different question, settled by
  // the seeding effect below: an address already on file opens collapsed.
  const [addressFormOpen, setAddressFormOpen] = useState(true);
  const [courierNote, setCourierNote] = useState("");
  const [shippingClearedNotice, setShippingClearedNotice] = useState(false);

  const items: OrderItem[] = cart?.items ?? [];
  const subtotal = cart?.subtotal ?? items.reduce((s, it) => s + it.jumlah, 0);
  const discount = cart?.discount ?? 0;
  const total = cart?.total ?? Math.max(0, subtotal - discount + (cart?.shipping_cost ?? 0));

  const hasPhysical = hasPhysicalItems(items);
  const addressReady = isAddressComplete(shippingAddress as any);
  const totalPhysicalWeight = calculateTotalPhysicalWeight(items);

  const handleAddressChange = useCallback((state: ShippingAddressFormState) => {
    setShippingAddress(state);
  }, []);

  // An address the buyer already completed — on this order or on their profile —
  // is not a form to fill in again. Seed it once and open collapsed, so the
  // default path to payment is read-and-continue rather than retype. The order's
  // own address wins: it is the one this delivery was aimed at.
  const addressSeeded = useRef(false);

  useEffect(() => {
    if (addressSeeded.current) return;

    const saved = cart?.shipping_address;
    const seed: ShippingAddressFormState | null = saved?.alamat
      ? {
          penerima: saved.penerima ?? "",
          telepon: saved.telepon ?? "",
          alamat: saved.alamat ?? "",
          provinsi_id: saved.provinsi_id ?? "",
          kota_id: saved.kota_id ?? "",
          kecamatan_id: saved.kecamatan_id ?? "",
          kode_pos: saved.kode_pos ?? "",
          provinsi: saved.provinsi,
          kota: saved.kota,
          kecamatan: saved.kecamatan,
        }
      : profile?.alamat_domisili
        ? {
            penerima: profile.name ?? "",
            telepon: profile.phone ?? "",
            alamat: profile.alamat_domisili ?? "",
            provinsi_id: profile.provinsi_id ?? "",
            kota_id: profile.kota_id ?? "",
            kecamatan_id: profile.kecamatan_id ?? "",
            kode_pos: profile.kode_pos ?? "",
          }
        : null;

    if (!seed) return;
    addressSeeded.current = true;
    setShippingAddress(seed);
    if (isAddressComplete(seed)) setAddressFormOpen(false);
  }, [cart, profile]);

  // Region names for the order's address snapshot. The form fills these in while
  // it is mounted, but a buyer whose address was already complete never opens
  // it — so they are resolved here too, or the snapshot ships with bare IDs.
  const { data: provinces } = useProvinces();
  const { data: cities } = useCitiesByProvince(shippingAddress.provinsi_id || null);
  const { data: districts } = useDistrictsByCity(shippingAddress.kota_id || null);

  const addressSnapshot = useMemo(
    () => ({
      penerima: shippingAddress.penerima,
      telepon: shippingAddress.telepon,
      alamat: shippingAddress.alamat,
      kode_pos: shippingAddress.kode_pos,
      provinsi_id: shippingAddress.provinsi_id,
      kota_id: shippingAddress.kota_id,
      kecamatan_id: shippingAddress.kecamatan_id,
      provinsi: shippingAddress.provinsi ?? provinces?.find((p) => p.id === shippingAddress.provinsi_id)?.name,
      kota: shippingAddress.kota ?? cities?.find((c) => c.id === shippingAddress.kota_id)?.name,
      kecamatan: shippingAddress.kecamatan ?? districts?.find((d) => d.id === shippingAddress.kecamatan_id)?.name,
    }),
    [shippingAddress, provinces, cities, districts]
  );

  const handleCheckShipping = useCallback(() => {
    if (!shippingAddress.provinsi_id || !shippingAddress.kota_id || !shippingAddress.kecamatan_id || !shippingAddress.kode_pos) return;
    shippingRates.mutate({
      destination_postal_code: shippingAddress.kode_pos,
      weight_grams: totalPhysicalWeight,
    });
  }, [shippingAddress, totalPhysicalWeight, shippingRates]);

  // Saving is what closes the form. The address goes onto the order here rather
  // than riding along with the courier choice, so abandoning the cart before
  // picking one no longer throws the address away.
  //
  // Saving does not price the delivery. Two actions that cost different things —
  // one writes the address, one calls the courier API — were on one button, so a
  // buyer correcting a typo silently re-quoted every courier.
  //
  // Deliberately no promo_code here (FR-5): the promo is attached by its own
  // patch when the buyer presses "Pakai", not re-sent from every cart mutation.
  const handleSaveAddress = useCallback(
    (saveAsPrimary: boolean) => {
      if (!cart) return;

      patchCart.mutate({
        orderId: cart.id,
        province_id: shippingAddress.provinsi_id,
        city_id: shippingAddress.kota_id,
        district_id: shippingAddress.kecamatan_id,
        kode_pos: shippingAddress.kode_pos,
        shipping_address: addressSnapshot,
      });

      // Only on request: a buyer shipping one order to someone else must not
      // lose their own address to it.
      if (saveAsPrimary) {
        updateProfile.mutate({
          address: shippingAddress.alamat,
          phone: shippingAddress.telepon,
          provinsi_id: shippingAddress.provinsi_id,
          kota_id: shippingAddress.kota_id,
          kecamatan_id: shippingAddress.kecamatan_id,
          kode_pos: shippingAddress.kode_pos,
        });
      }

      // Rates quoted for the old destination are not rates for this one, and a
      // courier picked against them would be priced wrong at checkout. Compared
      // against the persisted snapshot rather than local rate state, so this
      // still fires after a reload — ratedPostalCode used to start null on every
      // mount and silently never ran.
      const priorAddress = cart.shipping_address;
      const destinationChanged =
        Boolean(priorAddress) &&
        (priorAddress?.provinsi_id !== shippingAddress.provinsi_id ||
          priorAddress?.kota_id !== shippingAddress.kota_id ||
          priorAddress?.kecamatan_id !== shippingAddress.kecamatan_id ||
          priorAddress?.kode_pos !== shippingAddress.kode_pos);

      if (destinationChanged) {
        shippingRates.reset();
        setSelectedRateKey(null);
      }
      setShippingClearedNotice(destinationChanged);

      setAddressFormOpen(false);
    },
    [cart, shippingAddress, addressSnapshot, patchCart, updateProfile, shippingRates]
  );

  // "Primary" is the profile's own address, compared field by field — no column
  // and no migration. It marks which address an order is going to once a buyer
  // can have more than one.
  const isPrimaryAddress =
    Boolean(profile?.alamat_domisili) &&
    profile?.alamat_domisili === shippingAddress.alamat &&
    profile?.provinsi_id === shippingAddress.provinsi_id &&
    profile?.kota_id === shippingAddress.kota_id &&
    profile?.kecamatan_id === shippingAddress.kecamatan_id &&
    profile?.kode_pos === shippingAddress.kode_pos;

  // The rate and the note are stored by the same PATCH, so both have to be sent
  // whichever one the buyer changed — sending only the note would clear the
  // courier the backend already has.
  //
  // Deliberately no promo_code here either (FR-5): this fires on every courier
  // click, and re-sending the code would re-run ValidatePromo each time — a
  // promo that becomes exhausted between apply and checkout would then block
  // courier selection entirely. Omitting it is safe because the backend keeps
  // whatever promo the order already carries when the key is absent.
  const persistShipping = useCallback(
    (rate: { courier: string; service: string; price: number }, note: string) => {
      if (!cart) return;
      patchCart.mutate({
        orderId: cart.id,
        courier: rate.courier,
        service: rate.service,
        shipping_cost: rate.price,
        province_id: shippingAddress.provinsi_id,
        city_id: shippingAddress.kota_id,
        district_id: shippingAddress.kecamatan_id,
        kode_pos: shippingAddress.kode_pos,
        shipping_address: { ...addressSnapshot, catatan: note.trim() || undefined },
      });
    },
    [cart, shippingAddress, addressSnapshot, patchCart]
  );

  const handleSelectCourier = useCallback(
    (rate: { courier: string; service: string; price: number }) => {
      setSelectedRateKey(courierRateKey(rate));
      persistShipping(rate, courierNote);
    },
    [persistShipping, courierNote]
  );

  const selectedRate = shippingRates.data?.find((r) => courierRateKey(r) === selectedRateKey);

  // Before a courier is picked there is nothing to attach the note to; it rides
  // along with the first selection instead.
  const handleNoteBlur = useCallback(() => {
    if (selectedRate) persistShipping(selectedRate, courierNote);
  }, [selectedRate, courierNote, persistShipping]);

  // The dedicated promo patch (FR-5): client-side validation first, then the
  // code is only attached to the order once that succeeds. The displayed
  // discount always comes back from cart.discount after invalidation, never
  // from this validation response.
  const handleApplyPromo = useCallback(
    (code: string) => {
      if (!cart) return;
      setPromoError(undefined);
      validatePromo.mutate(
        { code, orderId: cart.id, subtotal },
        {
          onSuccess: () => {
            patchCart.mutate(
              {
                orderId: cart.id,
                promo_code: code,
                province_id: shippingAddress.provinsi_id,
                city_id: shippingAddress.kota_id,
                district_id: shippingAddress.kecamatan_id,
                kode_pos: shippingAddress.kode_pos || null,
              },
              {
                // Only named once the order actually carries it — the summary
                // must never show a code the buyer is not being charged under.
                onSuccess: () => setAppliedPromoCode(code),
                onError: () => setPromoError(t("cart_promo_invalid")),
              }
            );
          },
          onError: () => setPromoError(t("cart_promo_invalid")),
        }
      );
    },
    [cart, subtotal, shippingAddress, validatePromo, patchCart, t]
  );

  const handleClearPromo = useCallback(() => {
    if (!cart) return;
    setPromoError(undefined);
    setAppliedPromoCode(undefined);
    patchCart.mutate({
      orderId: cart.id,
      promo_code: "",
      province_id: shippingAddress.provinsi_id,
      city_id: shippingAddress.kota_id,
      district_id: shippingAddress.kecamatan_id,
      kode_pos: shippingAddress.kode_pos || null,
    });
  }, [cart, shippingAddress, patchCart]);

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10">
      <Link
        href="/catalog"
        className="mb-4 inline-flex items-center gap-1.5 text-sm font-medium text-ink-500 transition-colors hover:text-ink-900"
      >
        <ArrowLeft className="size-4" /> {t("cart_continue")}
      </Link>

      <header className="mb-6 flex items-center gap-3">
        <ShoppingCart className="size-6 text-success" />
        <h1 className="font-serif text-2xl font-bold text-ink-900 md:text-3xl">{t("cart_title")}</h1>
        {items.length > 0 && (
          <Badge variant="outline" className="border-transparent bg-success-bg text-success">
            {t("cart_item_count").replace("{n}", String(items.length))}
          </Badge>
        )}
      </header>

      {isLoading ? (
        <CartSkeleton />
      ) : isError ? (
        <ErrorState message={error instanceof Error ? error.message : t("cart_load_failed")} onRetry={refetch} />
      ) : items.length === 0 ? (
        <EmptyCart />
      ) : (
        <div className="grid gap-6 lg:grid-cols-[1fr_360px] lg:items-start">
          <section className="flex flex-col gap-3">
            {hasPhysical &&
              (addressFormOpen ? (
                <ShippingAddressForm
                  profile={profile}
                  initialAddress={shippingAddress}
                  onAddressChange={handleAddressChange}
                  onSave={handleSaveAddress}
                  isSaving={shippingRates.isPending || patchCart.isPending}
                />
              ) : (
                <ShippingAddressSummary
                  address={shippingAddress as any}
                  isPrimary={isPrimaryAddress}
                  onEdit={() => setAddressFormOpen(true)}
                />
              ))}

            {/* Items, the note and the courier picker share one card: the
                shipping choice belongs to these goods, not to the page. */}
            <div className="overflow-hidden rounded-lg border border-line bg-surface shadow-[var(--sh-sm)]">
              {items.map((it, idx) => (
                <div key={it.id} className={idx > 0 ? "border-t border-line" : ""}>
                  <CartLineItem
                    item={it}
                    flat
                    onRemove={() => {
                      if (!cart) return;
                      removeItem.mutate({ orderId: cart.id, itemId: it.id });
                    }}
                    onQtyChange={(qty) => {
                      if (!cart) return;
                      updateQty.mutate({ orderId: cart.id, itemId: it.id, qty });
                    }}
                    removing={removeItem.isPending}
                    updatingQty={updateQty.isPending}
                  />
                </div>
              ))}

              {hasPhysical && (addressReady || shippingRates.data) && (
                /* Note beside the shipping choice, both under the goods they
                   apply to. The note is for the courier, so it belongs here and
                   not in the address card. */
                <div className="grid gap-4 border-t border-line px-4 py-4 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
                  <div className="flex flex-col gap-2">
                    <label
                      htmlFor="cart-courier-note"
                      className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-500"
                    >
                      {t("cart_courier_note_label")}
                    </label>
                    <textarea
                      id="cart-courier-note"
                      value={courierNote}
                      onChange={(e) => setCourierNote(e.target.value)}
                      onBlur={handleNoteBlur}
                      rows={3}
                      maxLength={200}
                      placeholder={t("cart_courier_note_placeholder")}
                      className="w-full resize-none rounded-lg border border-line bg-surface px-3 py-2 text-sm text-ink-900 placeholder:text-ink-400 focus-visible:border-brand-500 focus-visible:outline-none"
                    />
                  </div>

                  <div className="flex flex-col gap-2">
                    {shippingRates.data ? (
                      <CourierRateList
                        rates={shippingRates.data}
                        selectedKey={selectedRateKey}
                        onSelect={handleSelectCourier}
                        isLoading={false}
                        isError={shippingRates.isError}
                      />
                    ) : (
                      <>
                        <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-500">
                          {t("cart_shipping_options")}
                        </span>
                        <Button
                          type="button"
                          onClick={handleCheckShipping}
                          disabled={shippingRates.isPending}
                          className="w-full"
                        >
                          {t("cart_check_shipping_cost")}
                        </Button>
                      </>
                    )}
                  </div>
                </div>
              )}
            </div>

            {hasPhysical && shippingRates.isError && (
              <div className="rounded-lg border border-danger/30 bg-danger-bg px-5 py-4 text-sm text-danger">
                {t("cart_shipping_unavailable" as any)}
              </div>
            )}

            {hasPhysical && shippingClearedNotice && (
              <div className="rounded-lg border border-warn/30 bg-warn-bg px-5 py-4 text-sm text-warn">
                {t("cart_shipping_address_changed" as any)}
              </div>
            )}
          </section>

          <aside className="lg:sticky lg:top-6">
            <Card className="p-5">
              <h2 className="font-serif text-lg font-semibold text-ink-900">{t("cart_order_summary")}</h2>

              {/* One row, not a wall of codes: the offers live behind it and
                  the manual code box lives with them. */}
              <PromoSheet
                promos={activePromos ?? []}
                subtotal={subtotal}
                applied={Boolean(cart?.promo_code_id)}
                appliedCode={appliedPromoCode}
                discount={discount}
                isApplying={validatePromo.isPending || patchCart.isPending}
                error={promoError}
                onApply={handleApplyPromo}
                onClear={handleClearPromo}
              />

              <div className="mt-4 space-y-2 border-t border-line pt-4 text-sm">
                <Row label={t("cart_subtotal")} value={formatRupiah(subtotal)} />
                {discount > 0 && <Row label={t("cart_discount")} value={`−${formatRupiah(discount)}`} tone="text-success" />}
                {(cart?.shipping_cost ?? 0) > 0 && <Row label={t("order_shipping")} value={formatRupiah(cart?.shipping_cost ?? 0)} />}
              </div>

              <div className="mt-4 flex items-center justify-between border-t border-line pt-4">
                <span className="font-semibold text-ink-900">{t("cart_total")}</span>
                <span className="font-serif text-2xl font-bold text-success">{formatRupiah(total)}</span>
              </div>

              <SnapCheckout orderId={cart?.id} disabled={hasPhysical && shippingRates.isError} />

              <p className="mt-3 text-center text-xs text-ink-400">
                {t("cart_secure_payment")}
              </p>
            </Card>
          </aside>
        </div>
      )}
    </div>
  );
}

function Row({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-ink-500">{label}</span>
      <span className={`font-semibold ${tone ?? "text-ink-900"}`}>{value}</span>
    </div>
  );
}

function CartSkeleton() {
  return (
    <div className="flex flex-col gap-3">
      {[0, 1, 2].map((i) => (
        <Skeleton key={i} className="h-24 w-full rounded-lg" />
      ))}
    </div>
  );
}

function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  const { t } = useTranslation();
  return (
    <Card className="flex flex-col items-center gap-3 p-10 text-center">
      <X className="size-8 text-danger" />
      <p className="text-sm text-ink-600">{message}</p>
      <Button variant="outline" size="sm" onClick={onRetry}>{t("retry")}</Button>
    </Card>
  );
}

function EmptyCart() {
  const { t } = useTranslation();
  return (
    <Card className="flex flex-col items-center gap-4 p-12 text-center">
      <div className="flex size-16 items-center justify-center rounded-full bg-paper">
        <ShoppingCart className="size-7 text-ink-400" />
      </div>
      <div>
        <h2 className="font-serif text-lg font-semibold text-ink-900">{t("cart_empty_title")}</h2>
        <p className="mt-1 text-sm text-ink-500">{t("cart_empty_desc")}</p>
      </div>
      <Button asChild>
        <Link href="/catalog">{t("cart_view_catalog")}</Link>
      </Button>
    </Card>
  );
}
