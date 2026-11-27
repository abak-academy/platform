import { Calculator, Code, FlaskConical, Languages, Trophy, type LucideIcon } from "lucide-react";
import { fileUrl } from "@/lib/api";
import type { EventIllustration, EventTone } from "@/lib/types";

export const EVENT_TONES: { value: EventTone; label: string }[] = [
  { value: "brand", label: "Biru" },
  { value: "ember", label: "Terracotta" },
  { value: "teal", label: "Teal" },
  { value: "gold", label: "Emas" },
  { value: "sky", label: "Langit" },
  { value: "berry", label: "Berry" },
  { value: "navy", label: "Navy" },
];

export const EVENT_ILLUSTRATIONS: { value: EventIllustration; label: string; icon: LucideIcon }[] = [
  { value: "math", label: "Matematika", icon: Calculator },
  { value: "science", label: "Sains", icon: FlaskConical },
  { value: "language", label: "Bahasa", icon: Languages },
  { value: "coding", label: "Coding / riset", icon: Code },
  { value: "trophy", label: "Piala", icon: Trophy },
];

export const EVENT_TONE_CLASS: Record<EventTone, string> = {
  brand: "bg-brand-50 text-brand-800",
  ember: "bg-ember-50 text-ember-700",
  teal: "bg-teal-50 text-teal-700",
  gold: "bg-gold-50 text-gold-700",
  sky: "bg-sky-50 text-sky-700",
  berry: "bg-berry-50 text-berry-700",
  navy: "bg-brand-100 text-brand-800",
};

const grad = (b: string, a: string) => `radial-gradient(120% 110% at 100% 0%, ${b}, ${a})`;

export const EVENT_ART_BG: Record<EventTone, string> = {
  brand: grad("#6e7cee", "#3d4ddb"),
  ember: grad("#f59a6b", "#e2683c"),
  teal: grad("#4cc3b2", "#1e978a"),
  gold: grad("#f5c868", "#e3a131"),
  sky: grad("#6fa2f2", "#2f6fde"),
  berry: grad("#ec7ea4", "#c23b6b"),
  navy: grad("#3c5390", "#22315b"),
};

export function EventMark({
  logoKey,
  logoUrl,
  tone,
  illustration,
  className = "size-16",
}: {
  logoKey?: string | null;
  logoUrl?: string | null;
  tone: EventTone;
  illustration: EventIllustration;
  className?: string;
}) {
  const src = logoUrl || fileUrl(logoKey);
  const Icon = EVENT_ILLUSTRATIONS.find((i) => i.value === illustration)?.icon ?? Trophy;
  return (
    <span className={`inline-flex shrink-0 items-center justify-center overflow-hidden rounded-xl ${EVENT_TONE_CLASS[tone] ?? EVENT_TONE_CLASS.brand} ${className}`}>
      {src ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={src} alt="" className="size-full object-contain p-1" />
      ) : (
        <Icon className="size-1/2" aria-hidden />
      )}
    </span>
  );
}
