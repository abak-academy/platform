"use client";

import Link from "next/link";
import { Hourglass } from "lucide-react";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";

export interface PendingBannerProps {
  id: string;
  product?: string;
  amount: number;
}

// Fixed text colours: the gold gradient stays the same in both themes (prototype .pending-banner).
export function PendingBanner({ id, product, amount }: PendingBannerProps) {
  const { t } = useTranslation();
  return (
    <div
      className="flex items-center gap-3.5 rounded-[22px] bg-[linear-gradient(100deg,var(--color-gold-600),#f3cd6b)] px-[18px] py-4 text-[#3d2b08] max-narrow:flex-wrap"
    >
      <span className="grid size-11 flex-none place-items-center rounded-[14px] bg-white/50">
        <Hourglass className="size-5" aria-hidden />
      </span>
      <div className="min-w-0 flex-1">
        <b className="block font-[family-name:var(--font-display)] text-[15px] font-extrabold">{t("stu_home_pending_title")}</b>
        <span className="block truncate text-[13px]">
          {product ? `${product} · ` : ""}
          {formatRupiah(amount)}
        </span>
      </div>
      <Button asChild variant="light" size="sm" className="px-[13px] text-[#14202b] max-narrow:w-full">
        <Link href={`/orders/${id}`}>{t("stu_home_pending_pay")}</Link>
      </Button>
    </div>
  );
}
