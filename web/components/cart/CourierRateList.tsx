"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { CourierRate } from "@/lib/types";

// courierRateKey uniquely identifies a quoted rate by carrier + service, since
// a single carrier (e.g. JNE) can offer multiple services at different prices.
export function courierRateKey(rate: { courier: string; service: string }): string {
  return `${rate.courier}::${rate.service}`;
}

interface CourierRateListProps {
  rates: CourierRate[] | undefined;
  selectedKey: string | null;
  onSelect: (rate: CourierRate) => void;
  isLoading: boolean;
  isError: boolean;
}

export function CourierRateList({
  rates,
  selectedKey,
  onSelect,
  isLoading,
  isError,
}: CourierRateListProps) {
  const { t } = useTranslation();

  if (isLoading) {
    return (
      <div className="flex flex-col gap-3 rounded-lg border border-line bg-surface p-5">
        <div className="h-6 w-24 animate-pulse rounded bg-line" />
        <div className="space-y-3">
          {[0, 1].map((i) => (
            <div key={i} className="h-20 animate-pulse rounded bg-line" />
          ))}
        </div>
      </div>
    );
  }

  if (isError || !rates || rates.length === 0) {
    return (
      <div className="rounded-lg border border-line bg-surface p-5 text-center">
        <p className="text-sm text-ink-500">
          {t("cart_shipping_error") || "Unable to calculate shipping cost"}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-line bg-surface p-5">
      <h3 className="font-semibold text-ink-900">
        {t("cart_shipping_options") || "Shipping Options"}
      </h3>

      <div className="space-y-3">
        {rates.map((rate) => {
          const isSelected = selectedKey === courierRateKey(rate);
          return (
            <Card
              key={courierRateKey(rate)}
              className={`cursor-pointer transition-all ${
                isSelected
                  ? "border-brand-400 bg-brand-50"
                  : "border-line hover:border-brand-200"
              }`}
              role="radio"
              aria-checked={isSelected}
              tabIndex={0}
              onClick={() => onSelect(rate)}
            >
              <div className="p-4">
                <div className="flex items-start justify-between">
                  <div className="flex flex-1 flex-col gap-1">
                    <div className="font-semibold text-ink-900">
                      {rate.courier.toUpperCase()}
                      {rate.is_estimate && (
                        <span className="ml-2 rounded bg-warn-bg px-1.5 py-0.5 text-xs text-warn">
                          {t("cart_rate_estimate_badge" as any)}
                        </span>
                      )}
                    </div>
                    <div className="text-sm text-ink-500">{rate.service}</div>
                    <div className="text-xs text-ink-400">
                      {rate.estimated_days} {rate.estimated_days === 1 ? "day" : "days"}
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="font-semibold text-ink-900">
                      {formatRupiah(rate.price)}
                    </div>
                  </div>
                </div>
              </div>
            </Card>
          );
        })}
      </div>
    </div>
  );
}
