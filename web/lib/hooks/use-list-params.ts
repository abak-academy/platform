"use client";

import { useCallback, useMemo } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

export const PAGE_SIZES = [10, 20, 50] as const;

export interface ListParams {
  q: string;
  status: string;
  type: string;
  from: string;
  to: string;
  page: number;
  per_page: number;
}

const TEXT_KEYS = ["q", "status", "type", "from", "to"] as const;

// The list state lives in the URL, so a refresh or the back button keeps the
// search, filters and page instead of resetting them. Any change other than the
// page itself returns to page 1.
export function useListParams(defaultPerPage: number = 20) {
  const search = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const current = search.toString();

  const params = useMemo<ListParams>(() => {
    const sp = new URLSearchParams(current);
    const perPage = Number(sp.get("per_page"));
    return {
      q: sp.get("q") ?? "",
      status: sp.get("status") ?? "",
      type: sp.get("type") ?? "",
      from: sp.get("from") ?? "",
      to: sp.get("to") ?? "",
      page: Math.max(1, Math.floor(Number(sp.get("page"))) || 1),
      per_page: (PAGE_SIZES as readonly number[]).includes(perPage) ? perPage : defaultPerPage,
    };
  }, [current, defaultPerPage]);

  const write = useCallback(
    (next: URLSearchParams) => {
      const qs = next.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [router, pathname],
  );

  const update = useCallback(
    (patch: Partial<ListParams>) => {
      const next = new URLSearchParams(current);
      for (const key of TEXT_KEYS) {
        if (key in patch) {
          const value = (patch[key] ?? "").trim();
          if (value) next.set(key, value);
          else next.delete(key);
        }
      }
      if (patch.per_page !== undefined) {
        if (patch.per_page === defaultPerPage) next.delete("per_page");
        else next.set("per_page", String(patch.per_page));
      }
      if (patch.page !== undefined && patch.page > 1) next.set("page", String(patch.page));
      else next.delete("page");
      write(next);
    },
    [current, defaultPerPage, write],
  );

  // Clears the filters but keeps unrelated params (such as a tab) and the page size.
  const reset = useCallback(() => {
    const next = new URLSearchParams(current);
    for (const key of [...TEXT_KEYS, "page"]) next.delete(key);
    write(next);
  }, [current, write]);

  const hasFilters = Boolean(params.q || params.status || params.type || params.from || params.to);

  return { params, update, reset, hasFilters };
}
