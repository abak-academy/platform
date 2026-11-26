"use client"

import * as React from "react"
import { Progress as ProgressPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

const FILL = {
  teal: "bg-teal-600",
  ember: "bg-ember-600",
  hatch: "bg-[repeating-linear-gradient(135deg,var(--color-ember-500)_0_5px,var(--color-ember-600)_5px_9px)]",
} as const

function Progress({
  className,
  value,
  variant = "teal",
  ...props
}: React.ComponentProps<typeof ProgressPrimitive.Root> & { variant?: keyof typeof FILL }) {
  return (
    <ProgressPrimitive.Root
      data-slot="progress"
      value={value}
      className={cn(
        "relative h-2 w-full overflow-hidden rounded-full bg-surface-sunken",
        className
      )}
      {...props}
    >
      <ProgressPrimitive.Indicator
        data-slot="progress-indicator"
        className={cn("h-full w-full flex-1 rounded-[inherit] transition-all duration-500", FILL[variant])}
        style={{ transform: `translateX(-${100 - (value || 0)}%)` }}
      />
    </ProgressPrimitive.Root>
  )
}

export { Progress }
