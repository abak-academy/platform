"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ProductCard } from "@/components/catalog/ProductCard";
import { useTranslation } from "@/lib/i18n";
import { shopPicks } from "@/lib/student-home";
import type { Product } from "@/lib/types";

export function ShopPicks({ products, enrolledCourseIds }: { products: Product[]; enrolledCourseIds: string[] }) {
  const { t } = useTranslation();
  const picks = shopPicks(products, enrolledCourseIds);
  if (picks.length === 0) return null;

  return (
    <section className="flex flex-col">
      <div className="mb-3.5 flex flex-wrap items-center justify-between gap-3">
        <h2 className="type-h2 text-ink-900">{t("stu_home_shop_title")}</h2>
        <Button asChild variant="ghost" size="sm" className="px-[13px]">
          <Link href="/catalog">
            {t("stu_home_shop_link")}
            <ArrowRight />
          </Link>
        </Button>
      </div>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(255px,1fr))] gap-[18px]">
        {picks.map((p) => (
          <ProductCard key={p.id} product={p} />
        ))}
      </div>
    </section>
  );
}
