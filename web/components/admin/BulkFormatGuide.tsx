"use client";

import { useTranslation } from "@/lib/i18n";
import type { BulkFieldSpec } from "@/lib/bulk-import-format";

type Translate = ReturnType<typeof useTranslation>["t"];

interface BulkFormatGuideProps {
  fields: BulkFieldSpec[];
  pitfallKeys: readonly Parameters<Translate>[0][];
}

export function BulkFormatGuide({ fields, pitfallKeys }: BulkFormatGuideProps) {
  const { t } = useTranslation();

  return (
    <details className="rounded-lg border border-line p-3">
      <summary className="cursor-pointer text-sm font-semibold text-ink-900">
        {t("bulk_format_show")}
      </summary>
      <div className="mt-3 overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="text-ink-600">
            <tr>
              <th className="py-1 pr-3 font-semibold">{t("bulk_format_col")}</th>
              <th className="py-1 pr-3 font-semibold">{t("bulk_format_required_col")}</th>
              <th className="py-1 pr-3 font-semibold">{t("bulk_format_rule")}</th>
              <th className="py-1 font-semibold">{t("bulk_format_example")}</th>
            </tr>
          </thead>
          <tbody className="align-top text-ink-800">
            {fields.map((f) => (
              <tr key={f.column} className="border-t border-line">
                <td className="py-2 pr-3 font-mono">{f.column}</td>
                <td className="py-2 pr-3">
                  {f.required ? t("bulk_format_required") : t("bulk_format_optional")}
                </td>
                <td className="py-2 pr-3">{t(f.ruleKey)}</td>
                <td className="py-2 font-mono">{f.example}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <ul className="mt-3 list-disc space-y-1 pl-4 text-xs text-ink-700">
        {pitfallKeys.map((k) => (
          <li key={k}>{t(k)}</li>
        ))}
      </ul>
    </details>
  );
}
