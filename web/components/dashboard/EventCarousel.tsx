"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { fileUrl } from "@/lib/api";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { levelText, type HeroSlide } from "@/lib/student-home";
import { Button } from "@/components/ui/button";
import type { EventIllustration } from "@/lib/types";
import { Spot } from "./Spot";
import { cn } from "@/lib/utils";

const INTERVAL_MS = 6000;

export function EventCarousel({ slides }: { slides: HeroSlide[] }) {
  const { t } = useTranslation();
  const [index, setIndex] = useState(0);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const paused = hovered || focused;
  const count = slides.length;
  const active = count ? index % count : 0;

  useEffect(() => {
    if (count < 2 || paused) return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const id = setInterval(() => setIndex((i) => (i + 1) % count), INTERVAL_MS);
    return () => clearInterval(id);
  }, [count, paused]);

  const mine = (
    <Button asChild variant="line-light" className="border-white/50 hover:bg-white/12">
      <Link href="/exam">{t("stu_home_carousel_cta_mine")}</Link>
    </Button>
  );

  const items = count
    ? slides.map((s, i) => {
        const level = levelText(s.stages, s.gradeMin, s.gradeMax, t("grade"));
        const price = s.price > 0 ? formatRupiah(s.price) : t("stu_home_carousel_free");
        const img = fileUrl(s.imageUrl);
        const open = s.phase === "open";
        return (
          <div key={s.productId} data-slide className={slideClass(i === active, i < active)} {...hide(i !== active)}>
            <div className="min-w-0">
              <span className={EYEBROW}>{s.eventName}</span>
              <h2 className={TITLE}>{t(open ? "stu_home_carousel_open" : "stu_home_carousel_live")}</h2>
              <p className={BODY}>{[s.title, level, price].filter(Boolean).join(" · ")}</p>
              <div className="flex flex-wrap items-center gap-2">
                <Button asChild variant="light" className="px-[18px] text-[#14202b]">
                  <Link href={`/catalog/${s.productId}`}>
                    {t(open ? "stu_home_carousel_cta_open" : "stu_home_carousel_cta_live")}
                    <ArrowRight />
                  </Link>
                </Button>
                {mine}
              </div>
            </div>
            {img ? (
              <div className="max-narrow:hidden">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={img} alt="" className="max-h-40 w-full rounded-xl object-cover" />
              </div>
            ) : (
              <Art illustration={s.illustration} />
            )}
          </div>
        );
      })
    : [
        <div key="fallback" data-slide className={slideClass(true, false)}>
          <div className="min-w-0">
            <h2 className={cn(TITLE, "mb-[18px]")}>{t("stu_home_carousel_fallback")}</h2>
            {mine}
          </div>
          <Art />
        </div>,
      ];

  return (
    <section
      role="region"
      aria-roledescription="carousel"
      aria-label={t("stu_home_carousel_label")}
      className="dash-hero-bg relative flex flex-col justify-center overflow-hidden rounded-[30px] px-8 py-[30px] text-white max-narrow:p-6"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onFocus={() => setFocused(true)}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setFocused(false);
      }}
    >
      <div className="grid">{items}</div>
      {count > 1 && (
        <div className="mt-3.5 -ml-1 flex items-center">
          {slides.map((s, i) => (
            <button
              key={s.productId}
              type="button"
              aria-label={t("stu_home_carousel_dot")
                .replace("{i}", String(i + 1))
                .replace("{n}", String(count))
                .replace("{name}", s.eventName)}
              aria-current={i === active ? "true" : undefined}
              onClick={() => setIndex(i)}
              className="flex h-6 items-center px-1 outline-none focus-visible:rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white"
            >
              <span
                className={cn(
                  "block h-2 rounded-[4px] bg-white/45 transition-all duration-350 motion-reduce:transition-none",
                  i === active ? "w-[22px] bg-white" : "w-2",
                )}
              />
            </button>
          ))}
        </div>
      )}
    </section>
  );
}

const EYEBROW = "mb-2.5 inline-block font-mono text-[11.5px] leading-[1.4] font-semibold tracking-[0.14em] text-white/85 uppercase";
const TITLE =
  "mb-2.5 font-[family-name:var(--font-display)] text-[clamp(26px,3vw,36px)] leading-[1.1] font-extrabold tracking-[-0.035em] text-white";
const BODY = "mb-[18px] max-w-[520px] text-[15px] text-white/90";

function Art({ illustration }: { illustration?: EventIllustration }) {
  return (
    <div className="dh-art relative max-narrow:hidden">
      <Spot illustration={illustration} />
      <div aria-hidden="true" className="pointer-events-none absolute inset-0 overflow-hidden">
        <span className="dh-floater" style={{ left: "6%", top: "12%", "--r": "-12deg" } as React.CSSProperties}>∑</span>
        <span className="dh-floater" style={{ left: "78%", top: "70%", "--r": "10deg" } as React.CSSProperties}>π</span>
      </div>
    </div>
  );
}

const hide = (hidden: boolean) => (hidden ? { inert: true, "aria-hidden": true as const } : {});

const slideClass = (on: boolean, before: boolean) =>
  cn(
    "grid [grid-area:1/1] grid-cols-[1fr_170px] items-center gap-5 max-narrow:grid-cols-1",
    "transition-[opacity,transform,visibility] duration-500 motion-reduce:transition-none motion-reduce:transform-none",
    on ? "visible opacity-100" : cn("invisible opacity-0", before ? "-translate-x-8" : "translate-x-8"),
  );
