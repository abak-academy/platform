"use client";

import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { Province, City, District } from "@/lib/types";

export const regionsKeys = {
  all: ["regions"] as const,
  provinces: () => [...regionsKeys.all, "provinces"] as const,
  citiesByProvince: (provinceId: string) => [...regionsKeys.all, "cities", provinceId] as const,
  districtsByCity: (cityId: string) => [...regionsKeys.all, "districts", cityId] as const,
};

export function useProvinces() {
  return useQuery({
    queryKey: regionsKeys.provinces(),
    queryFn: () => apiFetch<Province[]>("/provinces"),
  });
}

export function useCitiesByProvince(provinceId?: string | null) {
  return useQuery({
    queryKey: regionsKeys.citiesByProvince(provinceId ?? ""),
    queryFn: () => apiFetch<City[]>(`/provinces/${encodeURIComponent(provinceId!)}/cities`),
    enabled: Boolean(provinceId),
  });
}

export function useDistrictsByCity(cityId?: string | null) {
  return useQuery({
    queryKey: regionsKeys.districtsByCity(cityId ?? ""),
    queryFn: () => apiFetch<District[]>(`/cities/${encodeURIComponent(cityId!)}/districts`),
    enabled: Boolean(cityId),
  });
}
