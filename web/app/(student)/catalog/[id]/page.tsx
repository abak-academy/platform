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
  const isDigital = product.type === "exam" || product.type === "course";
  const isPhysical = product.type === "book" || product.type === "merchandise" || product.type === "medal";
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
    <div className="mx-auto max-w-6xl px-4 py-6 md:px-6 md:py-10">
      <Button asChild variant="ghost" size="sm" className="mb-4">
        <Link href="/catalog">
          <ArrowLeft className="size-4" />
          {t("product_back_catalog")}
        </Link>
      </Button>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[minmax(0,1fr)_300px] md:gap-8 lg:grid-cols-[minmax(0,360px)_minmax(0,1fr)_300px]">
        {/* Covers are portrait and merchandise shots are square, so the frame
            contains rather than crops — the old cover band sliced the top and
            bottom off every book. */}
        <div className="md:col-span-2 lg:col-span-1 lg:sticky lg:top-6 lg:self-start">
          <div
            className="mx-auto flex aspect-square w-full max-w-sm items-center justify-center overflow-hidden rounded-lg border border-line lg:max-w-none"
            style={cover ? undefined : { background: COVER_GRADIENT[product.type], border: 0 }}
          >
            {cover ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={cover} alt={product.name} className="h-full w-full object-contain" />
            ) : (
              <Icon className="size-16 text-white/90 drop-shadow-sm" strokeWidth={1.5} />
            )}
          </div>
        </div>

        <div className="flex min-w-0 flex-col gap-5">
          <div className="flex flex-col gap-2">
            <Badge
              variant="outline"
              className={cn("w-fit border-transparent", meta.bg, meta.tone)}
            >
              {t(meta.labelKey as any)}
            </Badge>
            <h1 className="font-serif text-2xl font-bold leading-tight text-ink-900 md:text-3xl">
              {product.name}
            </h1>
            <p className="font-serif text-3xl font-bold text-success">
              {formatRupiah(product.price)}
            </p>
            {isPhysical && (
              <p className="text-xs text-ink-500">
                {t("product_stock_label")}: {product.stock ?? 0} · {t("product_shipped_to_address")}
              </p>
            )}
          </div>

          {/* Specifications sit above the description on purpose. They answer
              what a buyer checks first — pages, edition, ISBN — and used to be
              stranded below a description that can run several hundred words. */}
          <ProductSpecTable specs={product.specs} />

          <ProductDescription text={product.description} />
        </div>

        <aside
          aria-labelledby="product-purchase-heading"
          className="md:sticky md:top-6 md:self-start"
        >
          <div className="rounded-lg border border-line bg-surface p-5 shadow-[var(--sh-sm)]">
            <h2 id="product-purchase-heading" className="text-sm font-semibold text-ink-900">
              {t("product_purchase_heading")}
            </h2>
            <div className="my-4 h-px bg-line" />
            {!alreadyInCart && !isDigital && (
              <div className="mb-4 flex items-center justify-between gap-3">
                <span className="text-sm text-ink-600">
                  {t("product_qty_label")}
                  {isPhysical && (
                    <span className="ml-2 text-xs text-ink-500">
                      {t("product_stock_label")}: {product.stock ?? 0}
                    </span>
                  )}
                </span>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    aria-label={t("product_qty_label")}
                    onClick={() => setQty((q) => Math.max(1, q - 1))}
                    disabled={qty <= 1}
                    className="flex size-8 items-center justify-center rounded-full border border-line text-ink-600 hover:bg-paper disabled:opacity-40"
                  >
                    <Minus className="size-3.5" />
                  </button>
                  <span className="w-6 text-center text-sm font-semibold text-ink-900">{qty}</span>
                  <button
                    type="button"
                    aria-label={t("product_qty_label")}
                    onClick={() => setQty((q) => Math.min(10, q + 1))}
                    disabled={qty >= 10}
                    className="flex size-8 items-center justify-center rounded-full border border-line text-ink-600 hover:bg-paper disabled:opacity-40"
                  >
                    <Plus className="size-3.5" />
                  </button>
                </div>
              </div>
            )}
            {isDigital && (
              <p className="mb-4 text-xs text-ink-500">{t("product_digital_single_qty")}</p>
            )}
            {/* The stepper goes to 10, so what the buyer owes stopped being
                obvious the moment it left 1. */}
            <div className="mb-4 flex items-baseline justify-between">
              <span className="text-sm text-ink-600">{t("product_subtotal")}</span>
              <span className="text-lg font-bold tabular-nums text-ink-900">
                {formatRupiah(product.price * (isDigital ? 1 : qty))}
              </span>
            </div>
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

// Descriptions are typed into a plain textarea, so the paragraph breaks the
// admin makes are bare newlines that HTML collapses — whitespace-pre-line keeps
// them. Long copy is clamped because it otherwise pushes everything else on the
// page below the fold.
const LONG_DESCRIPTION_CHARS = 400;

function ProductDescription({ text }: { text?: string | null }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  // A run of blank lines is almost always stray keystrokes in the textarea, and
  // pre-line renders every one of them as a hole in the copy.
  const body = text?.trim().replace(/\n{3,}/g, "\n\n");

  if (!body) {
    return <p className="text-sm text-ink-500">{t("product_no_description")}</p>;
  }

  const clampable = body.length > LONG_DESCRIPTION_CHARS;

  return (
    <section aria-labelledby="product-description-heading" className="flex flex-col gap-3">
      <h2
        id="product-description-heading"
        className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-500"
      >
        {t("product_description_heading")}
      </h2>
      <div className="relative">
        <p
          className={cn(
            "whitespace-pre-line text-sm leading-relaxed text-ink-600 md:text-[15px]",
            clampable && !expanded && "line-clamp-6",
          )}
        >
          {body}
        </p>
        {clampable && !expanded && (
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-x-0 bottom-0 h-10 bg-gradient-to-t from-paper to-transparent"
          />
        )}
      </div>
      {clampable && (
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="self-start text-sm font-semibold text-brand-600 hover:underline"
        >
          {expanded ? t("product_show_less") : t("product_show_more")}
        </button>
      )}
    </section>
  );
}

function DetailSkeleton() {
  return (
    <div className="mx-auto max-w-6xl px-4 py-6 md:px-6 md:py-10">
      <Skeleton className="mb-4 h-8 w-24" />
      <div className="grid grid-cols-1 gap-6 md:grid-cols-[minmax(0,1fr)_300px] md:gap-8 lg:grid-cols-[minmax(0,360px)_minmax(0,1fr)_300px]">
        <div className="md:col-span-2 lg:col-span-1">
          <Skeleton className="mx-auto aspect-square w-full max-w-sm rounded-lg lg:max-w-none" />
        </div>
        <div className="flex flex-col gap-3">
          <Skeleton className="h-6 w-2/3" />
          <Skeleton className="h-8 w-40" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-5/6" />
        </div>
        <Skeleton className="h-56 w-full rounded-lg" />
      </div>
    </div>
  );
}
