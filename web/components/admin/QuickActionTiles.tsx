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
      <h3 className="type-h2 mb-3.5">{title}</h3>
      <div className="grid grid-cols-2 gap-2.5 border-t border-dashed border-line pt-3.5">
        {actions.map((action) => (
          <Link
            key={action.href}
            href={action.href}
            className="flex items-center gap-2.5 rounded-[14px] bg-surface-sunken p-3 text-[13px] font-bold text-ink-900 transition-transform duration-200 hover:-translate-y-0.5"
          >
            <action.icon size={18} className="shrink-0 text-ember-700" aria-hidden />
            <span>{action.label}</span>
          </Link>
        ))}
      </div>
    </div>
  );
}
