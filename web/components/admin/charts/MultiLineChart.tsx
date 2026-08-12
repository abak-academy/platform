"use client";

import { buildPath, niceMax, scaleY, useChartHover, usePrefersReducedMotion } from "./chart-utils";
import { ChartHoverLayer } from "./ChartHoverLayer";
import { useTranslation } from "@/lib/i18n";

const W = 300;
const H = 100;

interface Series {
  values: number[];
  color: string;
  label: string;
  format?: (value: number) => string;
}

interface MultiLineChartProps {
  labels: string[];
  series: Series[];
  height?: number;
  emptyLabel: string;
}

export function MultiLineChart({ labels, series, height = H, emptyLabel }: MultiLineChartProps) {
  const reduced = usePrefersReducedMotion();
  const hover = useChartHover(series[0]?.values.length ?? 0, "point");
  const { t } = useTranslation();

  const allValues = series.flatMap((s) => s.values);
  const hasData = allValues.some((v) => v !== 0);

  if (!allValues.length || !hasData) {
    return (
      <div
        className="flex items-center justify-center text-label color-on-surface-variant"
        style={{ height }}
      >
        {emptyLabel}
      </div>
    );
  }

  // One shared scale: these are all student head-counts, so a per-series axis
  // would make three unrelated shapes look comparable when they are not.
  const max = niceMax(allValues);

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
          aria-label={series.map((s) => s.label).join(", ")}
        >
          {[0.25, 0.5, 0.75].map((f) => (
            <line
              key={f}
              x1="0"
              y1={height * f}
              x2={W}
              y2={height * f}
              stroke="var(--md-sys-color-outline-variant)"
              strokeWidth="1"
            />
          ))}

          {series.map((s, i) =>
            s.values.length === 1 ? (
              <circle
                key={s.label}
                cx={0}
                cy={scaleY(s.values[0], max, height)}
                r={4}
                fill={s.color}
              />
            ) : (
              <path
                key={s.label}
                className={reduced ? "" : "chart-draw"}
                style={reduced ? undefined : { animationDelay: `${0.1 + i * 0.15}s` }}
                d={buildPath(s.values, max, W, height)}
                fill="none"
                stroke={s.color}
                strokeWidth="2"
                strokeLinejoin="round"
              />
            )
          )}
        </svg>

        {hover.index !== null ? (
          <ChartHoverLayer
            index={hover.index}
            count={series[0]?.values.length ?? 0}
            mode="point"
            title={labels[hover.index] ?? ""}
            rows={series.map((s) => ({
              label: s.label,
              color: s.color,
              value: (s.format ?? String)(s.values[hover.index!] ?? 0),
              yFraction: scaleY(s.values[hover.index!] ?? 0, max, height) / height,
            }))}
          />
        ) : null}
      </div>

      <div className="text-label color-on-surface-variant mt-2 flex flex-wrap gap-3">
        {series.map((s) => (
          <span key={s.label}>
            <span
              className="inline-block h-[3px] w-[10px] rounded-sm align-middle mr-1"
              style={{ backgroundColor: s.color }}
            />
            {s.label}
          </span>
        ))}
      </div>

      <div className="sr-only">
        {Array.from(
          { length: Math.min(labels.length, ...series.map((s) => s.values.length)) },
          (_, i) => `${labels[i]}: ${series.map((s) => `${s.label} ${s.values[i]}`).join(", ")}`
        ).join("; ")}
      </div>
    </div>
  );
}
