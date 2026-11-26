"use client"

import * as React from "react"
import { Switch as SwitchPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

function Switch({
  className,
  size = "default",
  ...props
}: React.ComponentProps<typeof SwitchPrimitive.Root> & { size?: "default" | "sm" }) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      data-size={size}
      className={cn(
        "peer group/switch inline-flex shrink-0 cursor-pointer items-center rounded-full bg-line-strong outline-none transition-colors",
        "h-7 w-[46px] data-[size=sm]:h-[22px] data-[size=sm]:w-9",
        "data-[state=checked]:bg-brand-600",
        "focus-visible:outline-[2.5px] focus-visible:outline-offset-2 focus-visible:outline-solid focus-visible:outline-brand-600",
        "disabled:cursor-not-allowed disabled:opacity-45",
        className
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className={cn(
          "pointer-events-none block size-[22px] translate-x-[3px] rounded-full bg-white shadow-[0_1px_3px_rgba(0,0,0,.2)] transition-transform duration-300 ease-spring",
          "data-[state=checked]:translate-x-[21px]",
          "group-data-[size=sm]/switch:size-4 group-data-[size=sm]/switch:translate-x-[3px] group-data-[size=sm]/switch:data-[state=checked]:translate-x-[17px]"
        )}
      />
    </SwitchPrimitive.Root>
  )
}

export { Switch }
