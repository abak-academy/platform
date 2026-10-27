export interface AffiliatorCsvRow {
  name: string;
  email: string;
  password: string;
}

export const AFFILIATOR_CSV_HEADER = "name,email,password";

function splitLine(line: string): string[] {
  const cells: string[] = [];
  let cur = "";
  let quoted = false;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (quoted) {
      if (ch === '"' && line[i + 1] === '"') {
        cur += '"';
        i++;
      } else if (ch === '"') {
        quoted = false;
      } else {
        cur += ch;
      }
    } else if (ch === '"') {
      quoted = true;
    } else if (ch === ",") {
      cells.push(cur);
      cur = "";
    } else {
      cur += ch;
    }
  }
  cells.push(cur);
  return cells.map((c) => c.trim());
}

// Parses the bulk-affiliator CSV (header name,email,password). Returns the
// rows, or an error naming what is wrong so the admin can fix the file.
export function parseAffiliatorCsv(text: string): { rows: AffiliatorCsvRow[]; error?: string } {
  const lines = text.replace(/^﻿/, "").split(/\r?\n/).filter((l) => l.trim() !== "");
  if (lines.length === 0) return { rows: [], error: "empty" };
  const header = splitLine(lines[0]).map((h) => h.toLowerCase());
  const col = (name: string) => header.indexOf(name);
  const [iName, iEmail, iPassword] = [col("name"), col("email"), col("password")];
  if (iName < 0 || iEmail < 0 || iPassword < 0) return { rows: [], error: "header" };

  const rows: AffiliatorCsvRow[] = [];
  for (const line of lines.slice(1)) {
    const cells = splitLine(line);
    rows.push({ name: cells[iName] ?? "", email: cells[iEmail] ?? "", password: cells[iPassword] ?? "" });
  }
  if (rows.length === 0) return { rows: [], error: "empty" };
  return { rows };
}
