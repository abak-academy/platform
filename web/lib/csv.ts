import { downloadTextFile } from "@/lib/bulk-import-format";

// A leading = + - @ makes a spreadsheet read the cell as a formula, so it is quoted off.
function cell(value: string | number | null | undefined): string {
  let text = value === null || value === undefined ? "" : String(value);
  if (/^[=+\-@\t\r]/.test(text)) text = `'${text}`;
  return /[",\n\r]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
}

export function toCsv(header: string[], rows: (string | number | null | undefined)[][]): string {
  return [header, ...rows].map((r) => r.map(cell).join(",")).join("\n") + "\n";
}

export function downloadCsv(filename: string, header: string[], rows: (string | number | null | undefined)[][]): void {
  downloadTextFile(filename, "﻿" + toCsv(header, rows), "text/csv;charset=utf-8");
}
