import { cn } from "@/lib/utils";

type Mode = "login" | "register" | "otp";

// Login's headline is one unbroken line by design: the three pillars carry full
// white and the connectives recede, so the line reads as what the Hub is rather
// than as a sentence about it.
const headings: Record<Mode, React.ReactNode> = {
  login: (
    <>
      <span className="text-white/55">One Platform for </span>Learning
      <span className="text-white/55">, </span>Competition
      <span className="text-white/55"> &amp; </span>Growth
    </>
  ),
  register: "Mulai Perjalanan\nBelajarmu Bersama\nAbak Academy",
  otp: "Satu Langkah Lagi\nMenuju Akun\nAbak Academy",
};

const subs: Record<Mode, string> = {
  login: "Satu Platform untuk Belajar, Berkompetisi, dan Bertumbuh",
  register: "Daftar sekarang dan akses ribuan soal, kursus, dan ujian simulasi.",
  otp: "Verifikasi identitasmu untuk menjaga keamanan akun.",
};

function AbakLogo({ size = 24 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 120 120"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-label="abak academy"
    >
      <circle cx="44" cy="34" r="15" fill="currentColor" />
      <path d="M22 104 Q22 64 44 64 Q66 64 66 104 Z" fill="currentColor" />
      <path d="M62 104 Q62 78 80 78 Q98 78 98 104 Z" fill="#1E978A" />
      <path d="M80 44 L96 51 L80 58 L64 51 Z" fill="#D99A2B" />
      <circle cx="80" cy="62" r="11" fill="#1E978A" />
      <rect x="79" y="44" width="2.5" height="9" fill="#D99A2B" />
    </svg>
  );
}

function StudentIllustration() {
  return (
    <svg
      viewBox="0 0 360 300"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className="w-full max-w-[360px] [filter:drop-shadow(0_20px_40px_rgba(0,0,0,0.2))]"
    >
      <circle cx="180" cy="165" r="130" fill="rgba(255,255,255,0.05)" />
      <circle cx="180" cy="165" r="88" fill="rgba(255,255,255,0.05)" />
      <rect x="55" y="215" width="250" height="12" rx="6" fill="rgba(255,255,255,0.22)" />
      <rect x="95" y="227" width="11" height="38" rx="4" fill="rgba(255,255,255,0.13)" />
      <rect x="254" y="227" width="11" height="38" rx="4" fill="rgba(255,255,255,0.13)" />
      <rect x="95" y="200" width="170" height="15" rx="4" fill="rgba(255,255,255,0.28)" />
      <rect x="95" y="196" width="170" height="4" rx="2" fill="rgba(255,255,255,0.18)" />
      <rect x="103" y="116" width="154" height="82" rx="9" fill="rgba(30,33,60,0.55)" />
      <rect x="109" y="121" width="142" height="72" rx="6" fill="rgba(255,255,255,0.10)" />
      <rect x="109" y="121" width="142" height="72" rx="6" fill="url(#screenGlow)" opacity="0.6" />
      <rect x="118" y="129" width="50" height="4" rx="2" fill="rgba(255,255,255,0.45)" />
      <rect x="118" y="137" width="36" height="3" rx="1.5" fill="rgba(255,255,255,0.25)" />
      <rect x="178" y="140" width="9" height="22" rx="2" fill="rgba(110,124,238,0.8)" />
      <rect x="190" y="132" width="9" height="30" rx="2" fill="#9CA6F6" />
      <rect x="202" y="136" width="9" height="26" rx="2" fill="rgba(110,124,238,0.7)" />
      <rect x="214" y="128" width="9" height="34" rx="2" fill="#C4CAFB" />
      <rect x="226" y="133" width="9" height="29" rx="2" fill="rgba(110,124,238,0.6)" />
      <rect x="174" y="162" width="66" height="2" rx="1" fill="rgba(255,255,255,0.2)" />
      <circle cx="180" cy="74" r="27" fill="#FDDCB5" />
      <path d="M153 67 Q160 42 180 43 Q200 42 207 67 Q194 53 180 55 Q166 53 153 67Z" fill="#2D1B00" />
      <ellipse cx="153" cy="74" rx="5" ry="7" fill="#F5C99A" />
      <ellipse cx="207" cy="74" rx="5" ry="7" fill="#F5C99A" />
      <circle cx="172" cy="72" r="3.5" fill="#2D1B00" />
      <circle cx="188" cy="72" r="3.5" fill="#2D1B00" />
      <circle cx="173" cy="71" r="1" fill="white" />
      <circle cx="189" cy="71" r="1" fill="white" />
      <path d="M174 80 Q180 85 186 80" stroke="#C4846B" strokeWidth="1.8" strokeLinecap="round" fill="none" />
      <rect x="154" y="99" width="52" height="54" rx="14" fill="#3D4DDB" />
      <path d="M174 99 L180 110 L186 99Z" fill="rgba(255,255,255,0.35)" />
      <path d="M154 112 Q128 130 118 158 Q115 168 126 168 Q134 168 137 158 L154 130Z" fill="#3D4DDB" />
      <path d="M206 112 Q232 130 242 158 Q245 168 234 168 Q226 168 223 158 L206 130Z" fill="#3D4DDB" />
      <ellipse cx="122" cy="200" rx="15" ry="9" fill="#FDDCB5" />
      <ellipse cx="238" cy="200" rx="15" ry="9" fill="#FDDCB5" />
      <rect x="272" y="52" width="68" height="36" rx="10" fill="rgba(255,255,255,0.18)" />
      <text x="283" y="75" fontSize="18">🏆</text>
      <text x="306" y="68" fontSize="10" fill="rgba(255,255,255,0.9)" fontWeight="700">Top</text>
      <text x="305" y="80" fontSize="10" fill="rgba(255,255,255,0.6)">Rank</text>
      <rect x="20" y="88" width="68" height="36" rx="10" fill="rgba(255,255,255,0.18)" />
      <text x="30" y="111" fontSize="18">📚</text>
      <text x="53" y="104" fontSize="10" fill="rgba(255,255,255,0.9)" fontWeight="700">5+</text>
      <text x="50" y="116" fontSize="10" fill="rgba(255,255,255,0.6)">Mapel</text>
      <circle cx="76" cy="46" r="4" fill="rgba(255,255,255,0.5)" />
      <circle cx="91" cy="35" r="2.5" fill="rgba(255,255,255,0.35)" />
      <circle cx="275" cy="108" r="3" fill="rgba(255,255,255,0.5)" />
      <circle cx="292" cy="122" r="2" fill="rgba(255,255,255,0.3)" />
      <circle cx="44" cy="142" r="2.5" fill="rgba(255,255,255,0.4)" />
      <circle cx="315" cy="58" r="2" fill="rgba(255,255,255,0.35)" />
      <defs>
        <linearGradient id="screenGlow" x1="109" y1="121" x2="251" y2="193" gradientUnits="userSpaceOnUse">
          <stop stopColor="#6E7CEE" />
          <stop offset="1" stopColor="#3D4DDB" stopOpacity="0" />
        </linearGradient>
      </defs>
    </svg>
  );
}

