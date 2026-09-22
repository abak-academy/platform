"use client"

import * as React from "react"
import { Eye, EyeOff } from "lucide-react"

import { Input } from "@/components/ui/input"
import { useTranslation } from "@/lib/i18n"
import { cn } from "@/lib/utils"

// Password input with a show/hide toggle. Same props as Input; renders an
// Eye/EyeOff button inside the field. Defaults to masked.
function PasswordInput({ className, ...props }: React.ComponentProps<"input">) {
  const { t } = useTranslation()
  const [show, setShow] = React.useState(false)

  return (
    <div className="relative">
      <Input
        type={show ? "text" : "password"}
        className={cn("pr-10", className)}
        {...props}
      />
      <button
        type="button"
        onClick={() => setShow((p) => !p)}
        aria-label={show ? t("auth_hide_password") : t("auth_show_password")}
        className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-400 transition-colors hover:text-ink-600"
      >
        {show ? <EyeOff size={16} /> : <Eye size={16} />}
      </button>
    </div>
  )
}

export { PasswordInput }
