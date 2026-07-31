import { describe, it, expect } from "vitest";
import { Editor } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import { Table } from "@tiptap/extension-table";
import TableRow from "@tiptap/extension-table-row";
import TableHeader from "@tiptap/extension-table-header";
import TableCell from "@tiptap/extension-table-cell";
import { execFileSync } from "node:child_process";
import { writeFileSync, readFileSync, rmSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

// --- Task 16 (FR-41, FR-43): go/no-go gate for TipTap as the Part C editor ---
//
// This proves the ONE thing the library decision rests on: a table with
// merged cells, and inline math text, survive
//   getHTML() -> the REAL Go sanitizeQuestionBody -> re-parse
// unchanged. See decision-editor-library.md and spec.md FR-41/FR-42/FR-43.
//
// The "real Go sanitizer" is invoked through a committed, env-gated bridge test
// in backend/internal/service/ (the only place that can call the unexported
// sanitizeQuestionBody). Nothing here is a JS reimplementation of the
// allowlist/bluemonday policy.

const BACKEND_DIR = join(__dirname, "..", "..", "..", "backend");
const BRIDGE_TEST_NAME = "TestSanitizeQuestionBodyBridge";

// On a cold Go build cache the bridge invocation compiles internal/service from
// scratch, which far exceeds vitest's 5s default — CI's frontend job has no Go cache.
const GO_BRIDGE_TIMEOUT_MS = 120_000;

function sanitizeViaRealGoSanitizer(html: string): string {
  const dir = mkdtempSync(join(tmpdir(), "tiptap-spike-"));
  const inPath = join(dir, "in.html");
  const outPath = join(dir, "out.html");
  writeFileSync(inPath, html, "utf8");
  execFileSync(
    "bash",
    [
      "-lc",
      `if [ -z "$GOROOT" ] || [ ! -x "$GOROOT/bin/go" ]; then export GOROOT="$(ls -d /opt/homebrew/Cellar/go/*/libexec 2>/dev/null | sort -V | tail -n1)"; fi; cd "${BACKEND_DIR}" && go test ./internal/service/... -run '^${BRIDGE_TEST_NAME}$' -v`,
    ],
    { env: { ...process.env, SANITIZE_BRIDGE_IN: inPath, SANITIZE_BRIDGE_OUT: outPath }, stdio: "pipe" },
  );
  const result = readFileSync(outPath, "utf8");
  rmSync(dir, { recursive: true, force: true });
  return result;
}

function createTestEditor(content: string) {
  const element = document.createElement("div");
  document.body.appendChild(element);
  return new Editor({
    element,
    extensions: [
      StarterKit,
      Table.configure({ resizable: false }),
      TableRow,
      TableHeader,
      TableCell,
    ],
    content,
  });
}

function cellPositions(editor: Editor, typeName: string): number[] {
  const positions: number[] = [];
  editor.state.doc.descendants((node, pos) => {
    if (node.type.name === typeName) positions.push(pos);
    return true;
  });
  return positions;
}

describe("TipTap spike — Part C round-trip gate (Task 16)", () => {
  it("insert -> addRow -> addColumn -> mergeCells survives getHTML -> real Go sanitizer -> re-parse", () => {
    const editor = createTestEditor("<p></p>");

    editor.commands.insertTable({ rows: 2, cols: 2, withHeaderRow: true });
    editor.commands.addRowAfter();
    editor.commands.addColumnAfter();

    const bodyCells = cellPositions(editor, "tableCell");
    expect(bodyCells.length).toBeGreaterThanOrEqual(2);
    const [anchorCell, headCell] = bodyCells;
    editor.commands.setCellSelection({ anchorCell, headCell });
    const merged = editor.commands.mergeCells();
    expect(merged).toBe(true);

    const html = editor.getHTML();
    expect(html).toMatch(/colspan="2"|rowspan="2"/);

    const sanitized = sanitizeViaRealGoSanitizer(html);

    // The merge must survive the real Go sanitizer intact.
    expect(sanitized).toMatch(/colspan="2"|rowspan="2"/);

    const reparsed = new DOMParser().parseFromString(sanitized, "text/html");
    const table = reparsed.querySelector("table");
    expect(table).not.toBeNull();
    expect(table!.querySelector("tbody > tr > td, tbody > tr > th")).not.toBeNull();

    const originalRowCount = new DOMParser()
      .parseFromString(html, "text/html")
      .querySelectorAll("tr").length;
    const sanitizedRowCount = reparsed.querySelectorAll("tr").length;
    expect(sanitizedRowCount).toBe(originalRowCount);

    editor.destroy();
  }, GO_BRIDGE_TIMEOUT_MS);

  it("math delimiters round-trip byte-identical through getHTML -> real Go sanitizer -> re-parse", () => {
    const mathBody = "<p>Solve \\(x^2\\) and \\(\\frac{1}{2}\\)</p>";
    const editor = createTestEditor(mathBody);

    const html = editor.getHTML();
    expect(html).toContain("\\(x^2\\)");
    expect(html).toContain("\\(\\frac{1}{2}\\)");

    const sanitized = sanitizeViaRealGoSanitizer(html);

    // Byte-identical delimiters and backslashes — not merely "looks right"
    // when rendered. An editor that escapes a backslash still looks correct
    // in the DOM while silently corrupting every stored equation (FB-24
    // failure shape) — assert the stored string itself.
    expect(sanitized).toContain("\\(x^2\\)");
    expect(sanitized).toContain("\\(\\frac{1}{2}\\)");

    editor.destroy();
  }, GO_BRIDGE_TIMEOUT_MS);
});
