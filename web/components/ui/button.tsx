import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 rounded-[var(--radius-pill)] whitespace-nowrap transition-all duration-150 ease-spring outline-none focus-visible:outline-[2.5px] focus-visible:outline-offset-2 focus-visible:outline-solid focus-visible:outline-brand-600 disabled:pointer-events-none disabled:opacity-45 disabled:shadow-none aria-invalid:outline-danger-600 motion-reduce:hover:translate-y-0 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default:
          "bg-ember-600 text-on-ember shadow-cta hover:-translate-y-0.5 hover:shadow-cta-hover",
        secondary:
          "bg-brand-600 text-on-brand shadow-brand hover:-translate-y-0.5 hover:bg-brand-700",
        outline:
          "border-[1.5px] border-line bg-surface text-ink-900 hover:border-ink-900",
        ghost: "text-ink-900 hover:bg-surface-sunken",
        destructive: "bg-danger-600 text-on-danger hover:bg-danger-600/90",
        dark: "bg-brand-600 text-white shadow-[var(--shadow-brand)] hover:-translate-y-0.5 hover:bg-brand-600",
        light: "bg-white text-brand-900 hover:-translate-y-0.5",
        "line-light": "border-[1.5px] border-white/60 text-white hover:bg-white/10",
        link: "text-brand-ink underline-offset-4 hover:underline",
      },
      size: {
        default: "h-[var(--spacing-control-md)] px-[18px] type-label has-[>svg]:px-4",
        xs: "h-6 gap-1 px-2 text-xs font-semibold has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-[var(--spacing-control-sm)] gap-1.5 px-4 type-label-sm has-[>svg]:px-3",
        lg: "h-[var(--spacing-control-lg)] px-7 type-h3 has-[>svg]:px-5",
        icon: "size-[var(--spacing-control-md)]",
        "icon-xs": "size-6 [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-8",
        "icon-lg": "size-[var(--spacing-control-lg)]",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
