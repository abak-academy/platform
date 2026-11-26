"use client";

/** A simple SVG donut chart showing one filled segment + remainder. */
export function Donut({
  size = 120,
  thickness = 16,
  value,
  centerLabel,
  centerSub,
}: {
  size?: number;
  thickness?: number;
  value: number; // 0–1
  centerLabel: string;
  centerSub: string;
}) {
  const r = (size - thickness) / 2;
  const circ = 2 * Math.PI * r;
  const filled = circ * Math.min(value, 1);
  const cx = size / 2;
  const cy = size / 2;

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      className="shrink-0"
      role="img"
      aria-label={`${Math.round(value * 100)}%`}
    >
      {/* Background ring */}
      <circle
        cx={cx}
        cy={cy}
        r={r}
        fill="none"
        stroke="var(--color-brand-100)"
        strokeWidth={thickness}
      />
      {/* Filled arc */}
      <circle
        cx={cx}
        cy={cy}
        r={r}
        fill="none"
        stroke="var(--color-brand-600)"
        strokeWidth={thickness}
        strokeDasharray={`${filled} ${circ - filled}`}
        strokeDashoffset={0}
        strokeLinecap="round"
        transform={`rotate(-90 ${cx} ${cy})`}
        style={{ transition: "stroke-dasharray 0.5s ease" }}
      />
      {/* Center label */}
      <text
        x={cx}
        y={cy - 4}
        textAnchor="middle"
        dominantBaseline="central"
        className="fill-ink-900"
        style={{ fontSize: size * 0.14, fontWeight: 700, fontFamily: "serif" }}
      >
        {centerLabel}
      </text>
      <text
        x={cx}
        y={cy + size * 0.1}
        textAnchor="middle"
        dominantBaseline="central"
        className="fill-ink-500"
        style={{ fontSize: size * 0.08 }}
      >
        {centerSub}
      </text>
    </svg>
  );
}
