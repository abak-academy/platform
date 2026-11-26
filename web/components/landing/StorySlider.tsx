"use client";

import { useState } from "react";
import { ChevronLeft, ChevronRight, Trophy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { fileUrl } from "@/lib/api";
import { useTranslation } from "@/lib/i18n";
import type { PublicTestimonial } from "@/lib/hooks/testimonials";

const pad = (n: number) => String(n).padStart(2, "0");

export function StorySlider({ items }: { items: PublicTestimonial[] }) {
  const { t } = useTranslation();
  const [index, setIndex] = useState(0);
  if (items.length === 0) return null;

  const current = items[index % items.length];
  const step = (d: number) => setIndex((i) => (i + d + items.length) % items.length);
  const photo = fileUrl(current.photo_url);

  return (
    <section data-reveal className="mx-auto w-full max-w-public px-6 py-20">
      <div className="grid gap-8 overflow-hidden rounded-[var(--radius-panel)] border border-line bg-surface p-4 shadow-card md:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] md:p-6">
        <div className="aspect-[4/5] overflow-hidden rounded-[var(--radius-card)] bg-surface-sunken">
          {photo && (
            <img
              key={current.id}
              src={photo}
              alt={t("land_stories_photo_alt").replace("{name}", current.name)}
              loading="lazy"
              className="h-full w-full object-cover"
            />
          )}
        </div>
        <div className="flex flex-col justify-center gap-5 py-2 md:pr-6">
          <div className="flex items-baseline justify-between gap-4">
            <h2 className="type-overline text-brand-ink">{t("land_stories_eyebrow")}</h2>
            <span className="type-overline text-ink-600">{`${pad(index + 1)}/${pad(items.length)}`}</span>
          </div>
          <blockquote className="type-h1 text-ink-900">“{current.quote}”</blockquote>
          <div>
            <b className="type-label block text-ink-900">{current.name}</b>
            <span className="type-body-sm text-ink-600">{current.role_label}</span>
          </div>
          <ul className="flex flex-wrap gap-2">
            {current.achievements.slice(0, 3).map((a, i) => (
              <li key={`${i}-${a}`} className="type-label-xs inline-flex items-center gap-1.5 rounded-[var(--radius-pill)] bg-gold-50 px-3 py-1.5 text-gold-700">
                <Trophy className="size-3.5" aria-hidden="true" />
                {a}
              </li>
            ))}
          </ul>
          <div className="flex gap-2">
            <Button variant="outline" size="icon" aria-label={t("land_stories_prev")} onClick={() => step(-1)}>
              <ChevronLeft />
            </Button>
            <Button variant="outline" size="icon" aria-label={t("land_stories_next")} onClick={() => step(1)}>
              <ChevronRight />
            </Button>
          </div>
        </div>
      </div>
    </section>
  );
}
