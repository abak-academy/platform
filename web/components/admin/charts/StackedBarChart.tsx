"use client";

import { niceMax, useChartHover, usePrefersReducedMotion } from "./chart-utils";
import { ChartHoverLayer } from "./ChartHoverLayer";
import { useTranslation } from "@/lib/i18n";

const W = 300;
const H = 100;
const GAP_RATIO = 0.25;

interface Segment {
  values: number[];
  color: string;
  label: string;
  format?: (value: number) => string;
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
  const hover = useChartHover(count, "band");
  const { t } = useTranslation();

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
  // topPct is the complement of bottomPct, not its own independent rounding —
  // rounding both shares away from zero can otherwise land on 34% + 67% = 101%.
  const bottomPct = grand > 0 ? Math.round((bottomSum / grand) * 100) : 0;
  const topPct = grand > 0 ? 100 - bottomPct : 0;

  const summaryCount = Math.min(labels.length, bottom.values.length, top.values.length);

  return (
    <div>
      <div
        ref={hover.containerRef}
        data-testid="chart-hover-area"
        className="relative outline-none"
        style={{ height }}
        role="group"
        aria-label={t("chart_keyboard_hint")}
        {...hover.hoverProps}
      >
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

        {hover.index !== null ? (
          <ChartHoverLayer
            index={hover.index}
            count={count}
            mode="band"
            title={labels[hover.index] ?? ""}
            rows={[
              {
                label: bottom.label,
                color: bottom.color,
                value: (bottom.format ?? String)(bottom.values[hover.index] ?? 0),
              },
              {
                label: top.label,
                color: top.color,
                value: (top.format ?? String)(top.values[hover.index] ?? 0),
              },
            ]}
          />
        ) : null}
      </div>

      <div className="text-label color-on-surface-variant mt-2 flex flex-wrap gap-3">
        <span>
          <span
            className="inline-block h-[3px] w-[10px] rounded-sm align-middle mr-1"
            style={{ backgroundColor: bottom.color }}
          />
          {bottom.label} {bottomPct}%
        </span>
        <span>
          <span
            className="inline-block h-[3px] w-[10px] rounded-sm align-middle mr-1"
            style={{ backgroundColor: top.color }}
          />
          {top.label} {topPct}%
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
