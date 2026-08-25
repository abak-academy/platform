"use client";

import { useMemo, useState } from "react";
import { Download, FileText, KeyRound, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  type QuestionBundleScope,
  useCreateQuestionBundle,
  useQuestionBundle,
  useQuestionBundleDownload,
} from "@/lib/hooks/question-bundles";
import { useHasCapability } from "@/lib/hooks/use-capability";
import type { QuestionBundle } from "@/lib/types";

interface QuestionBundleControlsProps {
  scope: QuestionBundleScope;
  scopeId: string;
  disabled?: boolean;
}

function statusText(bundle: QuestionBundle | undefined): string {
  if (!bundle) return "Belum ada PDF yang dibuat di sesi ini.";
  switch (bundle.status) {
    case "queued":
      return "PDF masuk antrean worker.";
    case "processing":
      return "PDF sedang dibuat.";
    case "ready":
      return bundle.variant === "kunci"
        ? "PDF kunci siap diunduh. Jangan dibagikan ke peserta."
        : "PDF naskah siap diunduh.";
    case "failed":
      return bundle.error ? `Gagal membuat PDF: ${bundle.error}` : "Gagal membuat PDF.";
    default:
      return "Status PDF tidak diketahui.";
  }
}

export function QuestionBundleControls({ scope, scopeId, disabled }: QuestionBundleControlsProps) {
  const canGenerate = useHasCapability("question-bundles:write");
  const [bundleId, setBundleId] = useState<string | undefined>();
  const [pendingVariant, setPendingVariant] = useState<QuestionBundle["variant"] | undefined>();
  const createBundle = useCreateQuestionBundle(scope, scopeId);
  const status = useQuestionBundle(bundleId, Boolean(bundleId), (query) => {
    const nextStatus = query.state.data?.status;
    return nextStatus === "ready" || nextStatus === "failed" ? false : 2000;
  });
  const download = useQuestionBundleDownload();

  const bundle = status.data;
  const isBusy = createBundle.isPending || bundle?.status === "queued" || bundle?.status === "processing";
  const helper = useMemo(() => statusText(bundle), [bundle]);

  if (!canGenerate) return null;

  async function handleGenerate(includeAnswerKey: boolean) {
    const variant: QuestionBundle["variant"] = includeAnswerKey ? "kunci" : "naskah";
    setPendingVariant(variant);
    try {
      const next = await createBundle.mutateAsync({ include_answer_key: includeAnswerKey });
      setBundleId(next.id);
      setPendingVariant(undefined);
      toast.success("Pembuatan PDF masuk antrean.");
    } catch (err) {
      setPendingVariant(undefined);
      toast.error(err instanceof Error ? err.message : "Gagal membuat PDF.");
    }
  }

  async function handleDownload() {
    if (!bundle) return;
    const popup = window.open("", "_blank");
    if (popup) popup.opener = null;
    try {
      const res = await download.mutateAsync(bundle.id);
      if (popup) {
        popup.location.href = res.url;
      } else {
        window.location.assign(res.url);
      }
    } catch (err) {
      popup?.close();
      toast.error(err instanceof Error ? err.message : "Gagal mengambil tautan unduhan.");
    }
  }

  return (
    <section className="rounded-2xl border border-amber-200 bg-amber-50/50 p-4" data-testid={`${scope}-question-bundle-controls`}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h3 className="flex items-center gap-2 text-sm font-semibold text-ink-900">
            <FileText className="size-4 text-amber-700" />
            Unduh naskah soal PDF
          </h3>
          <p className="mt-1 text-sm text-ink-500">
            Buat PDF A4 async untuk ujian offline/cadangan. Naskah dan kunci dibuat sebagai file terpisah.
          </p>
          <p className="mt-2 text-xs text-ink-500" data-testid="question-bundle-status">
            {helper}
          </p>
        </div>
        <div className="flex flex-wrap gap-2 sm:justify-end">
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="rounded-full bg-white"
            disabled={disabled || isBusy}
            onClick={() => handleGenerate(false)}
          >
            {isBusy && (pendingVariant ?? bundle?.variant) === "naskah" ? <Loader2 className="mr-1 size-4 animate-spin" /> : <FileText className="mr-1 size-4" />}
            Buat naskah
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="rounded-full border-danger/30 bg-white text-danger hover:text-danger"
            disabled={disabled || isBusy}
            onClick={() => handleGenerate(true)}
          >
            {isBusy && (pendingVariant ?? bundle?.variant) === "kunci" ? <Loader2 className="mr-1 size-4 animate-spin" /> : <KeyRound className="mr-1 size-4" />}
            Buat kunci
          </Button>
          <Button
            type="button"
            size="sm"
            className="rounded-full"
            disabled={bundle?.status !== "ready" || download.isPending}
            onClick={handleDownload}
          >
            {download.isPending ? <Loader2 className="mr-1 size-4 animate-spin" /> : <Download className="mr-1 size-4" />}
            Unduh PDF
          </Button>
        </div>
      </div>
    </section>
  );
}
