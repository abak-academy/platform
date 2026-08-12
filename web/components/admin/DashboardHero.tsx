"use client";

import type { LucideIcon } from "lucide-react";

const DEFAULT_GRADIENT = "linear-gradient(135deg, #1A5CFF 0%, #0A3DBF 55%, #005B8E 100%)";

interface DashboardHeroProps {
  icon: LucideIcon;
  badge: string;
  name: string;
  subtitle: string;
  gradient?: string;
  className?: string;
}

// Literal hex, not MD3 variables: keeps one brand gradient in both palettes.
export function DashboardHero({
  icon: Icon,
  badge,
  name,
  subtitle,
  gradient = DEFAULT_GRADIENT,
  className = "mb-8",
}: DashboardHeroProps) {
  return (
    <div
      className={`${className} rounded-[20px] px-8 py-7`}
      style={{ background: gradient, color: "#FFFFFF", boxShadow: "0 4px 24px rgba(26,92,255,0.28)" }}
    >
      <div className="flex items-center gap-6">
        <div
          className="flex size-[72px] shrink-0 items-center justify-center rounded-[24px]"
          style={{ backgroundColor: "rgba(255,255,255,0.18)", backdropFilter: "blur(8px)" }}
        >
          <Icon size={36} color="#FFFFFF" />
        </div>
        <div>
          <div
            className="text-label"
            style={{ letterSpacing: "0.08em", textTransform: "uppercase", opacity: 0.75 }}
          >
            {badge}
          </div>
          <h1 className="text-headline" style={{ color: "#FFFFFF" }}>
            {name}
          </h1>
          <p className="text-body" style={{ marginTop: "4px", opacity: 0.85 }}>
            {subtitle}
          </p>
        </div>
      </div>
    </div>
  );
}
