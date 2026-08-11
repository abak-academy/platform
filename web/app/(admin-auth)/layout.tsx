"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/stores/auth";
import { AbakLogo } from "@/components/brand/AbakLogo";

// The photo is masked to a bloom rising off the bottom edge rather than laid
// flat: at 45% a full-bleed image puts photographed faces behind the footer,
// where no single text colour clears 4.5:1.
const BLOOM_MASK =
  "radial-gradient(ellipse 88% 80% at 50% 106%, #000 14%, transparent 74%)";

export default function AdminAuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const token = useAuthStore((s) => s.token);
  const router = useRouter();

  useEffect(() => {
    if (token) router.replace("/");
  }, [token, router]);

  if (token) {
    return (
      <div className="flex min-h-screen items-center justify-center text-ink-500">
        Memuat…
      </div>
    );
  }

  return (
    <div className="relative flex min-h-screen flex-col items-center justify-center overflow-hidden bg-[#F7F9FE] px-5 py-14">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 z-0 bg-[url('/auth/hero-award.webp')] bg-cover bg-center"
        style={{
          opacity: 0.45,
          filter: "grayscale(0.3)",
          WebkitMaskImage: BLOOM_MASK,
          maskImage: BLOOM_MASK,
        }}
      />

      {/* No opacity wrapper: at 70% the wordmark measured 4.49:1 and "ACADEMY"
          1.65:1 against this canvas. Recede via weight and size instead. */}
      <div className="absolute left-6 top-6 z-10 flex items-center gap-2">
        <AbakLogo size={26} />
        <span className="text-[13px] font-bold text-[#2B3674]">
          abak{" "}
          <span className="text-[11px] font-medium tracking-[0.12em] text-[#5B6690]">
            ACADEMY
          </span>
        </span>
      </div>

      <div className="relative z-10 w-full max-w-[376px]">{children}</div>

      {/* Ink, not a mid-grey: the bloom reaches this line at ~15% alpha, where
          #5b6690 falls to 3.95:1. */}
      <p className="relative z-10 mt-8 text-[11px] text-[#2B3674]">
        © {new Date().getFullYear()} Abak Academy
      </p>
    </div>
  );
}
