"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { fileUrl } from "@/lib/api";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { levelText, type HeroSlide } from "@/lib/student-home";
import { EventMark } from "@/components/admin/EventMark";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const INTERVAL_MS = 6000;

export function EventCarousel({ slides }: { slides: HeroSlide[] }) {
  const { t } = useTranslation();
  const [index, setIndex] = useState(0);
  const [paused, setPaused] = useState(false);
  const count = slides.length;
  const active = count ? index % count : 0;

  useEffect(() => {
    if (count < 2 || paused) return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const id = setInterval(() => setIndex((i) => (i + 1) % count), INTERVAL_MS);
    return () => clearInterval(id);
  }, [count, paused]);

  const mine = (
    <Button asChild variant="line-light">
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
              <span className="type-overline text-white/80">{s.eventName}</span>
              <h2 className="mt-2 mb-2 type-h2 text-white">
                {t(open ? "stu_home_carousel_open" : "stu_home_carousel_live")}
              </h2>
              <p className="mb-5 text-white/90">{[s.title, level, price].filter(Boolean).join(" · ")}</p>
              <div className="flex flex-wrap gap-3">
                <Button asChild variant="default">
                  <Link href={`/catalog/${s.productId}`}>
                    {t(open ? "stu_home_carousel_cta_open" : "stu_home_carousel_cta_live")}
                    <ArrowRight />
                  </Link>
                </Button>
                {mine}
              </div>
            </div>
            <div className="hidden justify-self-center sm:block">
              {img ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={img} alt="" className="max-h-40 w-full rounded-xl object-cover" />
              ) : (
                <EventMark tone={s.tone as never} illustration={s.illustration as never} className="size-32" />
              )}
            </div>
          </div>
        );
      })
    : [
        <div key="fallback" data-slide className={slideClass(true, false)}>
          <div>
            <h2 className="mb-5 type-h2 text-white">{t("stu_home_carousel_fallback")}</h2>
            {mine}
          </div>
        </div>,
      ];

  return (
    <section
      role="region"
      aria-roledescription="carousel"
      aria-label={t("stu_home_carousel_label")}
      className="rounded-2xl dash-hero-bg p-6 text-white sm:p-8"
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
    >
      <div className="grid">{items}</div>
      {count > 1 && (
        <div className="mt-3 flex items-center">
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
              className="group h-6 px-1 outline-none focus-visible:outline-2 focus-visible:outline-white"
            >
              <span
                className={cn(
                  "block h-2 rounded-full bg-white/45 transition-all motion-reduce:transition-none",
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

const hide = (hidden: boolean) => (hidden ? { inert: true, "aria-hidden": true as const } : {});

const slideClass = (on: boolean, before: boolean) =>
  cn(
    "grid [grid-area:1/1] items-center gap-5 sm:grid-cols-[1fr_160px]",
    "transition-[opacity,transform,visibility] duration-500 motion-reduce:transition-none motion-reduce:transform-none",
    on ? "visible opacity-100" : cn("invisible opacity-0", before ? "-translate-x-8" : "translate-x-8"),
  );
