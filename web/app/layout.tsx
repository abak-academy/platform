import type { Metadata } from "next";
import {
  Plus_Jakarta_Sans,
  Public_Sans,
  Source_Serif_4,
  Spline_Sans_Mono,
  Cinzel,
  Inter,
} from "next/font/google";
import "./globals.css";
import { Providers } from "./providers";
import { siteUrl } from "@/lib/site-metadata";
import { SESSION_FLAG_SCRIPT } from "@/lib/session-flag";

const plusJakarta = Plus_Jakarta_Sans({ subsets: ["latin"], weight: ["500", "600", "700", "800"], variable: "--font-display" });
const publicSans = Public_Sans({ subsets: ["latin"], variable: "--font-public-sans" });
const sourceSerif = Source_Serif_4({ subsets: ["latin"], variable: "--font-source-serif" });
const splineMono = Spline_Sans_Mono({ subsets: ["latin"], variable: "--font-spline-mono" });
// Exam card: Cinzel (title) + Inter (body)
const cinzel = Cinzel({ subsets: ["latin"], weight: ["600", "700"], variable: "--font-cinzel" });
const inter = Inter({ subsets: ["latin"], variable: "--font-inter" });

export const metadata: Metadata = {
  metadataBase: siteUrl(),
  title: "Abak Academy",
  description: "Tingkatkan Skills Bersama Abak Academy",
  other: { google: "notranslate" },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html
      lang="id"
      translate="no"
      suppressHydrationWarning
      className={`${plusJakarta.variable} ${publicSans.variable} ${sourceSerif.variable} ${splineMono.variable} ${cinzel.variable} ${inter.variable}`}
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: SESSION_FLAG_SCRIPT }} />
      </head>
      <body className="antialiased">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
