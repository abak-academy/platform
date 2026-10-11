"use client";

import { useRef, useState } from "react";
import { Lock, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useCheckout, ordersKeys } from "@/lib/hooks/orders";
import { payWithSnap } from "@/lib/snap";
import { ApiError } from "@/lib/api";
import { digitalConflictMessage } from "@/lib/digital-conflict";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";

export interface SnapCheckoutProps {
  orderId?: string;
  basePath?: string;
  disabled?: boolean;
}

export function SnapCheckout({ orderId, basePath = "/orders", disabled }: SnapCheckoutProps) {
  const router = useRouter();
  const qc = useQueryClient();
  const checkout = useCheckout(basePath);
  const { t } = useTranslation();
  const [conflict, setConflict] = useState<string | null>(null);

  const busy = useRef(false);
  const [paying, setPaying] = useState(false);
  const finish = () => {
    busy.current = false;
    setPaying(false);
  };

  const handleCheckout = () => {
    if (!orderId || busy.current || checkout.isPending || disabled) return;
    busy.current = true;
    setPaying(true);
    setConflict(null);
    checkout.mutate(orderId, {
      onSuccess: async (data) => {
        if (data.free) {
          finish();
          qc.invalidateQueries({ queryKey: ordersKeys.all });
          toast.success("Pesanan gratis berhasil diproses.");
          if (basePath === "/orders") {
            router.push(`/orders/${orderId}`);
          }
        } else {
          try {
            const path = await payWithSnap({
              snapToken: data.snap_token,
              paymentUrl: data.payment_url,
              onFinished: (event) => {
                finish();
                if (event === "error") {
                  toast.error(t("payment_failed"));
                  return;
                }
                qc.invalidateQueries({ queryKey: ordersKeys.all });
                if (event === "success") toast.info(t("payment_received"));
                if (event === "pending") toast.info(t("payment_pending"));
                if (event !== "close" && basePath === "/orders") router.push(`/orders/${orderId}`);
              },
            });
            if (path !== "popup") finish();
            if (path === "blocked" && basePath === "/orders") {
              router.push(`/orders/${orderId}`);
              toast.info(t("checkout_popup_blocked"));
            } else if (path === "new-tab") {
              toast.info(t("payment_new_tab"));
            }
          } catch {
            finish();
            toast.error(t("payment_start_failed"));
          }
        }
      },
      onError: (err) => {
        finish();
        const conflictMsg = digitalConflictMessage(err, t);
        if (conflictMsg) {
          setConflict(conflictMsg);
          toast.error(conflictMsg);
          return;
        }
        if (err instanceof ApiError && err.code === "shipping_required") {
          toast.error(t("shipping_required"));
          return;
        }
        if (err instanceof ApiError && err.code === "biodata_incomplete") {
          toast.error(err.message);
          router.push("/profile");
          return;
        }
        toast.error(err instanceof Error ? err.message : "Gagal memulai checkout.");
      },
    });
  };

  return (
    <>
      <Button
        type="button"
        className="mt-4 w-full"
        size="lg"
        onClick={handleCheckout}
        disabled={!orderId || checkout.isPending || paying || disabled}
      >
        {checkout.isPending || paying ? (
          <Loader2 className="size-4 animate-spin" />
        ) : (
          <Lock className="size-4" />
        )}
        {t("payment_pay")}
      </Button>
      {conflict && (
        <p role="alert" className="mt-3 text-sm text-danger">
          {conflict}
        </p>
      )}
    </>
  );
}
