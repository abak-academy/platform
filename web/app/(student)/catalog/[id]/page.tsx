"use client";

import { use, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Award, Book, ShoppingCart, PlayCircle, ClipboardList, Minus, Plus, Package } from "lucide-react";
import { toast } from "sonner";

import { useProduct } from "@/lib/hooks/products";
import { useAddToCart, useCart } from "@/lib/hooks/orders";
import { useCartStore } from "@/stores/cart";
import { useAuthStore } from "@/stores/auth";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import { ApiError, fileUrl } from "@/lib/api";
import type { ProductType } from "@/lib/types";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { ProductSpecTable } from "@/components/catalog/ProductSpecTable";

const TYPE_META: Record<
  ProductType,
  { labelKey: string; tone: string; bg: string; Icon: typeof Book }
> = {
  book: { labelKey: "product_type_book", tone: "text-warn", bg: "bg-warn-bg", Icon: Book },
  course: { labelKey: "product_type_course", tone: "text-success", bg: "bg-success-bg", Icon: PlayCircle },
  exam: { labelKey: "product_type_exam", tone: "text-info", bg: "bg-info-bg", Icon: ClipboardList },
  merchandise: { labelKey: "product_type_merchandise", tone: "text-warn", bg: "bg-warn-bg", Icon: Package },
  medal: { labelKey: "product_type_medal", tone: "text-warn", bg: "bg-warn-bg", Icon: Award },
};

const COVER_GRADIENT: Record<ProductType, string> = {
  book: "linear-gradient(135deg, #fbf1e2 0%, #f6e6cf 100%)",
  course: "linear-gradient(135deg, #e5f5ec 0%, #d4eede 100%)",
  exam: "linear-gradient(135deg, #e7eefb 0%, #d3e2f8 100%)",
  merchandise: "linear-gradient(135deg, #efeafc 0%, #e2d8f8 100%)",
  medal: "linear-gradient(135deg, #fff3cd 0%, #ffe69c 100%)",
};

