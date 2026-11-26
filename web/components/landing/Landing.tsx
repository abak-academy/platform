"use client";

import Link from "next/link";
import { useState } from "react";
import { CalendarX } from "lucide-react";
import { AboutLines } from "@/components/landing/AboutLines";
import { CtaBand } from "@/components/landing/CtaBand";
import { Faq } from "@/components/landing/Faq";
import { FeatureGrid } from "@/components/landing/FeatureGrid";
import { LandingHero } from "@/components/landing/LandingHero";
import { ProgramGrid } from "@/components/landing/ProgramGrid";
import { SchoolBand } from "@/components/landing/SchoolBand";
import { StorySlider } from "@/components/landing/StorySlider";
import { ProductCard } from "@/components/catalog/ProductCard";
import { EventSeriesPicker } from "@/components/events/EventSeriesPicker";
import { EventStoryCard } from "@/components/events/EventStoryCard";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { useEventSeries } from "@/lib/hooks/event-series";
import { useProducts } from "@/lib/hooks/products";
import { useTestimonials } from "@/lib/hooks/testimonials";
import { useTranslation } from "@/lib/i18n";
import { waHref } from "@/lib/site-config";
import type { Product } from "@/lib/types";

const SHOP_TYPES = ["book", "merchandise", "medal"];
const SECTION = "mx-auto w-full max-w-public px-6 pt-[60px] md:pt-[88px]";

function ProductSection({
  name,
  eyebrow,
  title,
  linkLabel,
  href,
  products,
}: {
  name: string;
  eyebrow: string;
  title: string;
  linkLabel: string;
  href: string;
  products: Product[];
}) {
  return (
    <section data-section={name} data-reveal className={SECTION}>
      <div className="mb-10 flex flex-wrap items-end justify-between gap-6">
        <div>
          <span className="type-overline text-brand-ink">{eyebrow}</span>
          <h2 className="type-display mt-2 text-ink-900">{title}</h2>
        </div>
        <Button asChild variant="outline">
          <Link href={href}>{linkLabel}</Link>
        </Button>
      </div>
      <div data-reveal-grid className="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {products.map((p) => (
          <div key={p.id} data-tilt className="flex">
            <ProductCard product={p} className="w-full" />
          </div>
        ))}
      </div>
    </section>
  );
}

export function Landing() {
  const { t, lang } = useTranslation();
  const events = useEventSeries(lang);
  const testimonials = useTestimonials(lang);
  const courses = useProducts("course");
  const products = useProducts();
  const [picked, setPicked] = useState<string | null>(null);

  const list = events.isError ? [] : (events.data ?? []);
  const selected = list.find((e) => e.slug === picked) ?? list[0];
  const courseList = (courses.isError ? [] : (courses.data ?? [])).filter((p) => !p.status || p.status === "published").slice(0, 4);
  const shopList = (products.isError ? [] : (products.data ?? [])).filter((p) => SHOP_TYPES.includes(p.type)).slice(0, 4);
  const stories = testimonials.isError ? [] : (testimonials.data ?? []);

  return (
    <>
      <div data-section="hero" className="contents">
        <LandingHero eventNames={list.map((e) => e.short_name ?? e.name)} />
      </div>
      <div data-section="programs" className="contents">
        <ProgramGrid />
      </div>
      <div data-section="about" className="contents">
        <AboutLines />
      </div>
      <section data-section="events" data-reveal className={SECTION}>
        <div className="mb-10 max-w-[60ch]">
          <span className="type-overline text-brand-ink">{t("land_event_eyebrow")}</span>
          <h2 className="type-display mt-2 text-ink-900">{t("land_event_title")}</h2>
          <p className="type-body mt-3 text-ink-600">{t("land_event_lede")}</p>
        </div>
        {selected ? (
          <div className="grid gap-6">
            <EventSeriesPicker events={list} selected={selected.slug} onSelect={setPicked} />
            <EventStoryCard event={selected} />
          </div>
        ) : (
          <EmptyState
            icon={CalendarX}
            title={t("land_event_empty_title")}
            text={t("land_event_empty_text")}
            action={
              <Button asChild>
                <a href={waHref(t("land_event_empty_wa"))} target="_blank" rel="noopener noreferrer">
                  {t("land_event_empty_cta")}
                </a>
              </Button>
            }
          />
        )}
      </section>
      <div data-section="features" className="contents">
        <FeatureGrid />
      </div>
      {courseList.length > 0 && (
        <ProductSection
          name="courses"
          eyebrow={t("land_courses_eyebrow")}
          title={t("land_courses_title")}
          linkLabel={t("land_courses_all")}
          href="/catalog?type=course"
          products={courseList}
        />
      )}
      {stories.length > 0 && (
        <div data-section="stories" className="contents">
          <StorySlider items={stories} />
        </div>
      )}
      {shopList.length > 0 && (
        <ProductSection
          name="shop"
          eyebrow={t("land_shop_eyebrow")}
          title={t("land_shop_title")}
          linkLabel={t("land_shop_link")}
          href="/catalog"
          products={shopList}
        />
      )}
      <div data-section="school" className="contents">
        <SchoolBand />
      </div>
      <div data-section="faq" className="contents">
        <Faq />
      </div>
      <div data-section="cta" className="contents">
        <CtaBand />
      </div>
    </>
  );
}
