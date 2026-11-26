import { BookOpen, Globe, House, PenLine, Target, Trophy, type LucideIcon } from "lucide-react";
import type { I18nKey } from "@/lib/i18n";
import type { EventTone } from "@/lib/types";

export type Program = {
  id: "kompetisi" | "olimpiade" | "bimbel" | "karya-tulis" | "homeschooling" | "study-tour";
  icon: LucideIcon;
  tone: EventTone;
  titleKey: I18nKey;
  shortKey: I18nKey;
  descKey: I18nKey;
  forKey: I18nKey;
  formatKey: I18nKey;
  scheduleKey: I18nKey;
  gets: I18nKey[];
  ctaKey: I18nKey;
  waKey: I18nKey | null;
};

export const PROGRAMS: Program[] = [
  {
    id: "kompetisi",
    icon: Trophy,
    tone: "ember",
    titleKey: "prog_kompetisi_title",
    shortKey: "prog_kompetisi_short",
    descKey: "prog_kompetisi_desc",
    forKey: "prog_kompetisi_for",
    formatKey: "prog_kompetisi_format",
    scheduleKey: "prog_kompetisi_schedule",
    gets: ["prog_kompetisi_get_1", "prog_kompetisi_get_2", "prog_kompetisi_get_3", "prog_kompetisi_get_4"],
    ctaKey: "prog_cta_event",
    waKey: null,
  },
  {
    id: "olimpiade",
    icon: Target,
    tone: "brand",
    titleKey: "prog_olimpiade_title",
    shortKey: "prog_olimpiade_short",
    descKey: "prog_olimpiade_desc",
    forKey: "prog_olimpiade_for",
    formatKey: "prog_olimpiade_format",
    scheduleKey: "prog_olimpiade_schedule",
    gets: ["prog_olimpiade_get_1", "prog_olimpiade_get_2", "prog_olimpiade_get_3", "prog_olimpiade_get_4"],
    ctaKey: "prog_cta_wa",
    waKey: "prog_olimpiade_wa",
  },
  {
    id: "bimbel",
    icon: BookOpen,
    tone: "teal",
    titleKey: "prog_bimbel_title",
    shortKey: "prog_bimbel_short",
    descKey: "prog_bimbel_desc",
    forKey: "prog_bimbel_for",
    formatKey: "prog_bimbel_format",
    scheduleKey: "prog_bimbel_schedule",
    gets: ["prog_bimbel_get_1", "prog_bimbel_get_2", "prog_bimbel_get_3"],
    ctaKey: "prog_cta_wa",
    waKey: "prog_bimbel_wa",
  },
  {
    id: "karya-tulis",
    icon: PenLine,
    tone: "berry",
    titleKey: "prog_karya_tulis_title",
    shortKey: "prog_karya_tulis_short",
    descKey: "prog_karya_tulis_desc",
    forKey: "prog_karya_tulis_for",
    formatKey: "prog_karya_tulis_format",
    scheduleKey: "prog_karya_tulis_schedule",
    gets: ["prog_karya_tulis_get_1", "prog_karya_tulis_get_2", "prog_karya_tulis_get_3"],
    ctaKey: "prog_cta_wa",
    waKey: "prog_karya_tulis_wa",
  },
  {
    id: "homeschooling",
    icon: House,
    tone: "gold",
    titleKey: "prog_homeschooling_title",
    shortKey: "prog_homeschooling_short",
    descKey: "prog_homeschooling_desc",
    forKey: "prog_homeschooling_for",
    formatKey: "prog_homeschooling_format",
    scheduleKey: "prog_homeschooling_schedule",
    gets: ["prog_homeschooling_get_1", "prog_homeschooling_get_2", "prog_homeschooling_get_3"],
    ctaKey: "prog_cta_wa",
    waKey: "prog_homeschooling_wa",
  },
  {
    id: "study-tour",
    icon: Globe,
    tone: "sky",
    titleKey: "prog_study_tour_title",
    shortKey: "prog_study_tour_short",
    descKey: "prog_study_tour_desc",
    forKey: "prog_study_tour_for",
    formatKey: "prog_study_tour_format",
    scheduleKey: "prog_study_tour_schedule",
    gets: ["prog_study_tour_get_1", "prog_study_tour_get_2", "prog_study_tour_get_3"],
    ctaKey: "prog_cta_wa",
    waKey: "prog_study_tour_wa",
  },
];
