"use client";

import { Lock, Loader2, ExternalLink } from "lucide-react";
import { toast } from "sonner";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useCheckout, ordersKeys } from "@/lib/hooks/orders";
import { ApiError } from "@/lib/api";
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

  const handleCheckout = () => {
    if (!orderId) return;
    checkout.mutate(orderId, {
      onSuccess: (data) => {
        if (data.free) {
          qc.invalidateQueries({ queryKey: ordersKeys.all });
          toast.success("Pesanan gratis berhasil diproses.");
          if (basePath === "/orders") {
            router.push(`/orders/${orderId}`);
          }
        } else if (data.payment_url) {
          window.open(data.payment_url, "_blank");
          toast.info("Selesaikan pembayaran di tab baru, lalu refresh halaman ini.");
        } else if (data.snap_token) {
          navigator.clipboard.writeText(data.snap_token);
          toast.info("Snap token disalin. Buka Midtrans untuk melanjutkan pembayaran.");
        } else {
          toast.error("Gagal memulai pembayaran. Coba lagi nanti.");
        }
      },
      onError: (err) => {
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
    <Button
      type="button"
      className="mt-4 w-full"
      size="lg"
      onClick={handleCheckout}
      disabled={!orderId || checkout.isPending || disabled}
    >
      {checkout.isPending ? (
        <Loader2 className="size-4 animate-spin" />
      ) : (
        <ExternalLink className="size-4" />
      )}
      Bayar di Tab Baru
    </Button>
  );
}
