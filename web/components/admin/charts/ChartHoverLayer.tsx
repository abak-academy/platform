"use client";

import { xPercentFor, type HoverIndexMode } from "./chart-utils";

export interface HoverRow {
  label: string;
  color: string;
  value: string;
  yFraction?: number;
}

interface ChartHoverLayerProps {
  index: number;
  count: number;
  mode: HoverIndexMode;
  title: string;
  rows: HoverRow[];
}

// HTML layer, not SVG: every chart sets preserveAspectRatio="none", so SVG circles become ellipses on the stretched viewBox.
export function ChartHoverLayer({ index, count, mode, title, rows }: ChartHoverLayerProps) {
  const xPct = xPercentFor(index, count, mode);
  const boxPct = Math.min(88, Math.max(12, xPct));

  return (
    <div className="pointer-events-none absolute inset-0">
      <div aria-hidden>
        {mode === "band" ? (
          <div
            data-testid="chart-guide"
            className="absolute top-0 bottom-0"
            style={{
              left: `${(index / count) * 100}%`,
              width: `${100 / count}%`,
              backgroundColor: "var(--md-sys-color-on-surface)",
              opacity: 0.08,
            }}
          />
        ) : (
          <div
            data-testid="chart-guide"
            className="absolute top-0 bottom-0 w-px"
            style={{ left: `${xPct}%`, backgroundColor: "var(--md-sys-color-outline)" }}
          />
        )}

        {rows.map((row) =>
          row.yFraction === undefined ? null : (
            <div
              key={row.label}
              data-testid="chart-marker"
              className="absolute size-[9px] rounded-full"
              style={{
                left: `${xPct}%`,
                top: `${row.yFraction * 100}%`,
                transform: "translate(-50%, -50%)",
                backgroundColor: row.color,
                boxShadow: "0 0 0 2px var(--md-sys-color-surface)",
              }}
            />
          ),
        )}
      </div>

      <div
        data-testid="chart-tooltip"
        role="status"
        className="chart-tooltip absolute top-0 z-10 rounded-[10px] px-3 py-2"
        style={{ left: `${boxPct}%`, transform: "translate(-50%, 8px)" }}
      >
        <div className="text-label" style={{ fontWeight: 600 }}>
          {title}
        </div>
        {rows.map((row) => (
          <div key={row.label} className="text-label flex items-center gap-2 whitespace-nowrap">
            <span
              className="inline-block size-[8px] shrink-0 rounded-full"
              style={{ backgroundColor: row.color }}
            />
            <span className="color-on-surface-variant">{row.label}</span>
            <span style={{ fontWeight: 600 }}>{row.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
