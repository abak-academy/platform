"use client";

import Link from "next/link";
import { ProductCard } from "@/components/catalog/ProductCard";
import { useTranslation } from "@/lib/i18n";
import { shopPicks } from "@/lib/student-home";
import type { Product } from "@/lib/types";

export function ShopPicks({ products, enrolledCourseIds }: { products: Product[]; enrolledCourseIds: string[] }) {
  const { t } = useTranslation();
  const picks = shopPicks(products, enrolledCourseIds);
  if (picks.length === 0) return null;

  return (
    <section>
      <div className="mb-4 flex items-center justify-between">
        <h2 className="font-serif text-xl font-semibold text-ink-900">{t("stu_home_shop_title")}</h2>
        <Link href="/catalog" className="text-sm font-medium text-brand-700 hover:text-brand-800">
          {t("stu_home_shop_link")} →
        </Link>
      </div>
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        {picks.map((p) => (
          <ProductCard key={p.id} product={p} />
        ))}
      </div>
    </section>
  );
}
