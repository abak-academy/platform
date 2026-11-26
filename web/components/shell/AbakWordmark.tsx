import { AbakLogo } from "@/components/brand/AbakLogo";
import { cn } from "@/lib/utils";

export function AbakWordmark({
  light = false,
  size = 34,
  hideTextOnPhone = false,
  className,
}: {
  light?: boolean;
  size?: number;
  hideTextOnPhone?: boolean;
  className?: string;
}) {
  return (
    <span className={cn("flex shrink-0 items-center gap-2.5", className)}>
      <AbakLogo size={size} />
      <span aria-hidden className={cn("leading-none", hideTextOnPhone && "hidden min-[421px]:block")}>
        <b className={cn("block font-serif text-[20px] font-extrabold tracking-[-0.03em]", light ? "text-white" : "text-ink-900")}>
          abak
        </b>
        <small className={cn("mt-1 block font-mono text-[8.5px] font-semibold uppercase tracking-[0.42em]", light ? "text-white/70" : "text-ink-600")}>
          academy
        </small>
      </span>
    </span>
  );
}
