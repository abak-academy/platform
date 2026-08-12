"use client";

import { useId } from "react";
import { buildAreaPath, buildPath, niceMax, scaleY, useChartHover, usePrefersReducedMotion } from "./chart-utils";
import { ChartHoverLayer } from "./ChartHoverLayer";
import { useTranslation } from "@/lib/i18n";

const W = 620;
const H = 130;

interface Series {
  values: number[];
  color: string;
  label: string;
  format?: (value: number) => string;
}

interface AreaLineChartProps {
  labels: string[];
  area: Series;
  line: Series;
  height?: number;
  emptyLabel: string;
}

export function AreaLineChart({ labels, area, line, height = H, emptyLabel }: AreaLineChartProps) {
  const reactId = useId();
  const gradientId = `${reactId}-gradient`;
  const clipId = `${reactId}-clip`;
  const reduced = usePrefersReducedMotion();
  const hover = useChartHover(area.values.length, "point");
  const { t } = useTranslation();

  const hasData = area.values.some((v) => v !== 0) || line.values.some((v) => v !== 0);

  if (!area.values.length || !hasData) {
    return (
      <div
        className="flex items-center justify-center text-label color-on-surface-variant"
        style={{ height }}
      >
        {emptyLabel}
      </div>
    );
  }

  const areaMax = niceMax(area.values);
  const lineMax = niceMax(line.values);
  const drawClass = reduced ? "" : "chart-draw";
  const isSinglePoint = area.values.length === 1;

  return (
    <div>
      <div
        ref={hover.containerRef}
        data-testid="chart-hover-area"
        className="relative outline-none focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
        style={{ height, outlineColor: "var(--md-sys-color-primary)" }}
        role="group"
        aria-label={t("chart_keyboard_hint")}
        {...hover.hoverProps}
      >
        <svg
          viewBox={`0 0 ${W} ${height}`}
          preserveAspectRatio="none"
          style={{ width: "100%", height }}
          role="img"
          aria-label={`${area.label}, ${line.label}`}
        >
          <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor={area.color} stopOpacity="0.34" />
              <stop offset="1" stopColor={area.color} stopOpacity="0" />
            </linearGradient>
            {/* Reveal for the dashed order-count line: .chart-draw's own
                stroke-dasharray would override the "5 3" dash pattern (CSS beats
                the SVG presentation attribute), erasing the dashes for good, not
                just during the draw. A clip-path wipe reveals the line without
                touching its dasharray. */}
            <clipPath id={clipId}>
              <rect x="0" y="0" width={W} height={height} className={drawClass ? "chart-wipe" : undefined} />
            </clipPath>
          </defs>

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

          {isSinglePoint ? (
            <>
              <circle cx={0} cy={scaleY(area.values[0], areaMax, height)} r={4} fill={area.color} />
              <circle
                cx={0}
                cy={scaleY(line.values[0], lineMax, height)}
                r={3}
                fill="none"
                stroke={line.color}
                strokeWidth="1.8"
              />
            </>
          ) : (
            <>
              <path d={buildAreaPath(area.values, areaMax, W, height)} fill={`url(#${gradientId})`} />
              <path
                className={drawClass}
                d={buildPath(area.values, areaMax, W, height)}
                fill="none"
                stroke={area.color}
                strokeWidth="2.2"
                strokeLinejoin="round"
              />
              <g clipPath={`url(#${clipId})`}>
                <path
                  d={buildPath(line.values, lineMax, W, height)}
                  fill="none"
                  stroke={line.color}
                  strokeWidth="1.8"
                  strokeDasharray="5 3"
                />
              </g>
            </>
          )}
        </svg>

        {hover.index !== null ? (
          <ChartHoverLayer
            index={hover.index}
            count={area.values.length}
            mode="point"
            title={labels[hover.index] ?? ""}
            rows={[
              {
                label: area.label,
                color: area.color,
                value: (area.format ?? String)(area.values[hover.index] ?? 0),
                yFraction: scaleY(area.values[hover.index] ?? 0, areaMax, height) / height,
              },
              {
                label: line.label,
                color: line.color,
                value: (line.format ?? String)(line.values[hover.index] ?? 0),
                yFraction: scaleY(line.values[hover.index] ?? 0, lineMax, height) / height,
              },
            ]}
          />
        ) : null}
      </div>

      <div className="text-label color-on-surface-variant mt-2 flex gap-4">
        <span>
          <span
            className="inline-block h-[3px] w-[10px] rounded-sm align-middle mr-1"
            style={{ backgroundColor: area.color }}
          />
          {area.label}
        </span>
        <span>
          <span
            className="inline-block h-[3px] w-[10px] rounded-sm align-middle mr-1"
            style={{ backgroundColor: line.color }}
          />
          {line.label}
        </span>
      </div>

      <div className="sr-only">
        {Array.from({ length: Math.min(labels.length, area.values.length, line.values.length) }, (_, i) =>
          `${labels[i]}: ${area.label} ${area.values[i]}, ${line.label} ${line.values[i]}`
        ).join(", ")}
      </div>
    </div>
  );
}
