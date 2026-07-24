"use client";

import { useCallback, useState } from "react";
import Link from "next/link";
import { ArrowLeft, ShoppingCart, X } from "lucide-react";
import { useCart, useRemoveCartItem, useUpdateCartItemQty, useValidatePromo, useShippingRates, usePatchCart } from "@/lib/hooks/orders";
import { useProfile } from "@/lib/hooks/students";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type { OrderItem } from "@/lib/types";
import { CartLineItem } from "@/components/cart/CartLineItem";
import { PromoInput } from "@/components/cart/PromoInput";
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
  const [editingAddress, setEditingAddress] = useState(false);

  const items: OrderItem[] = cart?.items ?? [];
  const subtotal = cart?.subtotal ?? items.reduce((s, it) => s + it.jumlah, 0);
  const discount = cart?.discount ?? 0;
  const total = cart?.total ?? Math.max(0, subtotal - discount + (cart?.shipping_cost ?? 0));

  const hasPhysical = hasPhysicalItems(items);
  const totalPhysicalWeight = calculateTotalPhysicalWeight(items);

  const handleAddressChange = useCallback((state: ShippingAddressFormState) => {
    setShippingAddress(state);
  }, []);

  const handleCheckShipping = useCallback(() => {
    if (!shippingAddress.provinsi_id || !shippingAddress.kota_id || !shippingAddress.kecamatan_id || !shippingAddress.kode_pos) return;
    shippingRates.mutate({
      destination_postal_code: shippingAddress.kode_pos,
      weight_grams: totalPhysicalWeight,
    });
  }, [shippingAddress, totalPhysicalWeight, shippingRates]);

  const handleSelectCourier = useCallback(
    (rate: { courier: string; service: string; price: number }) => {
      if (!cart) return;
      setSelectedRateKey(courierRateKey(rate));
      patchCart.mutate({
        orderId: cart.id,
        courier: rate.courier,
        service: rate.service,
        shipping_cost: rate.price,
        province_id: shippingAddress.provinsi_id,
        city_id: shippingAddress.kota_id,
        district_id: shippingAddress.kecamatan_id,
        kode_pos: shippingAddress.kode_pos,
        shipping_address: {
          penerima: shippingAddress.penerima,
          telepon: shippingAddress.telepon,
          alamat: shippingAddress.alamat,
          kode_pos: shippingAddress.kode_pos,
          provinsi_id: shippingAddress.provinsi_id,
          kota_id: shippingAddress.kota_id,
          kecamatan_id: shippingAddress.kecamatan_id,
        },
      });
    },
    [cart, shippingAddress, patchCart]
  );

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
            {items.map((it) => (
              <CartLineItem
                key={it.id}
                item={it}
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
            ))}

            {hasPhysical &&
              (editingAddress || !isAddressComplete(shippingAddress as any) ? (
                <ShippingAddressForm
                  profile={profile}
                  onAddressChange={handleAddressChange}
                  onCheckShipping={() => {
                    setEditingAddress(false);
                    handleCheckShipping();
                  }}
                  isCheckingShipping={shippingRates.isPending}
                />
              ) : (
                <ShippingAddressSummary
                  address={shippingAddress as any}
                  onEdit={() => setEditingAddress(true)}
                />
              ))}

            {hasPhysical && shippingRates.data && (
              <CourierRateList
                rates={shippingRates.data}
                selectedKey={selectedRateKey}
                onSelect={handleSelectCourier}
                isLoading={false}
                isError={shippingRates.isError}
              />
            )}
          </section>

          <aside className="lg:sticky lg:top-6">
            <Card className="p-5">
              <h2 className="font-serif text-lg font-semibold text-ink-900">{t("cart_order_summary")}</h2>

              <PromoInput
                onValidate={(code) => validatePromo.mutate({ code, orderId: cart?.id, subtotal })}
                isValidating={validatePromo.isPending}
                discount={validatePromo.data?.discount}
                finalTotal={validatePromo.data?.final_total}
                error={validatePromo.isError ? t("cart_promo_invalid") : undefined}
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

              <SnapCheckout orderId={cart?.id} />

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
