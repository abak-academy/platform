import type { Metadata } from "next";

export function siteUrl(): URL | undefined {
  const raw = process.env.NEXT_PUBLIC_SITE_URL;
  if (!raw) return undefined;
  try {
    return new URL(raw);
  } catch {
    return undefined;
  }
}

export function pageMetadata(path: string, title: string, description: string): Metadata {
  const canonical = siteUrl() ? path : undefined;
  return {
    title,
    description,
    ...(canonical && {
      alternates: { canonical },
      openGraph: { title, description, url: canonical, siteName: "Abak Academy", locale: "id_ID", type: "website" },
    }),
  };
}
