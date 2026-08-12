"use client";

import Link from "next/link";
import type { LucideIcon } from "lucide-react";

export interface QuickAction {
  icon: LucideIcon;
  label: string;
  href: string;
}

interface QuickActionTilesProps {
  title: string;
  actions: QuickAction[];
}

// Anchors, not buttons: a tile must survive middle-click and open-in-new-tab.
export function QuickActionTiles({ title, actions }: QuickActionTilesProps) {
  if (actions.length === 0) return null;

  return (
    <div className="md-card-outlined">
      <h3 className="text-title-large mb-6">{title}</h3>
      <div className="grid grid-cols-2 gap-3">
        {actions.map((action) => (
          <Link
            key={action.href}
            href={action.href}
            className="flex flex-col items-center gap-2 rounded-[12px] p-4 text-center transition-transform duration-200 hover:-translate-y-0.5 hover:shadow-lg"
            style={{ backgroundColor: "var(--md-sys-color-surface-container-high)" }}
          >
            <div
              className="flex size-10 items-center justify-center rounded-[10px]"
              style={{
                backgroundColor: "var(--md-sys-color-primary-container)",
                color: "var(--md-sys-color-primary)",
              }}
            >
              <action.icon size={20} />
            </div>
            <span className="text-label" style={{ fontWeight: 500 }}>
              {action.label}
            </span>
          </Link>
        ))}
      </div>
    </div>
  );
}
