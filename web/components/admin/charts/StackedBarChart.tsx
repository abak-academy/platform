"use client";

import { niceMax, usePrefersReducedMotion } from "./chart-utils";

const W = 300;
const H = 100;
const GAP_RATIO = 0.25;

interface Segment {
  values: number[];
  color: string;
  label: string;
}

interface StackedBarChartProps {
  labels: string[];
  bottom: Segment;
  top: Segment;
  height?: number;
  emptyLabel: string;
}

export function StackedBarChart({
  labels,
  bottom,
  top,
  height = H,
  emptyLabel,
}: StackedBarChartProps) {
  const reduced = usePrefersReducedMotion();

  const count = Math.max(bottom.values.length, top.values.length);
  const totals = Array.from({ length: count }, (_, i) => (bottom.values[i] ?? 0) + (top.values[i] ?? 0));
  const hasData = totals.some((v) => v !== 0);

  if (!count || !hasData) {
    return (
      <div
        className="flex items-center justify-center text-label color-on-surface-variant"
        style={{ height }}
      >
        {emptyLabel}
      </div>
    );
  }

  // Scaled on the stacked total, not on either segment — otherwise a tall
  // bottom segment pushes the top one off the top of the viewBox.
  const max = niceMax(totals);
  const slot = W / count;
  const barW = slot * (1 - GAP_RATIO);

  const bottomSum = bottom.values.reduce((a, b) => a + b, 0);
  const topSum = top.values.reduce((a, b) => a + b, 0);
  const grand = bottomSum + topSum;
  const pct = (v: number) => (grand > 0 ? Math.round((v / grand) * 100) : 0);

  const summaryCount = Math.min(labels.length, bottom.values.length, top.values.length);

  return (
    <div>
      <svg
        viewBox={`0 0 ${W} ${height}`}
        preserveAspectRatio="none"
        style={{ width: "100%", height }}
        role="img"
        aria-label={`${bottom.label}, ${top.label}`}
      >
        {Array.from({ length: count }, (_, i) => {
          const b = bottom.values[i] ?? 0;
          const t = top.values[i] ?? 0;
          const bH = (b / max) * height;
          const tH = (t / max) * height;
          const x = i * slot + (slot - barW) / 2;
          const cls = reduced ? "" : "chart-grow";
          const delay = reduced ? undefined : { animationDelay: `${i * 0.06}s` };
          return (
            <g key={i}>
              <rect
                className={cls}
                style={delay}
                x={x}
                y={height - bH}
                width={barW}
                height={bH}
                fill={bottom.color}
              />
              <rect
                className={cls}
                style={delay}
                x={x}
                y={height - bH - tH}
                width={barW}
                height={tH}
                fill={top.color}
              />
            </g>
          );
        })}
      </svg>

      <div className="text-label color-on-surface-variant mt-2 flex flex-wrap gap-3">
        <span>
          <span
            className="inline-block h-[3px] w-[10px] rounded-sm align-middle mr-1"
            style={{ backgroundColor: bottom.color }}
          />
          {bottom.label} {pct(bottomSum)}%
        </span>
        <span>
          <span
            className="inline-block h-[3px] w-[10px] rounded-sm align-middle mr-1"
            style={{ backgroundColor: top.color }}
          />
          {top.label} {pct(topSum)}%
        </span>
      </div>

      <div className="sr-only">
        {Array.from(
          { length: summaryCount },
          (_, i) => `${labels[i]}: ${bottom.label} ${bottom.values[i]}, ${top.label} ${top.values[i]}`
        ).join("; ")}
      </div>
    </div>
  );
}
