"use client";

import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";

export type PublicTestimonial = {
  id: string;
  name: string;
  role_label: string;
  photo_url: string;
  quote: string;
  achievements: string[];
};

export const testimonialKeys = {
  list: (lang: "id" | "en") => ["testimonials", "list", lang] as const,
};

export function useTestimonials(lang: "id" | "en") {
  return useQuery({
    queryKey: testimonialKeys.list(lang),
    queryFn: async () => {
      const res = await apiFetch<{ data: PublicTestimonial[] }>(`/testimonials?lang=${lang}`);
      return res.data ?? [];
    },
  });
}
