"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslation } from "@/lib/i18n";

export function formatCountdown(totalSeconds: number): string {
  const { minutes, seconds } = splitCountdown(totalSeconds);
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

export function splitCountdown(totalSeconds: number): { minutes: number; seconds: number } {
  const s = Math.max(0, Math.floor(totalSeconds));
  return { minutes: Math.floor(s / 60), seconds: s % 60 };
}

function remainingSeconds(expiresAt: string): number {
  return Math.ceil((new Date(expiresAt).getTime() - Date.now()) / 1000);
}

const pad = (n: number) => String(n).padStart(2, "0");

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
  // The payment window is 20 minutes (backend paymentWindow), so only minutes and seconds are shown.
  const { minutes, seconds: secs } = splitCountdown(seconds);
  return (
    <div className="mt-3">
      <p className="text-ink-700">{t("order_payment_pending_countdown")}</p>
      <div
        role="timer"
        aria-label={formatCountdown(seconds)}
        data-testid="payment-countdown"
        className="mt-2 grid w-[172px] grid-cols-2 gap-1.5"
      >
        {(
          [
            [pad(minutes), t("order_countdown_minutes")],
            [pad(secs), t("order_countdown_seconds")],
          ] as const
        ).map(([value, label]) => (
          <div key={label} className="rounded-[14px] bg-surface pt-2.5 pb-2 text-center">
            <b className="block font-[family-name:var(--font-display)] text-2xl leading-[1.1] font-extrabold tabular-nums text-ink-900">
              {value}
            </b>
            <span className="text-[11px] text-ink-600">{label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
