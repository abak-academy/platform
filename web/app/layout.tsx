import type { Metadata } from "next";
import {
  Public_Sans,
  Source_Serif_4,
  Spline_Sans_Mono,
  Cinzel,
  Inter,
} from "next/font/google";
import "./globals.css";
import { Providers } from "./providers";

const publicSans = Public_Sans({ subsets: ["latin"], variable: "--font-public-sans" });
const sourceSerif = Source_Serif_4({ subsets: ["latin"], variable: "--font-source-serif" });
const splineMono = Spline_Sans_Mono({ subsets: ["latin"], variable: "--font-spline-mono" });
// Exam card: Cinzel (title) + Inter (body)
const cinzel = Cinzel({ subsets: ["latin"], weight: ["600", "700"], variable: "--font-cinzel" });
const inter = Inter({ subsets: ["latin"], variable: "--font-inter" });

export const metadata: Metadata = {
  title: "Abak Academy",
  description: "Tingkatkan Skills Bersama Abak Academy",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body
        className={`${publicSans.variable} ${sourceSerif.variable} ${splineMono.variable} ${cinzel.variable} ${inter.variable} antialiased`}
      >
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
