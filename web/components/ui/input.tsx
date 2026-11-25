import * as React from "react"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-[var(--spacing-input)] w-full min-w-0 rounded-md border-[1.5px] border-line bg-surface px-3.5 py-1 type-body text-ink-900 transition-[color,box-shadow,border-color] outline-none selection:bg-primary selection:text-primary-foreground file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-ink-600 hover:border-line-strong disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-45",
        "focus-visible:border-brand-600 focus-visible:ring-4 focus-visible:ring-brand-50",
        "aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40",
        className
      )}
      {...props}
    />
  )
}

export { Input }
