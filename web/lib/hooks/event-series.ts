"use client";

import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { PublicEvent } from "@/lib/types";

export const eventSeriesKeys = {
  list: (lang: "id" | "en") => ["event-series", "list", lang] as const,
};

export function useEventSeries(lang: "id" | "en") {
  return useQuery({
    queryKey: eventSeriesKeys.list(lang),
    queryFn: async () => {
      const res = await apiFetch<{ data: PublicEvent[] }>(`/event-series?lang=${lang}`);
      return res.data ?? [];
    },
  });
}
