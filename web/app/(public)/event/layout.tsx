import type { ReactNode } from "react";
import { pageMetadata } from "@/lib/site-metadata";

export const metadata = pageMetadata(
  "/event",
  "Event — Abak Academy",
  "Semua event Abak Academy di satu tempat: bidang, jenjang, alur babak, jadwal, dan biaya, lalu pilih edisi yang sedang dibuka.",
);

export default function EventLayout({ children }: { children: ReactNode }) {
  return children;
}
