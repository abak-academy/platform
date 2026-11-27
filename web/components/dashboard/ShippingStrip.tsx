"use client";

import Link from "next/link";
import { Check, Copy, Truck } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { physicalItems, shipmentSteps } from "@/lib/student-home";
import type { Order } from "@/lib/types";
import { cn } from "@/lib/utils";

const STEP_KEY = {
  paid: "stu_home_ship_step_paid",
  packed: "stu_home_ship_step_packed",
  shipped: "stu_home_ship_step_shipped",
  delivered: "stu_home_ship_step_received",
} as const;

export function ShippingStrip({ order }: { order: Order }) {
  const { t, lang } = useTranslation();
  const dateFmt = new Intl.DateTimeFormat(lang === "id" ? "id-ID" : "en-US", { day: "numeric", month: "short", timeZone: "Asia/Jakarta" });
  const goods = physicalItems(order).map((i) => i.name).join(", ");
  const courier = [order.selected_courier, order.selected_service].filter(Boolean).join(" ");
  const resi = order.tracking_number;

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(resi!);
      toast.success(t("stu_home_ship_resi_copied"));
    } catch {
      toast.error(t("stu_home_ship_copy_failed"));
    }
  };

  return (
    <section className="flex flex-wrap items-center gap-5 rounded-[20px] border border-line bg-surface px-[18px] py-3.5">
      <div className="flex min-w-[200px] items-center gap-2.5">
        <span className="grid size-9 flex-none place-items-center rounded-xl bg-ember-50 text-ember-700">
          <Truck className="size-5" />
        </span>
        <div>
          <b className="block font-[family-name:var(--font-display)] text-sm font-extrabold text-ink-900">
            {t(order.status === "shipped" ? "stu_home_ship_shipped" : "stu_home_ship_processing")}
          </b>
          <small className="text-xs text-ink-600">{goods}</small>
        </div>
      </div>
      <ol aria-label={t("stu_home_ship_progress")} className="m-0 flex min-w-[300px] flex-1 list-none p-0">
        {shipmentSteps(order).map((s) => (
          <li
            key={s.key}
            data-reached={s.reached}
            className={cn(
              "relative flex flex-1 flex-col items-center gap-0.5 text-center text-[11px] text-ink-600",
              "before:absolute before:top-2.5 before:left-[calc(-50%+15px)] before:h-0.5 before:w-[calc(100%-30px)] before:rounded-[1px] before:content-[''] first:before:hidden",
              s.reached ? "before:bg-teal-600" : "before:bg-line",
            )}
          >
            <i
              className={cn(
                "relative mb-1 box-border grid size-[22px] place-items-center rounded-full border-2 text-white not-italic",
                s.reached ? "border-teal-600 bg-teal-600" : "border-line bg-surface",
              )}
            >
              {s.reached && <Check className="size-3" strokeWidth={2.5} />}
            </i>
            <b className={cn("font-[family-name:var(--font-display)] text-xs leading-[1.2] font-bold", s.reached ? "text-ink-900" : "text-ink-600")}>
              {t(STEP_KEY[s.key])}
            </b>
            <small className="text-[11px] leading-[1.2]">{s.date ? dateFmt.format(new Date(s.date)) : "—"}</small>
          </li>
        ))}
      </ol>
      <div className="flex items-center gap-2.5">
        {resi ? (
          <span className="flex flex-col gap-[3px]">
            <small className="text-[11px] text-ink-600">
              {[courier, t("stu_home_ship_resi_label")].filter(Boolean).join(" · ")}
            </small>
            <button
              type="button"
              onClick={copy}
              aria-label={t("stu_home_ship_copy_resi")}
              className="inline-flex cursor-pointer items-center gap-1.5 rounded-[10px] border border-dashed border-line bg-surface-sunken px-2.5 py-1.5 font-mono text-[12.5px] font-bold text-ink-900"
            >
              {resi}
              <Copy className="size-4" />
            </button>
          </span>
        ) : (
          <small className="text-[11px] text-ink-600">{t("stu_home_ship_no_resi")}</small>
        )}
        <Button asChild variant="outline" size="sm" className="px-[13px]">
          <Link href={`/orders/${order.id}`}>{t("stu_home_ship_detail")}</Link>
        </Button>
      </div>
    </section>
  );
}
