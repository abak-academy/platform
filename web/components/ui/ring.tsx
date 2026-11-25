import * as React from "react"

import { cn } from "@/lib/utils"

type RingProps = {
  value: number
  label: string
  fill?: string
  track?: string
  className?: string
  children?: React.ReactNode
}

function Ring({
  value,
  label,
  fill = "var(--color-brand-600)",
  track = "var(--color-surface-sunken)",
  className,
  children,
}: RingProps) {
  const pct = Number.isFinite(value) ? Math.min(100, Math.max(0, value)) : 0
  return (
    <div
      role="img"
      aria-label={label}
      data-slot="ring"
      className={cn("grid size-32 shrink-0 place-items-center rounded-full", className)}
      style={{ background: `conic-gradient(${fill} 0% ${pct}%, ${track} ${pct}% 100%)` }}
    >
      <div className="grid size-[72%] place-items-center rounded-full bg-surface">{children}</div>
    </div>
  )
}

export { Ring }
