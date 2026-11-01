"use client";

import { useEffect } from "react";
import { AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { useTranslation } from "@/lib/i18n";

// Used by the route-level error.tsx files so a failing page keeps its shell
// (sidebar, header) instead of being replaced by the full-screen app error.
export function SectionError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const { t } = useTranslation();

  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <Card role="alert" className="max-w-xl border-danger/30 bg-danger-bg px-5 py-4">
      <div className="flex items-center gap-3">
        <AlertCircle className="size-5 shrink-0 text-danger" />
        <p className="flex-1 text-sm text-ink-700">{t("sys_error_load")}</p>
        <Button variant="outline" size="sm" onClick={reset}>
          {t("retry")}
        </Button>
      </div>
    </Card>
  );
}
