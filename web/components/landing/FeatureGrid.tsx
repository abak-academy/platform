"use client";

import { Award, ChartColumn, Monitor, Ticket } from "lucide-react";
import { useTranslation } from "@/lib/i18n";

const FEATURES = [
  { icon: Ticket, tone: "bg-ember-50 text-ember-700", title: "land_feat_1_title", text: "land_feat_1_text" },
  { icon: Monitor, tone: "bg-teal-50 text-teal-700", title: "land_feat_2_title", text: "land_feat_2_text" },
  { icon: ChartColumn, tone: "bg-sky-50 text-sky-700", title: "land_feat_3_title", text: "land_feat_3_text" },
  { icon: Award, tone: "bg-gold-50 text-gold-700", title: "land_feat_4_title", text: "land_feat_4_text" },
] as const;

export function FeatureGrid() {
  const { t } = useTranslation();
  return (
    <section data-reveal className="mx-auto w-full max-w-public px-6 py-20">
      <div className="mb-10 max-w-[60ch]">
        <span className="type-overline text-brand-ink">{t("land_feat_eyebrow")}</span>
        <h2 className="type-display mt-2 text-ink-900">{t("land_feat_title")}</h2>
      </div>
      <div data-reveal-grid className="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {FEATURES.map((f) => (
          <article key={f.title} data-tilt className="flex flex-col gap-3 rounded-[var(--radius-card)] border border-line bg-surface p-6 shadow-card">
            <span className={`flex size-11 items-center justify-center rounded-[var(--r-md)] ${f.tone}`}>
              <f.icon className="size-5" aria-hidden="true" />
            </span>
            <h3 className="type-h2 text-ink-900">{t(f.title)}</h3>
            <p className="type-body text-ink-600">{t(f.text)}</p>
          </article>
        ))}
      </div>
    </section>
  );
}
