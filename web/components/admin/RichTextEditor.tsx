"use client";

import { forwardRef, useEffect, useImperativeHandle, useRef, useState, type ChangeEvent } from "react";
import { Editor, InputRule, mergeAttributes } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import { Bold as BoldMark } from "@tiptap/extension-bold";
import { Italic as ItalicMark } from "@tiptap/extension-italic";
import Subscript from "@tiptap/extension-subscript";
import Superscript from "@tiptap/extension-superscript";
import ImageExtension from "@tiptap/extension-image";
import { Table as TableExtension } from "@tiptap/extension-table";
import TableRow from "@tiptap/extension-table-row";
import TableHeader from "@tiptap/extension-table-header";
import TableCell from "@tiptap/extension-table-cell";
import { InlineMath } from "@tiptap/extension-mathematics";
import "katex/dist/katex.min.css";
import DOMPurify from "dompurify";
import { toast } from "sonner";
import {
  Bold,
  Italic,
  Underline,
  List,
  ListOrdered,
  Image as ImageIcon,
  ImagePlus,
  Table as TableIcon,
  Rows3,
  Columns3,
  TableCellsMerge,
  TableCellsSplit,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { fileUrl } from "@/lib/api";
import { usePresignAdminImageUpload } from "@/lib/hooks/admin-uploads";
import { QUESTION_BODY_ALLOWED_TAGS } from "@/lib/question-html";

interface RichTextEditorProps {
  value: string;
  onChange: (html: string) => void;
  placeholder?: string;
  disabled?: boolean;
  id?: string;
  "aria-label"?: string;
  "aria-labelledby"?: string;
  minHeightClassName?: string;
  compact?: boolean;
}

// Imperative escape hatch for callers that need to write into the editor
// from outside a toolbar click — e.g. QuestionEditor's insert-blank action
// (FB-25), which must insert `{{N}}` at the caret in lockstep with adding a
// blank row.
export interface RichTextEditorHandle {
  insertTextAtCaret: (text: string) => void;
  setContent: (html: string) => void;
}

// Retag Bold/Italic to <b>/<i> — TipTap's defaults render <strong>/<em>,
// neither of which is in QUESTION_BODY_ALLOWED_TAGS (the Go sanitizer's
// allowlist). Rendering <strong>/<em> would have the server silently strip
// every bold/italic run — a worse regression than FB-24, not a like-for-like
// swap. <b>/<i> parse back into these same marks unchanged (see parseHTML).
const BTag = BoldMark.extend({
  renderHTML({ HTMLAttributes }) {
    return ["b", mergeAttributes(this.options.HTMLAttributes, HTMLAttributes), 0];
  },
});
const ITag = ItalicMark.extend({
  renderHTML({ HTMLAttributes }) {
    return ["i", mergeAttributes(this.options.HTMLAttributes, HTMLAttributes), 0];
  },
});

// The stock Image node only round-trips src/alt/title/width/height — it
// would drop the inline `style` the upload handler sets (FB-23's inserted
// <img> carries sizing/spacing via `style`, not a class).
const ImageTag = ImageExtension.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      style: { default: null },
    };
  },
});

// The plain-text math syntax `question.body` already stores today — mirrors
// RichContent's renderMathInElement delimiters ("\(" ... "\)"). Matches the
// literal 2-char backslash+paren sequences, not a regex grouping construct.
// The body is lazily matched up to the first literal \) — it must NOT exclude a bare
// ")", or common latex (\sin(x), f(x), \left(a+b\right)) never matches and sits in the
// editor as literal text.
const INLINE_MATH_RE = /\\\(([\s\S]+?)\\\)/;
const INLINE_MATH_RE_GLOBAL = /\\\(([\s\S]+?)\\\)/g;

// @tiptap/extension-mathematics's default renderHTML emits
// `<span data-type="inline-math" data-latex="...">` — `span` isn't in
// QUESTION_BODY_ALLOWED_TAGS, so that would be stripped on save (FB-24
// again) and split the DB into two math formats (decision-editor-library.md,
// spec.md FR-44/FR-45). This override instead renders the literal delimited
// text as the span's only child, with no attributes at all — an
// attribute-less <span> is a marker nothing else in this schema produces, so
// getStorageHtml (below) can find and unwrap exactly this wrapper
// deterministically, and only this wrapper: if the override above were ever
// lost, the reverted default's `data-type`/`data-latex` attributes would
// make getStorageHtml leave the span alone, so FR-44's guard test goes red
// instead of the app quietly storing an unsanitisable span.
// Exported for RichTextEditor.test.tsx's Task 19 harness — the extension
// and its serialisation helpers are tested directly against a bare Editor
// (mirroring tiptap-spike.test.tsx), since jsdom cannot drive the real
// keystroke-detection path TipTap's input rules rely on.
export const InlineMathText = InlineMath.extend({
  renderHTML({ node }) {
    return ["span", {}, `\\(${node.attrs.latex}\\)`] as unknown as [string, Record<string, never>, 0];
  },
  // Replaces the base extension's own input rule (which matches "$$latex$$")
  // with one matching the app's actual delimiter, so typing "\(x^2\)" is
  // what triggers live rendering — FR-44.
  addInputRules() {
    return [
      new InputRule({
        find: INLINE_MATH_RE,
        handler: ({ state, range, match }) => {
          const latex = match[1];
          state.tr.replaceWith(range.from, range.to, this.type.create({ latex }));
        },
      }),
    ];
  },
});

