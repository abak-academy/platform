"use client";

import { useEffect, useRef, useState, type RefObject } from "react";

export interface Point {
  x: number;
  y: number;
}

/** SVG y grows downward, so the max value maps to 0 and zero maps to `height`. */
export function scaleY(value: number, max: number, height: number): number {
  if (max <= 0) return height;
  return height - (value / max) * height;
}

/** A readable axis ceiling. Never returns 0 — an all-zero series still needs a scale. */
export function niceMax(values: number[]): number {
  const peak = values.length ? Math.max(...values) : 0;
  if (peak <= 0) return 1;
  const magnitude = Math.pow(10, Math.floor(Math.log10(peak)));
  return Math.ceil(peak / magnitude) * magnitude;
}

function xAt(index: number, count: number, width: number): number {
  if (count <= 1) return 0;
  return (index / (count - 1)) * width;
}

/** Rounds to 2dp and drops trailing zeros — "0" not "0.00". Keeps path data
 *  short and keeps the emitted string predictable for tests. */
function num(n: number): string {
  return String(Math.round(n * 100) / 100);
}

export function buildPath(values: number[], max: number, width: number, height: number): string {
  if (!values.length) return "";
  return values
    .map((v, i) => {
      const x = xAt(i, values.length, width);
      const y = scaleY(v, max, height);
      return `${i === 0 ? "M" : "L"}${num(x)},${num(y)}`;
    })
    .join("");
}

export function buildAreaPath(values: number[], max: number, width: number, height: number): string {
  const line = buildPath(values, max, width, height);
  if (!line) return "";
  const lastX = xAt(values.length - 1, values.length, width);
  return `${line}L${num(lastX)},${height}L0,${height}Z`;
}

/**
 * Motion is opt-out at the OS level. Read as a state rather than inline so the
 * first render already knows, and so a change mid-session takes effect.
 */
export function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);

  useEffect(() => {
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReduced(mq.matches);
    const onChange = (e: MediaQueryListEvent) => setReduced(e.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  return reduced;
}

export type HoverIndexMode = "point" | "band";

/**
 * Maps a 0..1 horizontal fraction of the plot to a data index. "point" charts
 * place index 0 at x=0 and the last index at x=width (nearest wins, matching
 * buildPath's xAt); "band" charts give every index an equal-width slot.
 */
export function indexFromFraction(
  fraction: number,
  count: number,
  mode: HoverIndexMode,
): number | null {
  if (count <= 0) return null;
  const raw =
    mode === "band" ? Math.floor(fraction * count) : Math.round(fraction * (count - 1));
  return Math.min(count - 1, Math.max(0, raw));
}

/** Horizontal centre of a data index, as a percentage of the plot width. */
export function xPercentFor(index: number, count: number, mode: HoverIndexMode): number {
  if (count <= 0) return 0;
  if (mode === "band") return ((index + 0.5) / count) * 100;
  if (count === 1) return 0;
  return (index / (count - 1)) * 100;
}

export interface ChartHover {
  index: number | null;
  containerRef: RefObject<HTMLDivElement | null>;
  hoverProps: {
    tabIndex: number;
    onPointerMove: (e: React.PointerEvent<HTMLDivElement>) => void;
    onPointerLeave: () => void;
    onBlur: () => void;
    onKeyDown: (e: React.KeyboardEvent<HTMLDivElement>) => void;
  };
}

/**
 * Pointer and keyboard selection of a data index. Percentages, not viewBox
 * units: every chart sets preserveAspectRatio="none", so viewBox coordinates
 * do not survive the stretch to the card's real width.
 */
export function useChartHover(count: number, mode: HoverIndexMode): ChartHover {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [index, setIndex] = useState<number | null>(null);

  // A period change swaps the series under a held selection; index 27 of the
  // old 30 buckets is meaningless against the new 7.
  useEffect(() => {
    setIndex(null);
  }, [count]);

  return {
    index,
    containerRef,
    hoverProps: {
      tabIndex: 0,
      onPointerMove: (e) => {
        const el = containerRef.current;
        if (!el) return;
        const rect = el.getBoundingClientRect();
        // jsdom reports a zero-size rect for every element. Dividing by it
        // yields NaN, which Math.max/min collapse to 0 — a pointer test would
        // then pass by landing on index 0 for the wrong reason.
        if (rect.width <= 0) return;
        const next = indexFromFraction((e.clientX - rect.left) / rect.width, count, mode);
        if (next !== null) setIndex(next);
      },
      onPointerLeave: () => setIndex(null),
      onBlur: () => setIndex(null),
      onKeyDown: (e) => {
        if (e.key === "Escape") {
          setIndex(null);
          return;
        }
        const step = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
        if (step === 0) return;
        e.preventDefault();
        setIndex((cur) => {
          const from = cur ?? (step > 0 ? -1 : count);
          return Math.min(count - 1, Math.max(0, from + step));
        });
      },
    },
  };
}
