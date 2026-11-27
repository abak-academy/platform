import type { EventIllustration } from "@/lib/types";

// Fixed illustration colours, ported from the prototype's SPOT map.
const FONT = { fontFamily: "var(--font-display), sans-serif" };

const SPOTS: Record<EventIllustration, React.ReactNode> = {
  math: (
    <>
      <rect x="22" y="18" width="56" height="78" rx="12" fill="#fff" opacity=".95" />
      <rect x="30" y="27" width="40" height="16" rx="5" fill="#22315b" />
      <text x="66" y="39" textAnchor="end" fontWeight="800" fontSize="11" fill="#fff" style={FONT}>π=3,14</text>
      {[0, 1, 2].flatMap((r) =>
        [0, 1, 2].map((c) => (
          <rect key={`${r}${c}`} x={30 + c * 14} y={50 + r * 14} width="11" height="10" rx="3" fill={r === 2 && c === 2 ? "#c24e26" : "#f3e6dc"} />
        )),
      )}
      <circle cx="90" cy="30" r="14" fill="#d99a2b" />
      <text x="90" y="36" textAnchor="middle" fontWeight="800" fontSize="17" fill="#fff" style={FONT}>∑</text>
      <path d="M84 78 l18 18 M84 96 l18 -18" stroke="#1e978a" strokeWidth="6" strokeLinecap="round" />
    </>
  ),
  science: (
    <>
      <path d="M48 18 h24 M54 18 v26 L32 88 c-3 7 2 12 9 12 h38 c7 0 12-5 9-12 L66 44 V18" fill="#fff" stroke="#22315b" strokeWidth="5" strokeLinejoin="round" />
      <path d="M40 74 h40 l8 14 c3 7-2 12-9 12 H41 c-7 0-12-5-9-12 Z" fill="#1e978a" />
      <circle cx="52" cy="84" r="4" fill="#fff" />
      <circle cx="66" cy="90" r="3" fill="#fff" />
      <circle cx="92" cy="34" r="16" fill="none" stroke="#d99a2b" strokeWidth="4" />
      <ellipse cx="92" cy="34" rx="16" ry="6" fill="none" stroke="#c24e26" strokeWidth="3" transform="rotate(35 92 34)" />
      <circle cx="92" cy="34" r="4" fill="#c24e26" />
    </>
  ),
  language: (
    <>
      <path d="M16 26 h58 a10 10 0 0 1 10 10 v26 a10 10 0 0 1 -10 10 H40 l-14 12 v-12 h-10 a10 10 0 0 1 -10 -10 V36 a10 10 0 0 1 10 -10 Z" fill="#fff" />
      <text x="45" y="56" textAnchor="middle" fontWeight="800" fontSize="22" fill="#22315b" style={FONT}>Aa</text>
      <path d="M58 58 h44 a9 9 0 0 1 9 9 v20 a9 9 0 0 1 -9 9 h-6 v10 l-12 -10 H58 a9 9 0 0 1 -9 -9 V67 a9 9 0 0 1 9 -9 Z" fill="#c24e26" />
      <path d="M62 74 h34 M62 84 h22" stroke="#fff" strokeWidth="5" strokeLinecap="round" />
    </>
  ),
  coding: (
    <>
      <rect x="12" y="22" width="96" height="66" rx="12" fill="#22315b" />
      <rect x="12" y="22" width="96" height="14" rx="7" fill="#11212d" />
      <circle cx="24" cy="29" r="3" fill="#c24e26" />
      <circle cx="34" cy="29" r="3" fill="#d99a2b" />
      <circle cx="44" cy="29" r="3" fill="#1e978a" />
      <path d="M40 52 l-12 10 l12 10 M80 52 l12 10 l-12 10" fill="none" stroke="#7fd1c4" strokeWidth="5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M66 48 l-12 28" stroke="#f3cd6b" strokeWidth="5" strokeLinecap="round" />
      <rect x="44" y="92" width="32" height="8" rx="4" fill="#fff" />
    </>
  ),
  trophy: (
    <>
      <path d="M38 20 h44 v26 c0 14 -10 24 -22 24 s-22 -10 -22 -24 Z" fill="#d99a2b" />
      <path d="M38 28 h-12 c0 14 6 20 14 20 M82 28 h12 c0 14 -6 20 -14 20" fill="none" stroke="#d99a2b" strokeWidth="6" />
      <rect x="54" y="68" width="12" height="16" fill="#c98a25" />
      <rect x="40" y="84" width="40" height="14" rx="5" fill="#22315b" />
      <path d="M60 30 l4 8 9 1 -7 6 2 9 -8 -5 -8 5 2 -9 -7 -6 9 -1 Z" fill="#fff" />
      <circle cx="96" cy="20" r="4" fill="#c24e26" />
      <circle cx="22" cy="60" r="3" fill="#1e978a" />
      <path d="M100 50 l3 6 6 1 -5 4 1 6 -5 -3 -5 3 1 -6 -5 -4 6 -1 Z" fill="#f3cd6b" />
    </>
  ),
};

export function Spot({ illustration, className }: { illustration?: EventIllustration; className?: string }) {
  const key = illustration && illustration in SPOTS ? illustration : "trophy";
  return (
    <svg viewBox="0 0 120 120" aria-hidden="true" className={className} data-spot={key}>
      {SPOTS[key]}
    </svg>
  );
}
