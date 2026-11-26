import type { I18nKey } from "@/lib/i18n";

export const SITE_CONFIG = {
  whatsappNumber: "6282318910482",
  whatsappDisplay: "+62 823-1891-0482",
  address: "Komplek Nusa Loka, Sektor 14.6 BSD City, Jl. Biak Blok NA No.7",
  stats: [
    { value: "20.000+", labelKey: "land_stat_students" },
    { value: "200+", labelKey: "land_stat_partners" },
    { value: "10", labelKey: "land_stat_events" },
  ] as { value: string; labelKey: I18nKey }[],
  social: [
    { key: "instagram", label: "Instagram", url: null },
    { key: "tiktok", label: "TikTok", url: null },
    { key: "youtube", label: "YouTube", url: null },
    { key: "facebook", label: "Facebook", url: null },
  ] as { key: "instagram" | "tiktok" | "youtube" | "facebook"; label: string; url: string | null }[],
};

export function waHref(text: string): string {
  return `https://wa.me/${SITE_CONFIG.whatsappNumber}?text=${encodeURIComponent(text)}`;
}
