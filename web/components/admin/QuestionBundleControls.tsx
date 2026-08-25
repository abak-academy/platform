"use client";

import { Download, FileText, KeyRound, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  useQuestionBundleDownload,
  useQuestionBundleState,
  useRequestQuestionBundle,
} from "@/lib/hooks/question-bundles";
import { useHasCapability } from "@/lib/hooks/use-capability";
import type { QuestionBundleState as BundleState, QuestionBundleVariant } from "@/lib/types";

interface QuestionBundleControlsProps {
  testId: string;
  disabled?: boolean;
}

function statusText(state: BundleState | undefined, variant: QuestionBundleVariant): string {
  if (!state || state.status === "idle") return "Belum dibuat.";
  if (state.status === "queued") return "PDF masuk antrean worker.";
  return variant === "kunci" ? "PDF kunci siap. Jangan dibagikan ke peserta." : "PDF naskah siap diunduh.";
}

function VariantControl({ testId, variant, disabled }: QuestionBundleControlsProps & { variant: QuestionBundleVariant }) {
  const state = useQuestionBundleState(testId, variant, true);
  const request = useRequestQuestionBundle(testId, variant);
  const download = useQuestionBundleDownload(testId, variant);
  const isKey = variant === "kunci";
  const ready = state.data?.status === "ready";
  const busy = request.isPending || state.data?.status === "queued";

  async function handleRequest() {
    try {
      const next = await request.mutateAsync();
      toast.success(next.status === "ready" ? "PDF siap diunduh." : "Pembuatan PDF masuk antrean.");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Gagal membuat PDF.");
    }
  }

  async function handleDownload() {
    const popup = window.open("", "_blank");
    if (popup) popup.opener = null;
    try {
      const response = await download.mutateAsync();
      if (popup) popup.location.href = response.url;
      else window.location.assign(response.url);
    } catch (error) {
      popup?.close();
      toast.error(error instanceof Error ? error.message : "Gagal mengambil tautan unduhan.");
    }
  }

  return (
    <div className="flex flex-col gap-2 rounded-xl border border-amber-200 bg-white p-3 sm:flex-row sm:items-center sm:justify-between" data-testid={`question-bundle-${variant}`}>
      <div>
        <p className={`flex items-center gap-2 text-sm font-semibold ${isKey ? "text-danger" : "text-ink-900"}`}>
          {isKey ? <KeyRound className="size-4" /> : <FileText className="size-4" />}
          {isKey ? "Kunci jawaban" : "Naskah soal"}
        </p>
        <p className="mt-1 text-xs text-ink-500" data-testid={`question-bundle-${variant}-status`}>
          {statusText(state.data, variant)}
        </p>
      </div>
      <div className="flex gap-2">
        <Button type="button" size="sm" variant="outline" className="rounded-full" disabled={disabled || busy || ready} onClick={handleRequest}>
          {busy ? <Loader2 className="mr-1 size-4 animate-spin" /> : isKey ? <KeyRound className="mr-1 size-4" /> : <FileText className="mr-1 size-4" />}
          {ready ? "Siap" : "Buat PDF"}
        </Button>
        <Button type="button" size="sm" className="rounded-full" disabled={!ready || download.isPending} onClick={handleDownload}>
          {download.isPending ? <Loader2 className="mr-1 size-4 animate-spin" /> : <Download className="mr-1 size-4" />}
          Unduh
        </Button>
      </div>
    </div>
  );
}

export function QuestionBundleControls({ testId, disabled }: QuestionBundleControlsProps) {
  const canGenerate = useHasCapability("question-bundles:write");
  if (!canGenerate) return null;

  return (
    <section className="rounded-2xl border border-amber-200 bg-amber-50/50 p-4" data-testid="test-question-bundle-controls">
      <div>
        <h3 className="text-sm font-semibold text-ink-900">Unduh soal PDF</h3>
        <p className="mt-1 text-sm text-ink-500">Naskah dan kunci disimpan terpisah dan diperbarui saat konten soal berubah.</p>
      </div>
      <div className="mt-3 grid gap-2">
        <VariantControl testId={testId} variant="naskah" disabled={disabled} />
        <VariantControl testId={testId} variant="kunci" disabled={disabled} />
      </div>
    </section>
  );
}
