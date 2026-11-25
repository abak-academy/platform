import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

const TYPE_STYLES = [
  "display-hero", "display-xl", "display", "h1", "h2", "h3", "label", "label-sm",
  "label-xs", "body", "body-sm", "caption", "overline",
];

// type-* utilities set font-size; register them so a caller's text-xs replaces them instead of fighting on CSS order.
const twMerge = extendTailwindMerge({
  extend: { classGroups: { "font-size": [{ type: TYPE_STYLES }] } },
});

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
