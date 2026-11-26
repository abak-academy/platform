import { pageMetadata } from "@/lib/site-metadata";

export const metadata = pageMetadata(
  "/program",
  "Program — Abak Academy",
  "Kompetisi, pelatihan olimpiade, bimbel, karya tulis, homeschooling, dan study tour. Lihat untuk siapa tiap program dan apa yang kamu dapat.",
);

export default function ProgramLayout({ children }: { children: React.ReactNode }) {
  return children;
}