function AuthStatCard({ emoji, value, label }: { emoji: string; value: string; label: string }) {
  return (
    <div className="flex flex-1 items-center gap-3 rounded-[14px] border border-white/20 bg-white/13 px-[18px] py-[13px] backdrop-blur-md">
      <div className="flex h-[38px] w-[38px] flex-shrink-0 items-center justify-center rounded-[9px] bg-white/15 text-[17px]">
        {emoji}
      </div>
      <div>
        <div className="font-serif text-[18px] font-bold leading-none text-white">{value}</div>
        <div className="mt-[3px] text-[11.5px] text-white/65">{label}</div>
      </div>
    </div>
  );
}

export function BrandPanel({ mode, className }: { mode: Mode; className?: string }) {
  return (
    <div
      className={cn(
        "relative hidden flex-col overflow-hidden bg-[linear-gradient(148deg,#1A1060_0%,#3D4DDB_28%,#7C4DDB_55%,#1E978A_82%,#17C9AA_100%)] px-[clamp(32px,3.4vw,52px)] pb-10 pt-11 lg:flex lg:basis-[52%]",
        className
      )}
    >
      <div className="pointer-events-none absolute -right-[90px] -top-[90px] h-[340px] w-[340px] rounded-full bg-white/4" />
      <div className="pointer-events-none -left-[70px] -bottom-[110px] absolute h-[300px] w-[300px] rounded-full bg-white/4" />

      <div className="z-[1] flex items-center gap-3">
        <div className="flex h-14 w-14 items-center justify-center rounded-[14px] bg-white/15 text-white">
          <AbakLogo size={34} />
        </div>
        <span className="font-serif text-[22px] font-extrabold tracking-[-0.01em] text-white">
          abak{" "}
          <span className="text-[15px] font-bold uppercase tracking-[0.08em] text-[#D99A2B]">
            academy
          </span>
        </span>
      </div>

      <div className="z-[1] mt-11">
        <h1
          className={cn(
            "font-serif font-bold text-white",
            mode === "login"
              ? // Fluid so the single line survives every desktop width.
                "whitespace-nowrap text-[clamp(17px,1.85vw,36px)] leading-[1.2] tracking-[-0.02em]"
              : "whitespace-pre-line text-[30px] leading-[1.28] tracking-[-0.01em]"
          )}
        >
          {headings[mode]}
        </h1>
        <p className="mt-[14px] text-[13.5px] leading-[1.65] text-white/68">{subs[mode]}</p>
      </div>

      <div className="z-[1] flex flex-1 items-center justify-center py-5">
        <StudentIllustration />
      </div>

      <div className="z-[1] flex gap-[10px]">
        <AuthStatCard emoji="🎓" value="20.000+" label="Siswa terdaftar" />
        <AuthStatCard emoji="🏫" value="200+" label="Mitra institusi" />
      </div>
    </div>
  );
}