import { cn } from "@/lib/utils";
import { AbakLogo } from "@/components/brand/AbakLogo";

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

// Dissolves the photo's top and side edges into the panel. Two masks intersect
// rather than stack: a single gradient can only feather one axis.
const PHOTO_FEATHER = [
  "linear-gradient(to bottom, transparent 0%, rgba(0,0,0,.35) 26%, #000 58%, #000 93%, transparent 100%)",
  "linear-gradient(to right, transparent 0%, #000 12%, #000 88%, transparent 100%)",
].join(", ");

// `multiply` casts the panel's hue into the photo's shadows without rotating the
// photo's own hues — a duotone (mix-blend-mode: color) erases the merah-putih.
const PHOTO_TINT =
  "linear-gradient(148deg,#2A1E7A 0%,#4A5AE8 30%,#8A5CE8 58%,#2AA396 84%,#25D4B6 100%)";
const PHOTO_LIFT = "linear-gradient(148deg,#3D4DDB,#7C4DDB 55%,#17C9AA)";

function BrandPhoto() {
  return (
    <div
      aria-hidden
      className="pointer-events-none absolute inset-x-0 bottom-0 z-0 isolate h-[62%] overflow-hidden"
      style={{
        WebkitMaskImage: PHOTO_FEATHER,
        WebkitMaskComposite: "source-in",
        maskImage: PHOTO_FEATHER,
        maskComposite: "intersect",
      }}
    >
      <img
        src="/auth/hero-award.webp"
        alt=""
        className="h-full w-full object-cover [filter:saturate(.78)_contrast(1.09)_brightness(.84)]"
      />
      <div
        className="absolute inset-0 opacity-[.58] mix-blend-multiply"
        style={{ backgroundImage: PHOTO_TINT }}
      />
      <div
        className="absolute inset-0 opacity-20 mix-blend-screen"
        style={{ backgroundImage: PHOTO_LIFT }}
      />
    </div>
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

      <BrandPhoto />
      {/* keeps the stat cards legible wherever the photo happens to be bright */}
      <div className="pointer-events-none absolute inset-x-0 bottom-0 z-0 h-[23%] bg-[linear-gradient(to_top,rgba(14,12,58,.66),rgba(14,12,58,0))]" />

      <div className="z-[1] flex items-center gap-3">
        <div className="flex h-[72px] w-[72px] items-center justify-center rounded-[18px] bg-white/15 text-white">
          <AbakLogo size={44} />
        </div>
        <span className="font-serif text-[27px] font-extrabold tracking-[-0.01em] text-white">
          abak{" "}
          <span className="text-[18px] font-bold uppercase tracking-[0.08em] text-[#D99A2B]">
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

      <div className="z-[1] flex-1" />

      <div className="z-[1] flex gap-[10px]">
        <AuthStatCard emoji="🎓" value="20.000+" label="Siswa terdaftar" />
        <AuthStatCard emoji="🏫" value="200+" label="Mitra institusi" />
      </div>
    </div>
  );
}