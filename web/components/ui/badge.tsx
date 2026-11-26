import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex h-[26px] w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-[var(--radius-pill)] border border-transparent px-[11px] type-label-xs whitespace-nowrap transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&>svg]:pointer-events-none [&>svg]:size-3",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground [a&]:hover:bg-primary/90",
        secondary:
          "bg-secondary text-secondary-foreground [a&]:hover:bg-secondary/90",
        destructive:
          "bg-destructive text-white focus-visible:ring-destructive/20 dark:bg-destructive/60 dark:focus-visible:ring-destructive/40 [a&]:hover:bg-destructive/90",
        outline:
          "border-border text-foreground [a&]:hover:bg-accent [a&]:hover:text-accent-foreground",
        ghost: "[a&]:hover:bg-accent [a&]:hover:text-accent-foreground",
        link: "text-primary underline-offset-4 [a&]:hover:underline",
        teal: "bg-teal-50 text-teal-700",
        gold: "bg-gold-50 text-gold-700",
        danger: "bg-danger-50 text-danger-600",
        sky: "bg-sky-50 text-sky-700",
        mute: "bg-mute-50 text-mute-700",
        brand: "bg-brand-50 text-brand-ink",
        navy: "bg-brand-100 text-brand-800",
        berry: "bg-berry-50 text-berry-700",
        ember: "bg-ember-50 text-ember-700",
        live: "gap-1.5 bg-ember-600 text-on-ember before:size-1.5 before:rounded-full before:bg-current before:animate-live before:content-['']",
        count: "h-5 min-w-5 bg-ember-600 px-1.5 text-on-ember",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

function Badge({
  className,
  variant = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot.Root : "span"

  return (
    <Comp
      data-slot="badge"
      data-variant={variant}
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  )
}

const STATUS_TONE: Record<string, NonNullable<VariantProps<typeof badgeVariants>["variant"]>> = {
  published: "teal", active: "teal", paid: "teal", completed: "teal",
  draft: "mute", inactive: "mute", archived: "mute", silver: "mute",
  pending: "gold", waiting: "gold", gold: "gold",
  expired: "danger", failed: "danger", cancelled: "danger", rejected: "danger",
  live: "live", bronze: "ember",
}

function statusTone(status: string) {
  return STATUS_TONE[status.toLowerCase()] ?? "mute"
}

export { Badge, badgeVariants, statusTone }
