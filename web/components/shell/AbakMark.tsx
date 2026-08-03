import { AbakLogo } from "@/components/brand/AbakLogo";

export function AbakMark({ size = 28 }: { size?: number }) {
  return <AbakLogo size={size} className="text-brand-600" />;
}
