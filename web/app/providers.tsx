"use client"

import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { useEffect, useState } from "react"
import { Toaster } from "@/components/ui/sonner"
import { useUIStore } from "@/stores/ui"
import { useAuthStore } from "@/stores/auth"

export function ThemeSync() {
  const theme = useUIStore((s) => (s.publicShells > 0 ? "light" : s.theme))
  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme)
  }, [theme])
  return null
}

function SessionSync() {
  const hasToken = useAuthStore((s) => !!s.token)
  useEffect(() => {
    if (hasToken) document.documentElement.dataset.session = "1"
    else delete document.documentElement.dataset.session
  }, [hasToken])
  return null
}

function LangSync() {
  const lang = useUIStore((s) => s.lang)
  useEffect(() => {
    document.documentElement.lang = lang
  }, [lang])
  return null
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 30 * 1000,
            refetchOnWindowFocus: false,
            retry: 1,
          },
        },
      })
  )

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeSync />
      <SessionSync />
      <LangSync />
      {children}
      <Toaster />
    </QueryClientProvider>
  )
}