// FR-46: every question body already in the database stores math as plain
// "\(latex\)" text. Converts each such run into a live inlineMath node so
// existing questions render their math on open, not just freshly-typed
// formulas.
export function migrateLegacyInlineMath(editor: Editor) {
  const { state } = editor;
  const inlineMathType = state.schema.nodes.inlineMath;
  if (!inlineMathType) return;

  const replacements: { from: number; to: number; latex: string }[] = [];
  state.doc.descendants((node, pos) => {
    if (!node.isText || !node.text) return;
    for (const match of node.text.matchAll(INLINE_MATH_RE_GLOBAL)) {
      const start = pos + (match.index ?? 0);
      replacements.push({ from: start, to: start + match[0].length, latex: match[1] });
    }
  });
  if (replacements.length === 0) return;

  const tr = state.tr;
  for (let i = replacements.length - 1; i >= 0; i -= 1) {
    const { from, to, latex } = replacements[i];
    tr.replaceWith(from, to, inlineMathType.create({ latex }));
  }
  // Not a user edit — don't let it show up in undo history or trigger onChange.
  tr.setMeta("addToHistory", false);
  tr.setMeta("inlineMathMigration", true);
  editor.view.dispatch(tr);
}

// The storage-facing counterpart of InlineMathText's renderHTML override:
// unwraps its bare <span> back into the plain "\(latex\)" text it wraps.
// This, not the node's own renderHTML, is what actually achieves "no
// wrapping element in storage" — ProseMirror's DOMOutputSpec has no way for
// a node to serialize as a naked text node, only elements, so the
// span-then-unwrap round trip is the only way to get byte-identical plain
// text out of getHTML(). See FR-44 and the InlineMathText comment above for
// why a span with attributes is deliberately left untouched here.
export function getStorageHtml(editor: Editor): string {
  const raw = editor.getHTML();
  if (!raw.includes("<span")) return raw;
  const tmp = document.createElement("div");
  tmp.innerHTML = raw;
  tmp.querySelectorAll("span").forEach((span) => {
    if (span.attributes.length > 0) return;
    span.replaceWith(document.createTextNode(span.textContent ?? ""));
  });
  return tmp.innerHTML;
}

function isEffectivelyEmpty(html: string): boolean {
  const tmp = document.createElement("div");
  tmp.innerHTML = html;
  if (tmp.textContent && tmp.textContent.trim()) return false;
  if (tmp.querySelector("img")) return false;
  return true;
}

function sanitizeClipboardHtml(html: string): string {
  // For pasted content, only allow src/alt on img, no style attributes
  const ALLOWED_ATTR = ["src", "alt", "colspan", "rowspan"];
  return DOMPurify.sanitize(html, { ALLOWED_TAGS: QUESTION_BODY_ALLOWED_TAGS, ALLOWED_ATTR });
}

// Replaces the current selection (or inserts at a collapsed caret) with
// literal text — never parsed as markup. This is the TipTap-native
// equivalent of `execCommand("insertText", ...)`: used for the plain-text
// paste fallback (angle brackets must stay literal, not become elements),
// the formula button, and the imperative `insertTextAtCaret` handle that
// QuestionEditor's blank-token insertion rides on.
function insertLiteralText(editor: Editor, text: string) {
  editor
    .chain()
    .focus()
    .command(({ tr, dispatch }) => {
      if (dispatch) dispatch(tr.insertText(text));
      return true;
    })
    .run();
}

