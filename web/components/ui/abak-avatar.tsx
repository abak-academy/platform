import { cn } from "@/lib/utils";

const AV_F = [0, 1, 5, 7, 10, 15, 19, 20, 26, 27, 30, 31, 33];
const AV_M = [4, 8, 9, 11, 14, 17, 18, 22, 23, 25, 28, 29];
const AV_TF = [13, 32, 6, 24];
const AV_TM = [2, 12, 16, 21, 34, 3];
const FEMALE =
  /^(nadia|dewi|putri|sekar|intan|citra|bu|ibu|ms|ratna|sinta|laras|hesti|anisa|siti|rani|ayu|dina|nabila|aisyah|zahra|salsa)\b/i;
const TEACHER = /^(bu|ibu|pak|ms|mr)\b/i;

function hashStr(s: string): number {
  let h = 2166136261;
  for (const c of String(s)) {
    h ^= c.charCodeAt(0);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

export function avatarIndex(name: string): number {
  const n = String(name ?? "").replace(/[^\p{L}\s.]/gu, "").trim();
  const f = FEMALE.test(n);
  const t = TEACHER.test(n);
  const pool = t ? (f ? AV_TF : AV_TM) : f ? AV_F : AV_M;
  return pool[hashStr(n) % pool.length];
}

export function avatarPosition(i: number): string {
  return `${((i % 7) / 6) * 100}% ${(Math.floor(i / 7) / 4) * 100}%`;
}

const SIZES = { xs: 24, sm: 30, md: 44, lg: 64, xl: 104 } as const;
const PAPER = "var(--color-paper)";

export type AbakAvatarSize = keyof typeof SIZES | number;

export function AbakAvatar({
  name,
  size = 36,
  src,
  ring = true,
  className,
}: {
  name: string;
  size?: AbakAvatarSize;
  src?: string | null;
  // true = prototype ring, false = none, string = custom box-shadow
  ring?: boolean | string;
  className?: string;
}) {
  const px = typeof size === "number" ? size : SIZES[size];
  const shadow =
    typeof ring === "string"
      ? ring
      : !ring
        ? undefined
        : size === "xl"
          ? `0 0 0 4px ${PAPER}, 0 0 0 6px var(--color-brand-50)`
          : `0 0 0 2px ${PAPER}`;
  return (
    <span
      aria-hidden="true"
      data-slot="abak-avatar"
      data-avatar-index={src ? undefined : avatarIndex(name)}
      className={cn("inline-block shrink-0 rounded-full bg-surface-sunken", className)}
      style={{
        width: px,
        height: px,
        boxShadow: shadow,
        backgroundImage: src ? `url("${src}")` : "url(/avatars/sprite.webp)",
        backgroundSize: src ? "cover" : "700% 500%",
        backgroundPosition: src ? "center" : avatarPosition(avatarIndex(name)),
      }}
    />
  );
}
