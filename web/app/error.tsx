"use client";

import { useEffect } from "react";
import { AlertCircle } from "lucide-react";

import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const { t } = useTranslation();

  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-md border-danger/30 bg-danger-bg px-5 py-4">
        <div className="flex items-center gap-3">
          <AlertCircle className="size-5 text-danger" />
          <p className="flex-1 text-sm text-ink-700">{t("sys_error_load")}</p>
          <Button variant="outline" size="sm" onClick={reset}>
            {t("retry")}
          </Button>
        </div>
      </Card>
    </div>
  );
}
