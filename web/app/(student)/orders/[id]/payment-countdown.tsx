"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslation } from "@/lib/i18n";

export function formatCountdown(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const minutes = Math.floor(s / 60);
  const secs = s % 60;
  return `${String(minutes).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
}

function remainingSeconds(expiresAt: string): number {
  return Math.ceil((new Date(expiresAt).getTime() - Date.now()) / 1000);
}

export function PaymentCountdown({
  expiresAt,
  onExpired,
}: {
  expiresAt: string;
  onExpired?: () => void;
}) {
  const { t } = useTranslation();
  const [seconds, setSeconds] = useState(() => remainingSeconds(expiresAt));
  const firedRef = useRef(false);

  useEffect(() => {
    const tick = () => {
      const s = remainingSeconds(expiresAt);
      setSeconds(s);
      if (s <= 0 && !firedRef.current) {
        firedRef.current = true;
        onExpired?.();
      }
    };
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [expiresAt, onExpired]);

  if (seconds <= 0) {
    return <p className="mt-1 font-medium text-danger">{t("order_payment_pending_expired")}</p>;
  }
  return (
    <p className="mt-1 text-ink-600">
      {t("order_payment_pending_countdown")}{" "}
      <span className="font-semibold tabular-nums text-ink-900">{formatCountdown(seconds)}</span>
    </p>
  );
}
