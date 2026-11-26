"use client";

import Link from "next/link";
import { AlertCircle } from "lucide-react";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";

export interface PendingBannerProps {
  id: string;
  product?: string;
  amount: number;
}

export function PendingBanner({ id, product, amount }: PendingBannerProps) {
  const { t } = useTranslation();
  return (
    <div
      className="mb-8 flex items-center gap-4 rounded-[var(--radius-card)] border p-4"
      style={{
        borderColor: "var(--color-warn)",
        background: "var(--color-warn-bg)",
      }}
    >
      <AlertCircle className="size-5 shrink-0 text-warn" />
      <div className="flex-1 min-w-0">
        <p className="font-semibold text-warn">{t("stu_home_pending_title")}</p>
        <p className="truncate text-sm text-ink-600">
          {product ? `${product} · ` : ""}
          {formatRupiah(amount)}
        </p>
      </div>
      <Button asChild size="sm">
        <Link href={`/orders/${id}`}>{t("stu_home_pending_pay")}</Link>
      </Button>
    </div>
  );
}