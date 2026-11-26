"use client";

import Link from "next/link";
import { ArrowRight, Check, MessageCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { PROGRAMS, type Program } from "@/lib/programs";
import { waHref } from "@/lib/site-config";

const TONE: Record<string, string> = {
  brand: "bg-brand-50 text-brand-ink",
  ember: "bg-ember-50 text-ember-700",
  teal: "bg-teal-50 text-teal-700",
  gold: "bg-gold-50 text-gold-700",
  sky: "bg-sky-50 text-sky-700",
  berry: "bg-berry-50 text-berry-700",
  navy: "bg-brand-50 text-brand-ink",
};

const WRAP = "mx-auto w-full max-w-public px-6";

function ProgramBlock({ program, index }: { program: Program; index: number }) {
  const { t } = useTranslation();
  const Icon = program.icon;
  const external = program.waKey !== null;
  return (
    <section id={program.id} data-reveal className={`${WRAP} scroll-mt-24 py-16 md:grid md:grid-cols-[240px_1fr] md:gap-12`}>
      <div className="mb-6 flex items-center gap-4 md:mb-0 md:flex-col md:items-start">
        <span className={`grid size-20 place-items-center rounded-3xl ${TONE[program.tone]}`}>
          <Icon className="size-10" aria-hidden="true" />
        </span>
        <span className="type-display-xl text-ink-400" aria-hidden="true">
          {String(index + 1).padStart(2, "0")}
        </span>
      </div>
      <div>
        <h2 className="type-display text-ink-900">{t(program.titleKey)}</h2>
        <p className="mt-3 max-w-[60ch] text-ink-600">{t(program.descKey)}</p>
        <dl data-reveal-grid className="mt-6 grid gap-4 sm:grid-cols-3">
          {(
            [
              ["prog_label_for", program.forKey],
              ["prog_label_format", program.formatKey],
              ["prog_label_schedule", program.scheduleKey],
            ] as const
          ).map(([label, value]) => (
            <div key={label} data-tilt className="rounded-2xl border border-line bg-surface p-4">
              <dt className="type-label-sm text-ink-500">{t(label)}</dt>
              <dd className="mt-1 text-ink-900">{t(value)}</dd>
            </div>
          ))}
        </dl>
        <h3 className="type-h3 mt-8 text-ink-900">{t("prog_label_gets")}</h3>
        <ul className="mt-3 grid gap-2">
          {program.gets.map((key) => (
            <li key={key} className="flex items-start gap-2 text-ink-700">
              <Check className="mt-1 size-4 shrink-0 text-teal-700" aria-hidden="true" />
              {t(key)}
            </li>
          ))}
        </ul>
        <div className="mt-8">
          <Button asChild size="lg">
            {external ? (
              <a
                data-cta
                href={waHref(t(program.waKey!))}
                target="_blank"
                rel="noopener noreferrer"
              >
                {t(program.ctaKey)}
                <MessageCircle aria-hidden="true" />
              </a>
            ) : (
              <Link data-cta href="/event">
                {t(program.ctaKey)}
                <ArrowRight aria-hidden="true" />
              </Link>
            )}
          </Button>
        </div>
      </div>
    </section>
  );
}

export default function ProgramPage() {
  const { t } = useTranslation();
  return (
    <>
      <section className="relative overflow-hidden bg-brand-50 pb-12 pt-32">
        <div aria-hidden="true" className="page-hero-fade absolute inset-0 bg-brand-100" />
        <div className={`${WRAP} relative`}>
          <span className="type-label text-brand-ink">{t("prog_page_eyebrow")}</span>
          <h1 className="type-display-xl mt-2 text-ink-900">{t("prog_page_title")}</h1>
          <p className="mt-4 max-w-[60ch] text-lg text-ink-600">{t("prog_page_lede")}</p>
          <nav aria-label={t("prog_jump_label")} className="mt-8 flex flex-wrap gap-2">
            {PROGRAMS.map((p) => {
              const Icon = p.icon;
              return (
                <a
                  key={p.id}
                  href={`#${p.id}`}
                  className="inline-flex items-center gap-2 rounded-[var(--radius-pill)] border border-line bg-surface px-4 py-2 type-label-sm text-ink-900 hover:border-ink-900"
                >
                  <Icon className="size-4" aria-hidden="true" />
                  {t(p.titleKey)}
                </a>
              );
            })}
          </nav>
        </div>
      </section>
      {PROGRAMS.map((p, i) => (
        <ProgramBlock key={p.id} program={p} index={i} />
      ))}
      <section data-reveal className={`${WRAP} pb-20 pt-8`}>
        <div className="rounded-3xl bg-surface-sunken p-10 text-center">
          <h2 className="type-display text-ink-900">{t("prog_help_title")}</h2>
          <p className="mx-auto mt-3 max-w-[50ch] text-ink-600">{t("prog_help_text")}</p>
          <Button asChild size="lg" className="mt-6">
            <a href={waHref(t("prog_help_wa"))} target="_blank" rel="noopener noreferrer">
              {t("prog_help_cta")}
              <MessageCircle aria-hidden="true" />
            </a>
          </Button>
        </div>
      </section>
    </>
  );
}