export default function ProductDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { t } = useTranslation();
  const { id } = use(params);
  const router = useRouter();
  const { data: product, isLoading, isError, error, refetch } = useProduct(id);
  const addToCart = useAddToCart();
  const { data: cart } = useCart();
  const bumpBadge = useCartStore((s) => s.setCount);
  const token = useAuthStore((s) => s.token);
  const [qty, setQty] = useState(1);

  if (isLoading) return <DetailSkeleton />;

  if (isError || !product) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 md:px-6">
        <div className="rounded-lg border border-danger/30 bg-danger-bg px-5 py-4 text-sm text-danger">
          <p>{t("product_load_failed")} {(error as Error)?.message}</p>
          <button onClick={() => refetch()} className="mt-2 underline">
            {t("retry")}
          </button>
        </div>
      </div>
    );
  }

  const meta = TYPE_META[product.type];
  const { Icon } = meta;
  const cover = fileUrl(product.image_url);
  const alreadyInCart = cart?.items?.some((i) => i.product_id === product.id) ?? false;

  const handleAdd = (thenRoute?: () => void) => {
    if (!token) {
      router.push("/login");
      return;
    }
    if (alreadyInCart) {
      thenRoute?.() ?? router.push("/cart");
      return;
    }
    addToCart.mutate(
      { productId: product.id, qty },
      {
        onSuccess: () => {
          bumpBadge(useCartStore.getState().count + 1);
          toast.success(t("product_added_toast"), {
            description: product.name,
          });
          thenRoute?.();
        },
        onError: (err) => {
          const msg = err instanceof ApiError ? err.message : t("product_add_failed_desc");
          toast.error(t("product_add_failed_title"), { description: msg });
        },
      },
    );
  };

  return (
    <div className="mx-auto max-w-5xl px-4 py-6 md:px-6 md:py-10">
      <Button asChild variant="ghost" size="sm" className="mb-4">
        <Link href="/catalog">
          <ArrowLeft className="size-4" />
          {t("product_back_catalog")}
        </Link>
      </Button>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[1fr_340px] md:gap-8">
        <div className="flex flex-col gap-6">
          <div
            className="flex h-64 items-center justify-center overflow-hidden rounded-lg border border-line md:h-72"
            style={
              cover
                ? { backgroundImage: `url(${cover})`, backgroundSize: "cover", backgroundPosition: "center" }
                : { background: COVER_GRADIENT[product.type], border: 0 }
            }
          >
            {!cover && (
              <Icon className="size-16 text-white/90 drop-shadow-sm" strokeWidth={1.5} />
            )}
          </div>

          <div className="flex flex-col gap-3">
            <div className="flex items-center gap-3">
              <Badge variant="outline" className={cn("border-transparent", meta.bg, meta.tone)}>
                {t(meta.labelKey as any)}
              </Badge>
              <h1 className="font-serif text-2xl font-bold text-ink-900 md:text-3xl">
                {product.name}
              </h1>
            </div>
            <p className="max-w-2xl text-sm leading-relaxed text-ink-600 md:text-[15px]">
              {product.description ?? t("product_no_description")}
            </p>
            {(product.type === "book" || product.type === "merchandise" || product.type === "medal") && (
              <p className="text-xs text-ink-500">
                {t("product_stock_label")}: {product.stock ?? 0} · {t("product_shipped_to_address")}
              </p>
            )}
          </div>

          <ProductSpecTable specs={product.specs} />
        </div>

        <aside className="md:sticky md:top-6 md:self-start">
          <div className="rounded-lg border border-line bg-surface p-5 shadow-[var(--sh-sm)]">
            <div className="font-serif text-3xl font-bold text-success">
              {formatRupiah(product.price)}
            </div>
            {(product.type === "book" || product.type === "merchandise" || product.type === "medal") && (
              <div className="mt-1 text-xs text-ink-500">
                {t("product_stock_label")}: {product.stock ?? 0}
              </div>
            )}
            <div className="my-4 h-px bg-line" />
            {!alreadyInCart && (
              <div className="mb-3 flex items-center justify-between">
                <span className="text-sm text-ink-600">{t("product_qty_label")}</span>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => setQty((q) => Math.max(1, q - 1))}
                    disabled={qty <= 1}
                    className="flex size-8 items-center justify-center rounded-full border border-line text-ink-600 hover:bg-paper disabled:opacity-40"
                  >
                    <Minus className="size-3.5" />
                  </button>
                  <span className="w-6 text-center text-sm font-semibold text-ink-900">{qty}</span>
                  <button
                    type="button"
                    onClick={() => setQty((q) => Math.min(10, q + 1))}
                    disabled={qty >= 10}
                    className="flex size-8 items-center justify-center rounded-full border border-line text-ink-600 hover:bg-paper disabled:opacity-40"
                  >
                    <Plus className="size-3.5" />
                  </button>
                </div>
              </div>
            )}
            <div className="flex flex-col gap-3">
              <Button
                size="lg"
                className="w-full"
                disabled={addToCart.isPending}
                onClick={() => alreadyInCart ? router.push("/cart") : handleAdd()}
              >
                <ShoppingCart className="size-4" />
                {alreadyInCart ? t("product_view_cart") : t("product_add_cart")}
              </Button>
              <Button
                size="lg"
                variant="secondary"
                className="w-full"
                disabled={addToCart.isPending}
                onClick={() => handleAdd(() => router.push("/cart"))}
              >
                {t("product_buy_now")}
              </Button>
            </div>
          </div>
        </aside>
      </div>
    </div>
  );
}

function DetailSkeleton() {
  return (
    <div className="mx-auto max-w-5xl px-4 py-6 md:px-6 md:py-10">
      <Skeleton className="mb-4 h-8 w-24" />
      <div className="grid grid-cols-1 gap-6 md:grid-cols-[1fr_340px] md:gap-8">
        <div className="flex flex-col gap-6">
          <Skeleton className="h-72 w-full rounded-lg" />
          <div className="flex flex-col gap-3">
            <Skeleton className="h-6 w-2/3" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-5/6" />
          </div>
        </div>
        <Skeleton className="h-56 w-full rounded-lg" />
      </div>
    </div>
  );
}
