import { pageMetadata } from "@/lib/site-metadata";

export const metadata = pageMetadata(
  "/catalog",
  "Toko — Abak Academy",
  "Buku, modul, dan produk digital Abak Academy untuk persiapan olimpiade dan kompetisi."
);

export default function CatalogLayout({ children }: { children: React.ReactNode }) {
  return children;
}