export const RichTextEditor = forwardRef<RichTextEditorHandle, RichTextEditorProps>(function RichTextEditor(
  { value, onChange, placeholder, disabled, id, "aria-label": ariaLabel, "aria-labelledby": ariaLabelledby, minHeightClassName = "min-h-[130px]", compact = false },
  forwardedRef
) {
  const mountRef = useRef<HTMLDivElement | null>(null);
  const editorInstanceRef = useRef<Editor | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const savedRangeRef = useRef<{ from: number; to: number } | null>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const [empty, setEmpty] = useState<boolean>(!value || isEffectivelyEmpty(value));
  const presign = usePresignAdminImageUpload();

  useEffect(() => {
    if (!mountRef.current) return;
    const attributes: Record<string, string> = {
      // TipTap only merges its own `role: "textbox"` default in at initial
      // mount (inside createView); editor.setEditable() later calls
      // view.setProps() with this *raw* editorProps object, replacing
      // view.dom's props wholesale and silently dropping that one-time
      // default. Setting it here ourselves survives every setEditable call,
      // not just the first.
      role: "textbox",
      class: cn(minHeightClassName, "px-3 py-2 text-sm leading-relaxed outline-none"),
    };
    if (id) attributes.id = id;
    if (ariaLabel) attributes["aria-label"] = ariaLabel;
    if (ariaLabelledby) attributes["aria-labelledby"] = ariaLabelledby;

    const editor = new Editor({
      element: mountRef.current,
      editable: !disabled,
      content: value || "",
      extensions: [
        StarterKit.configure({
          bold: false,
          italic: false,
          blockquote: false,
          code: false,
          codeBlock: false,
          heading: false,
          horizontalRule: false,
          link: false,
          strike: false,
          dropcursor: false,
          gapcursor: false,
        }),
        BTag,
        ITag,
        Subscript,
        Superscript,
        ImageTag.configure({ inline: true, allowBase64: false }),
        TableExtension.configure({ resizable: false }),
        TableRow,
        TableHeader,
        TableCell,
        InlineMathText,
      ],
      editorProps: {
        attributes,
        // Runs before ProseMirror parses pasted HTML into the doc — the
        // correct hook for sanitizing paste, unlike a React onPaste handler,
        // which would race the view's own native paste listener (already
        // fired and applied its default parse by the time ours runs). Plain
        // text-only paste never reaches this: ProseMirror inserts it as a
        // literal text node by default, so angle brackets already can't
        // become markup without any code here.
        transformPastedHTML: (html: string) => sanitizeClipboardHtml(html),
      },
      onUpdate: ({ editor: ed, transaction }) => {
        // Skip the migration's own transaction (FR-46) — it converts
        // existing text to a math node without the user having changed
        // anything, so it must not report a content change.
        if (transaction.getMeta("inlineMathMigration")) return;
        const html = getStorageHtml(ed);
        setEmpty(isEffectivelyEmpty(html));
        onChangeRef.current(html);
      },
    });
    editorInstanceRef.current = editor;
    migrateLegacyInlineMath(editor);

    return () => {
      editor.destroy();
      editorInstanceRef.current = null;
    };
    // Mount once, mirroring the previous "on mount only" contract for `value`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    // emitUpdate=false: editability is a UI-availability concern, not a
    // content change. setEditable() defaults to emitting a synthetic
    // "update" event even when nothing about the document changed — left
    // at its default, that fires onChange(getHTML()) on every mount
    // (disabled starts undefined, so this effect always runs once) as well
    // as on every later toggle (e.g. savePending flipping disabled during a
    // save), which would misreport "content changed" with identical HTML.
    editorInstanceRef.current?.setEditable(!disabled, false);
  }, [disabled]);

  // Captured on toolbar mousedown, before a native OS dialog (the file
  // chooser) can steal focus mid-upload. TipTap's own selection state
  // survives a DOM blur, but this mirrors the original defensive capture for
  // the async image-insert path.
  function saveSelection() {
    const editor = editorInstanceRef.current;
    if (!editor) return;
    const { from, to } = editor.state.selection;
    savedRangeRef.current = { from, to };
  }

  function preventBlur(e: React.MouseEvent) {
    e.preventDefault();
    saveSelection();
  }

  useImperativeHandle(forwardedRef, () => ({
    insertTextAtCaret: (text: string) => {
      const editor = editorInstanceRef.current;
      if (!editor) return;
      insertLiteralText(editor, text);
    },
    setContent: (html: string) => {
      const ed = editorInstanceRef.current;
      if (!ed) return;
      ed.commands.setContent(html);
      migrateLegacyInlineMath(ed);
    },
  }));

  function insertFormula() {
    const editor = editorInstanceRef.current;
    if (!editor) return;
    const { from, to, empty: noSelection } = editor.state.selection;
    const chosen = noSelection ? "" : editor.state.doc.textBetween(from, to);
    insertLiteralText(editor, chosen ? `\\(${chosen}\\)` : "\\(\\ \\)");
  }

  async function handleFileSelected(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const editor = editorInstanceRef.current;
    try {
      const presigned = await presign.mutateAsync({
        filename: file.name,
        content_type: file.type,
      });
      const uploadRes = await fetch(presigned.url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": file.type },
      });
      if (!uploadRes.ok) {
        throw new Error(`Upload failed: ${uploadRes.status}`);
      }
      const src = fileUrl(presigned.key) ?? presigned.key;
      if (editor) {
        const range = savedRangeRef.current ?? editor.state.selection;
        editor
          .chain()
          .focus()
          .insertContentAt(
            { from: range.from, to: range.to },
            { type: "image", attrs: { src, alt: "", style: "max-width:60%;border-radius:8px;margin:6px 0;" } }
          )
          .run();
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Upload failed");
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  }

  const uploading = presign.isPending;
  const iconSize = compact ? "icon-xs" : "icon-sm";
  const formulaSize = compact ? "xs" : "sm";
  const iconGlyphSize = compact ? "size-3" : "size-4";

  return (
    <div
      className={cn(
        "rounded-md border border-input bg-background text-sm shadow-xs",
        disabled && "pointer-events-none opacity-50",
      )}
    >
      <div className={cn("flex items-center border-b bg-muted/40", compact ? "flex-nowrap gap-0.5 p-1" : "flex-wrap gap-1 p-1.5")}>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().toggleBold().run()}
          aria-label="Bold"
          disabled={disabled}
        >
          <Bold className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().toggleItalic().run()}
          aria-label="Italic"
          disabled={disabled}
        >
          <Italic className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().toggleUnderline().run()}
          aria-label="Underline"
          disabled={disabled}
        >
          <Underline className={iconGlyphSize} />
        </Button>
        <Separator orientation="vertical" className={cn(compact ? "mx-0.5 h-4" : "mx-1 h-5")} />
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().toggleBulletList().run()}
          aria-label="Bulleted list"
          disabled={disabled}
        >
          <List className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().toggleOrderedList().run()}
          aria-label="Numbered list"
          disabled={disabled}
        >
          <ListOrdered className={iconGlyphSize} />
        </Button>
        <Separator orientation="vertical" className={cn(compact ? "mx-0.5 h-4" : "mx-1 h-5")} />
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().toggleSuperscript().run()}
          aria-label="Superscript"
          disabled={disabled}
          className="font-mono text-xs"
        >
          x²
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().toggleSubscript().run()}
          aria-label="Subscript"
          disabled={disabled}
          className="font-mono text-xs"
        >
          x₂
        </Button>
        <Separator orientation="vertical" className={cn(compact ? "mx-0.5 h-4" : "mx-1 h-5")} />
        <Button
          type="button"
          variant="ghost"
          size={formulaSize}
          onMouseDown={preventBlur}
          onClick={insertFormula}
          aria-label="Insert formula"
          disabled={disabled}
        >
          <span className="italic">ƒ</span>
          <span className="text-[11px] font-semibold">x</span>
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => fileInputRef.current?.click()}
          aria-label="Insert image"
          disabled={disabled || uploading}
        >
          {uploading ? <ImageIcon className={cn(iconGlyphSize, "animate-pulse")} /> : <ImagePlus className={iconGlyphSize} />}
        </Button>
        <input
          ref={fileInputRef}
          type="file"
          accept="image/*"
          hidden
          onChange={handleFileSelected}
        />
        <Separator orientation="vertical" className={cn(compact ? "mx-0.5 h-4" : "mx-1 h-5")} />
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() =>
            editorInstanceRef.current
              ?.chain()
              .focus()
              .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
              .run()
          }
          aria-label="Insert table"
          disabled={disabled}
        >
          <TableIcon className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().addRowAfter().run()}
          aria-label="Add row"
          disabled={disabled}
        >
          <Rows3 className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().deleteRow().run()}
          aria-label="Delete row"
          disabled={disabled}
        >
          <Trash2 className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().addColumnAfter().run()}
          aria-label="Add column"
          disabled={disabled}
        >
          <Columns3 className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().deleteColumn().run()}
          aria-label="Delete column"
          disabled={disabled}
        >
          <Trash2 className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().mergeCells().run()}
          aria-label="Merge cells"
          disabled={disabled}
        >
          <TableCellsMerge className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onMouseDown={preventBlur}
          onClick={() => editorInstanceRef.current?.chain().focus().splitCell().run()}
          aria-label="Split cell"
          disabled={disabled}
        >
          <TableCellsSplit className={iconGlyphSize} />
        </Button>
      </div>
      <div
        className={cn(
          "relative",
          "[&_table]:my-2 [&_table]:w-full [&_table]:table-fixed [&_table]:border-collapse",
          "[&_td]:border [&_td]:border-line [&_td]:p-2 [&_td]:align-top",
          "[&_th]:border [&_th]:border-line [&_th]:bg-muted/40 [&_th]:p-2 [&_th]:text-left [&_th]:font-medium",
        )}
        ref={mountRef}
      >
        {empty && placeholder && (
          <div className="pointer-events-none absolute left-3 top-2 text-sm text-muted-foreground">
            {placeholder}
          </div>
        )}
      </div>
    </div>
  );
});
