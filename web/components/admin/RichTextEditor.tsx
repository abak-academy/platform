"use client";

import { useEffect, useRef, useState, type ChangeEvent } from "react";
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

function isEffectivelyEmpty(html: string): boolean {
  const tmp = document.createElement("div");
  tmp.innerHTML = html;
  if (tmp.textContent && tmp.textContent.trim()) return false;
  if (tmp.querySelector("img")) return false;
  return true;
}

function sanitizeClipboardHtml(html: string): string {
  // For pasted content, only allow src/alt on img, no style attributes
  const ALLOWED_ATTR = ["src", "alt"];
  return DOMPurify.sanitize(html, { ALLOWED_TAGS: QUESTION_BODY_ALLOWED_TAGS, ALLOWED_ATTR });
}

export function RichTextEditor({ value, onChange, placeholder, disabled, id, "aria-label": ariaLabel, "aria-labelledby": ariaLabelledby, minHeightClassName = "min-h-[130px]", compact = false }: RichTextEditorProps) {
  const ref = useRef<HTMLDivElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [empty, setEmpty] = useState<boolean>(!value || isEffectivelyEmpty(value));
  const presign = usePresignAdminImageUpload();

  // On mount only, mirror `value` into the contentEditable if it differs.
  useEffect(() => {
    if (ref.current && ref.current.innerHTML !== value) {
      ref.current.innerHTML = value || "";
    }
    // Without this, Chromium's default Enter behaviour wraps new lines in
    // <div>, which isn't allowlisted and gets stripped server-side (FB-24) —
    // <p> is.
    document.execCommand("defaultParagraphSeparator", false, "p");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function sync() {
    if (!ref.current) return;
    const html = ref.current.innerHTML;
    setEmpty(isEffectivelyEmpty(html));
    onChange(html);
  }

  function exec(cmd: string, arg?: string) {
    document.execCommand(cmd, false, arg);
    if (ref.current) ref.current.focus();
    sync();
  }

  function insertFormula() {
    const sel = typeof window !== "undefined" ? window.getSelection() : null;
    const chosen = sel ? sel.toString() : "";
    exec("insertText", chosen ? `\\(${chosen}\\)` : "\\(\\ \\)");
  }

  function handlePaste(e: React.ClipboardEvent<HTMLDivElement>) {
    e.preventDefault();
    const html = e.clipboardData?.getData("text/html");

    if (html) {
      const sanitized = sanitizeClipboardHtml(html);
      if (sanitized) {
        exec("insertHTML", sanitized);
      }
    } else {
      // Fall back to plain text if HTML is not available.
      // Use insertText to insert literal text without parsing markup.
      const text = e.clipboardData?.getData("text/plain") || "";
      if (text) {
        exec("insertText", text);
      }
    }
  }

  async function handleFileSelected(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
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
      exec(
        "insertHTML",
        `<img src="${src}" alt="" style="max-width:60%;border-radius:8px;margin:6px 0;" />`,
      );
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
          onClick={() => exec("bold")}
          aria-label="Bold"
          disabled={disabled}
        >
          <Bold className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onClick={() => exec("italic")}
          aria-label="Italic"
          disabled={disabled}
        >
          <Italic className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onClick={() => exec("underline")}
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
          onClick={() => exec("insertUnorderedList")}
          aria-label="Bulleted list"
          disabled={disabled}
        >
          <List className={iconGlyphSize} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size={iconSize}
          onClick={() => exec("insertOrderedList")}
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
          onClick={() => exec("superscript")}
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
          onClick={() => exec("subscript")}
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
      </div>
      <div className="relative">
        <div
          ref={ref}
          id={id}
          aria-label={ariaLabel}
          aria-labelledby={ariaLabelledby}
          role="textbox"
          contentEditable={!disabled}
          suppressContentEditableWarning
          onInput={sync}
          onBlur={sync}
          onPaste={handlePaste}
          className={cn(minHeightClassName, "px-3 py-2 text-sm leading-relaxed outline-none")}
        />
        {empty && placeholder && (
          <div className="pointer-events-none absolute left-3 top-2 text-sm text-muted-foreground">
            {placeholder}
          </div>
        )}
      </div>
    </div>
  );
}
