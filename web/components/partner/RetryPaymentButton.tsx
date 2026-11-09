"use client";

import { useRef, useState } from "react";
import { Loader2, Lock } from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { useRetryPayment } from "@/lib/hooks/orders";
import { partnerKeys } from "@/lib/hooks/partner";
import { payWithSnap } from "@/lib/snap";
import { useTranslation } from "@/lib/i18n";

export function RetryPaymentButton({ orderId }: { orderId: string }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const retry = useRetryPayment("/partners/me/orders");
  const busy = useRef(false);
  const [paying, setPaying] = useState(false);

  const finish = () => {
    busy.current = false;
    setPaying(false);
  };

  const handleClick = () => {
    if (busy.current || retry.isPending) return;
    busy.current = true;
    setPaying(true);
    retry.mutate(orderId, {
      onSuccess: async (res) => {
        try {
          const path = await payWithSnap({
            snapToken: res.snap_token,
            paymentUrl: res.payment_url,
            onFinished: (event) => {
              finish();
              if (event === "error") {
                toast.error(t("payment_failed"));
                return;
              }
              qc.invalidateQueries({ queryKey: partnerKeys.orders() });
              if (event === "success") toast.info(t("payment_received"));
              if (event === "pending") toast.info(t("payment_pending"));
            },
          });
          if (path !== "popup") finish();
          if (path === "new-tab") toast.info(t("payment_new_tab"));
        } catch {
          finish();
          toast.error(t("order_snap_unavailable"));
        }
      },
      onError: (err) => {
        finish();
        qc.invalidateQueries({ queryKey: partnerKeys.orders() });
        toast.error(t("order_retry_failed_title"), { description: err instanceof Error ? err.message : undefined });
      },
    });
  };

  return (
    <Button type="button" size="sm" variant="outline" onClick={handleClick} disabled={retry.isPending || paying}>
      {retry.isPending || paying ? <Loader2 className="size-4 animate-spin" /> : <Lock className="size-4" />}
      {t("payment_pay")}
    </Button>
  );
}
