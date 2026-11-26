"use client"

import * as React from "react"
import { Select as SelectPrimitive } from "radix-ui"
import { Check, ChevronDown } from "lucide-react"

import { cn } from "@/lib/utils"

const Select = SelectPrimitive.Root

const SelectGroup = SelectPrimitive.Group

const SelectLabel = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Label>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Label>
>(({ className, children, ...props }, ref) => (
  <SelectPrimitive.Label
    ref={ref}
    className={cn("px-2 py-1.5 text-xs font-semibold text-ink-500", className)}
    {...props}
  >
    {children}
  </SelectPrimitive.Label>
))
SelectLabel.displayName = SelectPrimitive.Label.displayName

const SelectValue = SelectPrimitive.Value

const TRIGGER_SIZE = {
  default: "h-[var(--spacing-input)]",
  sm: "h-[var(--spacing-control-sm)] w-auto rounded-full type-label-sm",
  pill: "h-10 w-auto rounded-full type-label-sm",
} as const

const SelectTrigger = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Trigger> & { size?: keyof typeof TRIGGER_SIZE }
>(({ className, children, size = "default", ...props }, ref) => (
  <SelectPrimitive.Trigger
    ref={ref}
    className={cn(
      "flex w-full items-center justify-between rounded-md border-[1.5px] border-line bg-surface px-3.5 py-1 type-body text-ink-900 transition-[color,box-shadow,border-color] outline-none hover:border-line-strong",
      "focus-visible:border-brand-600 focus-visible:ring-4 focus-visible:ring-brand-50",
      "disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-45",
      "aria-invalid:border-destructive aria-invalid:ring-destructive/20",
      "[&>span]:line-clamp-1",
      TRIGGER_SIZE[size],
      className
    )}
    {...props}
  >
    {children}
    <SelectPrimitive.Icon asChild>
      <ChevronDown className="size-4 opacity-50" />
    </SelectPrimitive.Icon>
  </SelectPrimitive.Trigger>
))
SelectTrigger.displayName = SelectPrimitive.Trigger.displayName

const SelectContent = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Content>
>(({ className, children, position = "popper", ...props }, ref) => (
  <SelectPrimitive.Portal>
    <SelectPrimitive.Content
      ref={ref}
      position={position}
      className={cn(
        "relative z-[var(--z-dialog)] max-h-96 min-w-[8rem] overflow-hidden rounded-[20px] border border-line bg-popover text-popover-foreground shadow-pop",
        position === "popper" && "translate-y-1",
        className
      )}
      {...props}
    >
      <SelectPrimitive.Viewport
        className={cn(
          "p-2",
          position === "popper" && "h-[var(--radix-select-trigger-height)] w-full min-w-[var(--radix-select-trigger-width)]"
        )}
      >
        {children}
      </SelectPrimitive.Viewport>
    </SelectPrimitive.Content>
  </SelectPrimitive.Portal>
))
SelectContent.displayName = SelectPrimitive.Content.displayName

const SelectItem = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Item>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Item>
>(({ className, children, ...props }, ref) => (
  <SelectPrimitive.Item
    ref={ref}
    className={cn(
      "relative flex w-full cursor-default select-none items-center rounded-[14px] py-2.5 pl-8 pr-3 type-label-sm outline-none focus:bg-brand-50 focus:text-brand-ink data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
      className
    )}
    {...props}
  >
    <span className="absolute left-2 flex size-3.5 items-center justify-center">
      <SelectPrimitive.ItemIndicator>
        <Check className="size-4" />
      </SelectPrimitive.ItemIndicator>
    </span>
    <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
  </SelectPrimitive.Item>
))
SelectItem.displayName = SelectPrimitive.Item.displayName

export { Select, SelectGroup, SelectLabel, SelectValue, SelectTrigger, SelectContent, SelectItem }
