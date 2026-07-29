import localFont from "next/font/local";

const publicSans = localFont({
  src: [
    { path: "../../../../../../public/fonts/public_sans/PublicSans-Regular.ttf", weight: "400" },
    { path: "../../../../../../public/fonts/public_sans/PublicSans-SemiBold.ttf", weight: "700" },
  ],
  variable: "--font-certificate-public-sans",
});
const sourceSerif = localFont({
  src: [
    { path: "../../../../../../public/fonts/source_serif_4/SourceSerif4-SemiBold.ttf", weight: "400" },
    { path: "../../../../../../public/fonts/source_serif_4/SourceSerif4-Bold.ttf", weight: "700" },
  ],
  variable: "--font-certificate-source-serif",
});
const cinzel = localFont({
  src: "../../../../../../public/fonts/cinzel/Cinzel-Bold.ttf",
  variable: "--font-certificate-cinzel",
});
const playfairDisplay = localFont({
  src: "../../../../../../public/fonts/playfair_display/PlayfairDisplay-Bold.ttf",
  variable: "--font-certificate-playfair-display",
});
const cormorantGaramond = localFont({
  src: "../../../../../../public/fonts/cormorant_garamond/CormorantGaramond-SemiBold.ttf",
  variable: "--font-certificate-cormorant-garamond",
});
const greatVibes = localFont({
  src: "../../../../../../public/fonts/great_vibes/GreatVibes-Regular.ttf",
  variable: "--font-certificate-great-vibes",
});
const poppins = localFont({
  src: [
    { path: "../../../../../../public/fonts/poppins/Poppins-Regular.ttf", weight: "400" },
    { path: "../../../../../../public/fonts/poppins/Poppins-SemiBold.ttf", weight: "700" },
  ],
  variable: "--font-certificate-poppins",
});
const libreBaskerville = localFont({
  src: "../../../../../../public/fonts/libre_baskerville/LibreBaskerville-Variable.ttf",
  variable: "--font-certificate-libre-baskerville",
});
const allura = localFont({
  src: "../../../../../../public/fonts/allura/Allura-Regular.ttf",
  variable: "--font-certificate-allura",
});
const parisienne = localFont({
  src: "../../../../../../public/fonts/parisienne/Parisienne-Regular.ttf",
  variable: "--font-certificate-parisienne",
});

export default function ExamPackageLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className={`${publicSans.variable} ${sourceSerif.variable} ${cinzel.variable} ${playfairDisplay.variable} ${cormorantGaramond.variable} ${greatVibes.variable} ${poppins.variable} ${libreBaskerville.variable} ${allura.variable} ${parisienne.variable}`}>
      {children}
    </div>
  );
}
