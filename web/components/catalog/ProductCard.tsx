import Link from "next/link";
import { Award, Book, PlayCircle, ClipboardList, Package } from "lucide-react";
import type { Product, ProductType } from "@/lib/types";
import { formatRupiah } from "@/lib/format";
import { fileUrl } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const TYPE_META: Record<
  ProductType,
  { label: string; tone: string; bg: string; Icon: typeof Book }
> = {
  book: { label: "Buku", tone: "text-warn", bg: "bg-warn-bg", Icon: Book },
  course: { label: "Kursus", tone: "text-success", bg: "bg-success-bg", Icon: PlayCircle },
  exam: { label: "Ujian", tone: "text-info", bg: "bg-info-bg", Icon: ClipboardList },
  merchandise: { label: "Merchandise", tone: "text-warn", bg: "bg-warn-bg", Icon: Package },
  medal: { label: "Medali", tone: "text-warn", bg: "bg-warn-bg", Icon: Award },
};

const COVER_GRADIENT: Record<ProductType, string> = {
  book: "linear-gradient(135deg, #fbf1e2 0%, #f6e6cf 100%)",
  course: "linear-gradient(135deg, #e5f5ec 0%, #d4eede 100%)",
  exam: "linear-gradient(135deg, #e7eefb 0%, #d3e2f8 100%)",
  merchandise: "linear-gradient(135deg, #efeafc 0%, #e2d8f8 100%)",
  medal: "linear-gradient(135deg, #fff3cd 0%, #ffe69c 100%)",
};

export interface ProductCardProps {
  product: Product;
  className?: string;
}

export function ProductCard({ product, className }: ProductCardProps) {
  const meta = TYPE_META[product.type];
  const { Icon } = meta;
  const cover = fileUrl(product.image_url);

  return (
    <Link
      href={`/catalog/${product.id}`}
      className={cn(
        "group flex flex-col overflow-hidden rounded-lg border border-line bg-surface shadow-[var(--sh-sm)] transition-all hover:-translate-y-0.5 hover:shadow-[var(--sh-md)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
        className,
      )}
    >
      <div
        data-testid="product-cover"
        className="relative flex aspect-[3/4] items-center justify-center bg-paper"
        style={cover ? undefined : { background: COVER_GRADIENT[product.type] }}
      >
        {cover ? (
          <img
            src={cover}
            alt={product.name}
            loading="lazy"
            className="size-full object-contain p-2"
          />
        ) : (
          <Icon className="size-10 text-white/90 drop-shadow-sm" strokeWidth={1.5} />
        )}
        <div className="absolute left-2 top-2">
          <Badge variant="outline" className={cn("border-transparent", meta.bg, meta.tone)}>
            {meta.label}
          </Badge>
        </div>
      </div>
      <div className="flex flex-1 flex-col gap-1 p-3">
        <div className="line-clamp-2 text-sm font-semibold leading-snug text-ink-900">
          {product.name}
        </div>
        <div className="mt-auto pt-2 font-serif text-base font-bold text-success">
          {formatRupiah(product.price)}
        </div>
      </div>
    </Link>
  );
}
