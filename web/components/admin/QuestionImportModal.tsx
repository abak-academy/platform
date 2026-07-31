"use client";

import { useEffect, useState } from "react";
import { Download } from "lucide-react";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  useImportBankQuestions,
  useDownloadQuestionImportTemplate,
} from "@/lib/hooks/admin-bank-questions";
import { useTranslation } from "@/lib/i18n";
import type { AdminQuestionImportResponse } from "@/lib/types";

interface QuestionImportModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}

export function QuestionImportModal({ open, onOpenChange, onSuccess }: QuestionImportModalProps) {
  const { t } = useTranslation();
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<AdminQuestionImportResponse | null>(null);
  const [inputKey, setInputKey] = useState(0);

  const importMutation = useImportBankQuestions();
  const downloadMutation = useDownloadQuestionImportTemplate();

  useEffect(() => {
    setFile(null);
    setResult(null);
    setInputKey((k) => k + 1);
  }, [open]);

  async function handleDownloadTemplate() {
    try {
      const blob = await downloadMutation.mutateAsync();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "question_import_template.csv";
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("error_generic"));
    }
  }

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    setFile(e.target.files?.[0] ?? null);
    setResult(null);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) {
      toast.error(t("import_no_file"));
      return;
    }
    try {
      const data = await importMutation.mutateAsync(file);
      setResult(data);
      toast.success(t("import_success").replace("{n}", String(data.inserted)));
      onSuccess();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("error_generic"));
    }
  }

  const errorRows = result?.rows.filter((row) => row.status === "error") ?? [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("import_questions_title")}</DialogTitle>
        </DialogHeader>

        <Button
          type="button"
          variant="outline"
          size="sm"
          className="w-fit"
          onClick={handleDownloadTemplate}
          disabled={downloadMutation.isPending}
        >
          <Download className="mr-2 size-4" />
          {t("questions_download_template")}
        </Button>

        <form onSubmit={handleSubmit} className="space-y-4 py-2">
          <div className="grid gap-2">
            <Label htmlFor="question-import-file">{t("import_choose_file")}</Label>
            <Input
              key={inputKey}
              id="question-import-file"
              type="file"
              accept=".csv,text/csv"
              onChange={handleFileChange}
              disabled={importMutation.isPending}
            />
            {file && <p className="text-sm text-muted-foreground">{file.name}</p>}
          </div>

          <Button type="submit" disabled={importMutation.isPending || !file}>
            {importMutation.isPending ? t("saving") : t("import_submit")}
          </Button>
        </form>

        {result && (
          <div className="space-y-3 rounded-lg border p-3">
            <p className="text-sm font-medium">
              {t("import_success").replace("{n}", String(result.inserted))}
            </p>

            {errorRows.length > 0 && (
              <div className="space-y-1">
                <p className="text-sm font-medium text-destructive">{t("import_errors_title")}</p>
                <ul className="max-h-[200px] overflow-y-auto space-y-1">
                  {errorRows.map((row) => (
                    <li
                      key={row.row_number}
                      className="text-sm text-destructive"
                    >
                      {t("import_row_error")
                        .replace("{row}", String(row.row_number))
                        .replace("{error}", row.error ?? "")}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t("cancel")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
