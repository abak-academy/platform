"use client";

import { useEffect, useState } from "react";

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
