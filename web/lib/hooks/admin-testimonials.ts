"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { AdminTestimonial, AdminTestimonialInput } from "@/lib/types";

export const adminTestimonialsKeys = {
  all: ["admin", "testimonials"] as const,
  list: () => [...adminTestimonialsKeys.all, "list"] as const,
};

export const TESTIMONIAL_MISSING_MESSAGES: Record<string, string> = {
  has_consent: "Centang persetujuan tertulis sebelum tampil di landing",
  consent_note: "Isi catatan lokasi persetujuan sebelum tampil di landing",
  "content.id.quote": "Isi kutipan Indonesia sebelum tampil di landing",
  "content.en.quote": "Isi kutipan English sebelum tampil di landing",
  photo_key: "Unggah foto peserta sebelum tampil di landing",
};

// testimonialMissing lists the parts that keep a testimonial off the landing, in the server's order.
export function testimonialMissing(t: Pick<AdminTestimonial, "has_consent" | "consent_note" | "content" | "photo_key">): string[] {
  const missing: string[] = [];
  if (!t.has_consent) missing.push("has_consent");
  if (!t.consent_note?.trim()) missing.push("consent_note");
  if (!t.content.id?.quote?.trim()) missing.push("content.id.quote");
  if (!t.content.en?.quote?.trim()) missing.push("content.en.quote");
  if (!t.photo_key) missing.push("photo_key");
  return missing;
}

export function testimonialBlockedMessage(missing: string[]): string {
  return missing.map((m) => TESTIMONIAL_MISSING_MESSAGES[m] ?? m).join("; ");
}

export function useAdminTestimonials() {
  return useQuery({
    queryKey: adminTestimonialsKeys.list(),
    queryFn: async () => {
      const res = await authFetch<{ data: AdminTestimonial[] }>("/admin/testimonials");
      return res.data ?? [];
    },
  });
}

export function useCreateTestimonial() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminTestimonialInput) =>
      authFetch<AdminTestimonial>("/admin/testimonials", { method: "POST", body: JSON.stringify(input) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: adminTestimonialsKeys.list() }),
  });
}

export function useUpdateTestimonial() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: AdminTestimonialInput }) =>
      authFetch<AdminTestimonial>(`/admin/testimonials/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: adminTestimonialsKeys.list() }),
  });
}

export function useDeleteTestimonial() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<void>(`/admin/testimonials/${encodeURIComponent(id)}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: adminTestimonialsKeys.list() }),
  });
}
