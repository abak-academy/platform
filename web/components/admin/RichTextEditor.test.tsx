import { describe, it, expect, vi, beforeEach, afterEach, afterAll } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { Editor } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import { RichTextEditor, InlineMathText, migrateLegacyInlineMath, getStorageHtml } from "./RichTextEditor";
import { RichContent } from "./RichContent";
import { QUESTION_BODY_ALLOWED_TAGS } from "@/lib/question-html";
import { execFileSync } from "node:child_process";
import { writeFileSync, readFileSync, rmSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

type PresignInput = { filename: string; content_type: string };
type PresignOutput = { url: string; method: "PUT"; key: string };
type PresignFn = (input: PresignInput) => Promise<PresignOutput>;

let presignState: {
  mutateAsync: PresignFn;
  isPending: boolean;
} = {
  mutateAsync: vi.fn(),
  isPending: false,
};

vi.mock("@/lib/hooks/admin-uploads", () => ({
  usePresignAdminImageUpload: () => presignState,
}));

beforeEach(() => {
  presignState = {
    mutateAsync: vi.fn().mockResolvedValue({
      url: "https://upload.example.com/put-here",
      method: "PUT",
      key: "questions/uuid/pic.png",
    }),
    isPending: false,
  };
});

describe("RichTextEditor", () => {
  it("renders a contentEditable surface and the toolbar buttons", () => {
    render(<RichTextEditor value="" onChange={vi.fn()} />);
    const editable = screen.getByRole("textbox");
    expect(editable).toBeInTheDocument();
    expect(editable).toHaveAttribute("contenteditable", "true");
  });

  it("initializes contentEditable with the provided value on mount", () => {
    render(<RichTextEditor value="<b>hello</b>" onChange={vi.fn()} />);
    const editable = screen.getByRole("textbox");
    // TipTap's schema requires block-level content at the top level, so bare
    // inline markup gets wrapped in a paragraph on parse — <b>hello</b>
    // becomes <p><b>hello</b></p>. The text and its bold mark are unchanged;
    // only the implicit block wrapper is new (behaviour change, not a loss).
    expect(editable.innerHTML).toBe("<p><b>hello</b></p>");
  });

  // TipTap owns Enter entirely via its own keymap plugin (splitBlock), which
  // ProseMirror always registers regardless of the DOM's native
  // execCommand("defaultParagraphSeparator") setting — there is no more
  // execCommand call in this component at all, so the old line is
  // unreachable. Verified here by evidence (FR-40): press real Enter and
  // assert the resulting markup, rather than assuming.
  it("Enter produces a new <p>, never a <div> (FB-24, FR-40)", async () => {
    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();
    fireEvent.keyDown(editable, { key: "Enter", code: "Enter" });
    await waitFor(() => {
      expect(editable.querySelectorAll("div").length).toBe(0);
      expect(editable.querySelectorAll("p").length).toBeGreaterThanOrEqual(2);
    });
  });

  // jsdom cannot drive a real native Selection/Range the way a browser can
  // (see web/e2e/question-editor.spec.ts's docstring) — Ctrl+A is used here
  // instead of a partial mouse-drag selection because it is a command
  // TipTap's own keymap intercepts directly, so it is reliable in jsdom.
  // Partial-selection precision (FB-22) is covered by the Playwright suite.
  it("clicking Bold with a selection bolds the selected text via the real TipTap command", async () => {
    const onChange = vi.fn();
    render(<RichTextEditor value="hello" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();
    fireEvent.keyDown(editable, { key: "a", code: "KeyA", ctrlKey: true });

    fireEvent.click(screen.getByRole("button", { name: /bold/i }));

    await waitFor(() => {
      const last = onChange.mock.calls[onChange.mock.calls.length - 1][0] as string;
      expect(last).toBe("<p><b>hello</b></p>");
    });
  });

  it("clicking the formula button with no selection inserts '\\( \\)'", async () => {
    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    fireEvent.click(screen.getByRole("button", { name: /formula/i }));

    await waitFor(() => {
      const last = onChange.mock.calls[onChange.mock.calls.length - 1][0] as string;
      expect(last).toContain("\\(\\ \\)");
    });
  });

  it("clicking the formula button with a selection wraps the selection in '\\(...\\)'", async () => {
    const onChange = vi.fn();
    render(<RichTextEditor value="x" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();
    fireEvent.keyDown(editable, { key: "a", code: "KeyA", ctrlKey: true });

    fireEvent.click(screen.getByRole("button", { name: /formula/i }));

    await waitFor(() => {
      const last = onChange.mock.calls[onChange.mock.calls.length - 1][0] as string;
      expect(last).toContain("\\(x\\)");
    });
  });

  it("disables the image button while a presign is in flight and re-enables on resolve", async () => {
    let resolveUpload!: (v: { url: string; method: "PUT"; key: string }) => void;
    presignState.mutateAsync = vi.fn((): Promise<PresignOutput> => {
      presignState.isPending = true;
      return new Promise((resolve) => { resolveUpload = resolve; });
    });
    presignState.isPending = false;

    const fetchSpy = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchSpy);

    const execSpy = vi.spyOn(document, "execCommand").mockImplementation(() => true);
    const onChange = vi.fn();
    const { rerender } = render(<RichTextEditor value="" onChange={onChange} />);

    const imageButton = screen.getByRole("button", { name: /insert image/i });
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
    expect(fileInput).toBeTruthy();

    // Simulate the user picking a file — this kicks off presign.mutateAsync, which
    // flips isPending=true inside our mock.
    const file = new File(["dummy"], "pic.png", { type: "image/png" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    // Force the component to re-read the (mutated) mock state.
    rerender(<RichTextEditor value="" onChange={onChange} />);

    // While presign is pending, the image button must be disabled.
    await waitFor(() => expect(imageButton).toBeDisabled());

    // Resolve presign so the upload chain proceeds; flip isPending to mirror the hook.
    resolveUpload({
      url: "https://upload.example.com/put-here",
      method: "PUT",
      key: "questions/uuid/pic.png",
    });
    presignState.isPending = false;
    rerender(<RichTextEditor value="" onChange={onChange} />);

    await waitFor(() => expect(imageButton).not.toBeDisabled());

    execSpy.mockRestore();
    vi.unstubAllGlobals();
  });

  it("after image upload resolves, calls onChange with HTML containing an <img> tag", async () => {
    presignState.mutateAsync = vi.fn().mockResolvedValue({
      url: "https://upload.example.com/put-here",
      method: "PUT",
      key: "questions/uuid/pic.png",
    });
    presignState.isPending = false;

    const fetchSpy = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchSpy);

    const execSpy = vi.spyOn(document, "execCommand").mockImplementation((cmd, _ui, arg) => {
      // Mirror the editor's append behavior on insertHTML so onChange can pick it up.
      if (cmd === "insertHTML" && typeof arg === "string") {
        const editable = document.querySelector('[contenteditable="true"]');
        if (editable) editable.innerHTML = arg;
        return true;
      }
      return true;
    });

    const onChange = vi.fn();
    const { rerender } = render(<RichTextEditor value="" onChange={onChange} />);

    // Find the hidden file input and simulate file selection.
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
    expect(fileInput).toBeTruthy();
    const file = new File(["dummy"], "pic.png", { type: "image/png" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(onChange).toHaveBeenCalled();
      const last = onChange.mock.calls[onChange.mock.calls.length - 1][0] as string;
      expect(last).toMatch(/<img/i);
    });

    execSpy.mockRestore();
    vi.unstubAllGlobals();
    rerender(<RichTextEditor value="" onChange={onChange} />);
  });

  it("toolbar buttons prevent default on mousedown so focus never leaves the editable (FR27)", () => {
    render(<RichTextEditor value="hello" onChange={vi.fn()} />);
    for (const name of [/bold/i, /italic/i, /underline/i, /bulleted list/i, /numbered list/i, /superscript/i, /subscript/i, /formula/i, /insert image/i]) {
      const button = screen.getByRole("button", { name });
      const event = new MouseEvent("mousedown", { bubbles: true, cancelable: true });
      const prevented = !button.dispatchEvent(event);
      expect(prevented, `mousedown on "${button.getAttribute("aria-label")}" should be prevented`).toBe(true);
    }
  });

  // The old execCommand engine needed to manually clone and restore a DOM
  // Range because a native contentEditable's browser Selection is lost on
  // blur. TipTap keeps selection in its own EditorState, which a DOM blur
  // does not touch — so there is no Range to clone/restore any more, and
  // this test now proves the surviving guarantee (selection makes it through
  // an intervening focus change) at the observable-output level instead of
  // asserting on the removed internal mechanism.
  it("Bold survives an intervening focus change between mousedown and click (FR27/29)", async () => {
    const onChange = vi.fn();
    render(<RichTextEditor value="hello world" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();
    fireEvent.keyDown(editable, { key: "a", code: "KeyA", ctrlKey: true });

    // Simulate the toolbar's mousedown firing first (preventDefault keeps
    // focus/selection intact) — this is what the component wires up.
    fireEvent.mouseDown(screen.getByRole("button", { name: /bold/i }), {});

    // Now simulate focus moving away entirely (e.g. an async gap, like the
    // OS file dialog) before the action actually runs. Use a <button>, not
    // an <input>, so it carries no implicit "textbox" role that could leak
    // into other tests' getByRole("textbox") queries if cleanup is skipped.
    const outside = document.createElement("button");
    document.body.appendChild(outside);
    try {
      outside.focus();

      fireEvent.click(screen.getByRole("button", { name: /bold/i }));

      await waitFor(() => {
        const last = onChange.mock.calls[onChange.mock.calls.length - 1][0] as string;
        expect(last).toBe("<p><b>hello world</b></p>");
      });
    } finally {
      document.body.removeChild(outside);
    }
  });

  it("never uses window.prompt for image insertion", () => {
    const promptSpy = vi.spyOn(window, "prompt").mockImplementation(() => null);
    render(<RichTextEditor value="" onChange={vi.fn()} />);
    // No prompt call from render. The image button's onClick is wired to a file input,
    // not a prompt. Clicking the button should also not call prompt.
    fireEvent.click(screen.getByRole("button", { name: /image/i }));
    expect(promptSpy).not.toHaveBeenCalled();
    promptSpy.mockRestore();
  });

  it("sanitizes Word-style HTML on paste by removing style attributes and unwrapping span", async () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation((cmd, _ui, arg) => {
      // Mirror the insertHTML behavior so onChange can pick it up.
      if (cmd === "insertHTML" && typeof arg === "string") {
        const editable = document.querySelector('[contenteditable="true"]');
        if (editable) editable.innerHTML = arg;
        return true;
      }
      return true;
    });

    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    // Simulate paste with Word-style HTML containing style attributes.
    const wordHtml = '<span style="mso-line-height-rule:exactly;line-height:9999%">text</span>';
    const pasteEvent = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(pasteEvent, "clipboardData", {
      value: {
        getData: (type: string) => (type === "text/html" ? wordHtml : ""),
      },
    });

    editable.dispatchEvent(pasteEvent);

    // The result should have no style attribute and no wrapping span (text rendered directly).
    await waitFor(() => {
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1]?.[0];
      expect(lastCall).toBeDefined();
      expect(lastCall).not.toContain('style=');
      expect(lastCall).not.toContain('<span>text</span>');
      // The plain text "text" should be present.
      expect(lastCall).toContain('text');
    });

    execSpy.mockRestore();
  });

  it("preserves plain text paste when text/html is not available", async () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation((cmd, _ui, arg) => {
      // Mirror the insertText and insertHTML behavior so onChange can pick it up.
      if (cmd === "insertText" && typeof arg === "string") {
        const editable = document.querySelector('[contenteditable="true"]');
        if (editable) {
          const text = document.createTextNode(arg);
          editable.appendChild(text);
        }
        return true;
      }
      if (cmd === "insertHTML" && typeof arg === "string") {
        const editable = document.querySelector('[contenteditable="true"]');
        if (editable) editable.innerHTML = arg;
        return true;
      }
      return true;
    });

    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    // Simulate paste with only plain text (no text/html).
    const pasteEvent = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(pasteEvent, "clipboardData", {
      value: {
        getData: (type: string) => (type === "text/plain" ? "plain text content" : ""),
      },
    });

    editable.dispatchEvent(pasteEvent);

    // Plain text should be inserted.
    await waitFor(() => {
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1]?.[0];
      expect(lastCall).toBeDefined();
      expect(lastCall).toContain('plain text content');
    });

    execSpy.mockRestore();
  });

  // No execCommand mock needed any more: a text/plain-only paste with no
  // text/html data is never routed through our code at all — ProseMirror's
  // own default paste handling inserts it as a literal text node before our
  // component sees it (see RichTextEditor.tsx's transformPastedHTML
  // comment), so angle brackets can't become markup without any code here.
  it("does not parse angle brackets in plain-text paste as markup (prevents XSS)", async () => {
    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    // Simulate paste with plain text containing angle brackets (no text/html).
    // This mimics someone pasting a code snippet like "if (x<5) { <div>test</div> }"
    const plainTextWithTags = 'if (x<5) { <div>test</div> }';
    const pasteEvent = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(pasteEvent, "clipboardData", {
      value: {
        getData: (type: string) => (type === "text/plain" ? plainTextWithTags : ""),
      },
    });

    editable.dispatchEvent(pasteEvent);

    await waitFor(() => {
      // Verify no actual <div> element was created (text should be escaped/literal)
      const divElements = editable.querySelectorAll("div");
      expect(divElements.length).toBe(0); // The <div> in the plain text should NOT be parsed

      // Verify the literal text is present in the editor
      expect(editable.textContent).toContain(plainTextWithTags);
    });
  });

  it("preserves clean HTML with allowed tags on paste", async () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation((cmd, _ui, arg) => {
      // Mirror the insertHTML behavior so onChange can pick it up.
      if (cmd === "insertHTML" && typeof arg === "string") {
        const editable = document.querySelector('[contenteditable="true"]');
        if (editable) editable.innerHTML = arg;
        return true;
      }
      return true;
    });

    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    // Simulate paste with clean HTML containing only allowed tags.
    const cleanHtml = '<b>bold</b> and <i>italic</i>';
    const pasteEvent = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(pasteEvent, "clipboardData", {
      value: {
        getData: (type: string) => (type === "text/html" ? cleanHtml : ""),
      },
    });

    editable.dispatchEvent(pasteEvent);

    // Clean HTML should be preserved.
    await waitFor(() => {
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1]?.[0];
      expect(lastCall).toBeDefined();
      expect(lastCall).toContain('<b>bold</b>');
      expect(lastCall).toContain('<i>italic</i>');
    });

    execSpy.mockRestore();
  });

  it("preserves <br> and <p> line breaks on paste (FB-24)", async () => {
    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    // A <br> must sit inside a block to be schema-valid (TipTap's doc
    // requires block-level content at the top level) — a bare/trailing <br>
    // with nothing after it collapses into the same "trailing break" marker
    // an empty paragraph gets for free, and is not serialized by getHTML().
    // That is a real, if narrow, behaviour change from the old execCommand
    // engine (which inserted any given HTML unconditionally, schema or
    // not); a <br> that actually separates two runs of text — the
    // realistic FB-24 shape — still survives, which is what this asserts.
    const html = "<p>line one</p><p>line two<br>line three</p>";
    const pasteEvent = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(pasteEvent, "clipboardData", {
      value: {
        getData: (type: string) => (type === "text/html" ? html : ""),
      },
    });

    editable.dispatchEvent(pasteEvent);

    await waitFor(() => {
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1]?.[0];
      expect(lastCall).toBeDefined();
      expect(lastCall).toContain("<p>line one</p>");
      expect(lastCall).toContain("line two<br>line three");
    });
  });

  // Task 18 wired Table/TableRow/TableHeader/TableCell into the schema, so a
  // pasted <table> now has somewhere to go instead of collapsing to text.
  it("keeps a pasted table with colspan intact (FR-38) — restore in Task 18", async () => {
    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    const tableHtml = '<table><thead><tr><th>Header</th></tr></thead><tbody><tr><td colspan="2">Cell</td></tr></tbody></table>';
    const pasteEvent = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(pasteEvent, "clipboardData", {
      value: {
        getData: (type: string) => (type === "text/html" ? tableHtml : ""),
      },
    });

    editable.dispatchEvent(pasteEvent);

    // Assert against the parsed DOM rather than a literal HTML substring —
    // the Table extension's renderHTML always emits a colgroup + inline
    // sizing style on <table> (prosemirror-tables' own rendering, unrelated
    // to paste sanitisation), so the serialized string is no longer the bare
    // "<table>" this assertion predates. The structural claim FR-38 actually
    // cares about — the table survived, colspan intact — holds regardless.
    await waitFor(() => {
      expect(editable.querySelector("table")).not.toBeNull();
      expect(editable.querySelector('[colspan="2"]')).not.toBeNull();
    });
  });

  it("removes disallowed tags (e.g., script) on paste", async () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation((cmd, _ui, arg) => {
      // Mirror the insertHTML behavior so onChange can pick it up.
      if (cmd === "insertHTML" && typeof arg === "string") {
        const editable = document.querySelector('[contenteditable="true"]');
        if (editable) editable.innerHTML = arg;
        return true;
      }
      return true;
    });

    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    // Simulate paste with dangerous content.
    const dangerousHtml = '<b>safe</b><script>alert("xss")</script>';
    const pasteEvent = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(pasteEvent, "clipboardData", {
      value: {
        getData: (type: string) => (type === "text/html" ? dangerousHtml : ""),
      },
    });

    editable.dispatchEvent(pasteEvent);

    // Script tag should be removed, but bold should remain.
    await waitFor(() => {
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1]?.[0];
      expect(lastCall).toBeDefined();
      expect(lastCall).toContain('<b>safe</b>');
      expect(lastCall).not.toContain('script');
    });

    execSpy.mockRestore();
  });
});

describe("RichTextEditor tables (Task 18)", () => {
  it("Insert table adds a 3x3 table with a header row", async () => {
    render(<RichTextEditor value="" onChange={vi.fn()} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    fireEvent.click(screen.getByRole("button", { name: /insert table/i }));

    await waitFor(() => {
      expect(editable.querySelectorAll("tr").length).toBe(3);
      expect(editable.querySelectorAll("tr")[0].children.length).toBe(3);
      expect(editable.querySelectorAll("th").length).toBe(3);
    });
  });

  it("Add row and Add column grow the table from the toolbar", async () => {
    render(<RichTextEditor value="" onChange={vi.fn()} />);
    const editable = screen.getByRole("textbox");
    editable.focus();
    fireEvent.click(screen.getByRole("button", { name: /insert table/i }));
    await waitFor(() => expect(editable.querySelector("table")).not.toBeNull());

    fireEvent.click(screen.getByRole("button", { name: /^add row$/i }));
    expect(editable.querySelectorAll("tr").length).toBe(4);

    fireEvent.click(screen.getByRole("button", { name: /^add column$/i }));
    expect(editable.querySelectorAll("tr")[0].children.length).toBe(4);
  });

  it("Delete row and Delete column shrink the table, leaving the header row intact", async () => {
    render(<RichTextEditor value="" onChange={vi.fn()} />);
    const editable = screen.getByRole("textbox");
    editable.focus();
    fireEvent.click(screen.getByRole("button", { name: /insert table/i }));
    await waitFor(() => expect(editable.querySelector("table")).not.toBeNull());

    // Tab (goToNextCell) from the first header cell four times lands the
    // caret in the second row's first body cell — jsdom has no
    // posAtCoords, so a mouse-driven click into a specific cell (real
    // browser navigation) can't be simulated; Tab is a real keymap binding
    // registered by the Table extension (goToNextCell), not a test-only path.
    for (let i = 0; i < 4; i++) {
      fireEvent.keyDown(editable, { key: "Tab", code: "Tab" });
    }

    fireEvent.click(screen.getByRole("button", { name: /^delete row$/i }));
    expect(editable.querySelectorAll("tr").length).toBe(2);
    expect(editable.querySelectorAll("th").length).toBe(3);

    fireEvent.click(screen.getByRole("button", { name: /^delete column$/i }));
    expect(editable.querySelectorAll("tr")[0].children.length).toBe(2);
  });

  it("Merge cells and Split cell toggle colspan from the toolbar", async () => {
    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();
    fireEvent.click(screen.getByRole("button", { name: /insert table/i }));
    await waitFor(() => expect(editable.querySelector("table")).not.toBeNull());

    // Extend the cell selection rightward — the Shift-ArrowRight cell
    // selection extension registered by prosemirror-tables' tableEditing
    // plugin, not a test-only shortcut.
    fireEvent.keyDown(editable, { key: "ArrowRight", code: "ArrowRight", shiftKey: true });
    fireEvent.click(screen.getByRole("button", { name: /merge cells/i }));

    await waitFor(() => {
      expect(editable.querySelector('[colspan="2"]')).not.toBeNull();
    });

    fireEvent.click(screen.getByRole("button", { name: /split cell/i }));

    await waitFor(() => {
      expect(editable.querySelector('[colspan="2"]')).toBeNull();
    });
  });
});

// --- FR-42: the epic's real acceptance is save+reload survival, not a
// post-insert DOM check. This bridges the actual production RichTextEditor
// component (toolbar-driven insert + merge) through the REAL Go
// sanitizeQuestionBody and back through the actual RichContent component —
// mirroring Task 16's spike (tiptap-spike.test.tsx), whose bridge-file
// helpers are duplicated here rather than imported: that file is a Task 16
// artifact explicitly marked "do not modify", and the bridge is a few dozen
// lines of throwaway plumbing, not a shared abstraction worth extracting for
// two call sites.
describe("RichTextEditor + RichContent — table save+reload proof (FR-42)", () => {
  const BACKEND_DIR = join(__dirname, "..", "..", "..", "backend");
  const BRIDGE_TEST_NAME = "TestZZZTask18SanitizeBridge";
  const BRIDGE_FILE = join(BACKEND_DIR, "internal/service/zzz_task18_sanitize_bridge_test.go");

  function writeBridgeFile() {
    writeFileSync(
      BRIDGE_FILE,
      `package service

import (
	"os"
	"testing"
)

// ${BRIDGE_TEST_NAME} is a throwaway bridge used only by Task 18's
// RichTextEditor+RichContent round-trip test to run real HTML through the
// production sanitizeQuestionBody from a vitest test. Written and deleted by
// the test itself on every invocation — never committed.
func ${BRIDGE_TEST_NAME}(t *testing.T) {
	in := os.Getenv("SPIKE_SANITIZE_IN")
	out := os.Getenv("SPIKE_SANITIZE_OUT")
	if in == "" || out == "" {
		t.Skip("bridge env vars not set")
	}
	data, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte(sanitizeQuestionBody(string(data))), 0o644); err != nil {
		t.Fatal(err)
	}
}
`,
    );
  }

  function sanitizeViaRealGoSanitizer(html: string): string {
    const dir = mkdtempSync(join(tmpdir(), "task18-roundtrip-"));
    const inPath = join(dir, "in.html");
    const outPath = join(dir, "out.html");
    writeFileSync(inPath, html, "utf8");
    writeBridgeFile();
    try {
      execFileSync(
        "bash",
        [
          "-lc",
          `if [ -z "$GOROOT" ] || [ ! -x "$GOROOT/bin/go" ]; then export GOROOT="$(ls -d /opt/homebrew/Cellar/go/*/libexec 2>/dev/null | sort -V | tail -n1)"; fi; cd "${BACKEND_DIR}" && go test ./internal/service/... -run '^${BRIDGE_TEST_NAME}$' -v`,
        ],
        { env: { ...process.env, SPIKE_SANITIZE_IN: inPath, SPIKE_SANITIZE_OUT: outPath }, stdio: "pipe" },
      );
    } finally {
      rmSync(BRIDGE_FILE, { force: true });
    }
    const result = readFileSync(outPath, "utf8");
    rmSync(dir, { recursive: true, force: true });
    return result;
  }

  afterAll(() => {
    rmSync(BRIDGE_FILE, { force: true });
  });

  it(
    "a table merged via the toolbar survives getHTML -> real Go sanitizeQuestionBody -> RichContent render, colspan intact",
    async () => {
      const onChange = vi.fn();
      render(<RichTextEditor value="" onChange={onChange} />);
      const editable = screen.getByRole("textbox");
      editable.focus();

      fireEvent.click(screen.getByRole("button", { name: /insert table/i }));
      await waitFor(() => expect(editable.querySelector("table")).not.toBeNull());

      fireEvent.keyDown(editable, { key: "ArrowRight", code: "ArrowRight", shiftKey: true });
      fireEvent.click(screen.getByRole("button", { name: /merge cells/i }));
      await waitFor(() => expect(editable.querySelector('[colspan="2"]')).not.toBeNull());

      // This is the "save": the exact string RichTextEditor hands its
      // caller via onChange, which QuestionEditor persists to the backend.
      const savedHtml = onChange.mock.calls[onChange.mock.calls.length - 1][0] as string;
      expect(savedHtml).toContain('colspan="2"');

      // This is the server round-trip: the REAL Go sanitizer, not a JS
      // reimplementation of its allowlist.
      const sanitized = sanitizeViaRealGoSanitizer(savedHtml);
      expect(sanitized).toContain('colspan="2"');
      expect(sanitized).toContain("<table>");
      expect(sanitized).not.toContain("colgroup");
      expect(sanitized).not.toContain("style=");

      // This is the "reload": the actual RichContent component, not a
      // hand-rolled parse.
      const { container } = render(<RichContent html={sanitized} />);
      await waitFor(() => {
        const merged = container.querySelector('[data-rich-content] [colspan="2"]');
        expect(merged).not.toBeNull();
      });
    },
    // Matches tiptap-spike.test.tsx's GO_BRIDGE_TIMEOUT_MS. The go-test bridge
    // recompiles internal/service, and CI's frontend job has no Go build cache:
    // 30s passes warm locally and times out cold on CI.
    120_000,
  );
});

// Task 19 (FR-44/FR-45/FR-46): live in-editor math via
// @tiptap/extension-mathematics's InlineMath node, with its default
// serialisation overridden so storage stays plain "\(latex\)" text.
//
// Exercised against a bare Editor built from the exact InlineMathText export
// RichTextEditor.tsx wires in — not a React-level simulation — because
// jsdom cannot drive the real keystroke-detection path
// (MutationObserver-based DOM diffing) TipTap's input rules rely on to fire
// from an actual keypress. `applyInputRules: true` on `insertContent` is
// TipTap's own supported hook for simulating that same trigger
// (@tiptap/core's inputRulesPlugin reads the `applyInputRules` transaction
// meta it sets), so this exercises the real InputRule handler, not a
// hand-rolled substitute. See tiptap-spike.test.tsx for the precedent of
// testing a TipTap extension against a bare Editor instead of the full
// React component.
describe("RichTextEditor — live math (Task 19)", () => {
  // Bare-Editor instances mount their contentEditable directly onto
  // document.body (mirroring tiptap-spike.test.tsx) rather than into a
  // React tree, so testing-library's automatic cleanup() between tests
  // never touches them — left unremoved, a leftover contenteditable (which
  // carries TipTap's own default role="textbox") makes screen.getByRole in
  // later component-level tests ambiguous. Track and tear down every one.
  let mounted: { editor: Editor; element: HTMLElement }[] = [];

  function createMathTestEditor(content: string) {
    const element = document.createElement("div");
    document.body.appendChild(element);
    const editor = new Editor({
      element,
      extensions: [StarterKit, InlineMathText],
      content,
    });
    mounted.push({ editor, element });
    return editor;
  }

  afterEach(() => {
    for (const { editor, element } of mounted) {
      editor.destroy();
      element.remove();
    }
    mounted = [];
  });

  it("typing \\(x^2\\) converts to a live KaTeX-rendered node in the editing surface, not literal text (FR-44)", async () => {
    const editor = createMathTestEditor("<p></p>");

    editor.chain().insertContent("\\(x^2\\)", { applyInputRules: true }).run();

    await waitFor(() => {
      expect(editor.view.dom.querySelector(".katex")).not.toBeNull();
      expect(editor.view.dom.textContent).not.toContain("\\(x^2\\)");
    });
  });

  it("a math node's storage HTML round-trips byte-identical to plain \\(latex\\) text (FR-44)", () => {
    const editor = createMathTestEditor("<p></p>");
    editor.commands.insertInlineMath({ latex: "x^2" });

    const stored = getStorageHtml(editor);
    expect(stored).toContain("\\(x^2\\)");
    expect(stored).not.toContain("<span");
  });

  // The point of this task's design (spec.md FR-44): if InlineMathText's
  // renderHTML override is ever lost — e.g. a future TipTap upgrade changes
  // internal behaviour the override relied on — this must go red instead of
  // the app quietly writing an unsanitisable format. Deliberately asserts on
  // getStorageHtml's string output, not the editing-surface DOM: an editor
  // that escapes a backslash still looks correct on screen while silently
  // corrupting every stored equation. Because renderHTML and parseHTML live
  // on the same node type, reverting the override would make even
  // legacy-shaped `<span data-type="inline-math">` input serialize back out
  // through the (now default) renderHTML too — there is no separate code
  // path to pin down; this test IS the pinned-down behaviour.
  it("never emits data-type=\"inline-math\", data-type=\"block-math\", or tiptap-mathematics-render in storage (FR-44 anti-quiet-failure guard)", () => {
    const editor = createMathTestEditor("<p></p>");
    editor.commands.insertInlineMath({ latex: "x^2" });

    const stored = getStorageHtml(editor);
    expect(stored).not.toContain('data-type="inline-math"');
    expect(stored).not.toContain('data-type="block-math"');
    expect(stored).not.toContain("tiptap-mathematics-render");
  });

  it("loading a body with legacy \\(latex\\) text renders it as math, not literal text (FR-46)", async () => {
    const editor = createMathTestEditor("<p>Solve \\(x^2\\) and \\(\\frac{1}{2}\\) now</p>");

    migrateLegacyInlineMath(editor);

    await waitFor(() => {
      expect(editor.view.dom.querySelectorAll(".katex").length).toBe(2);
    });
    expect(editor.view.dom.textContent).not.toContain("\\(x^2\\)");
    expect(editor.view.dom.textContent).not.toContain("\\(\\frac{1}{2}\\)");

    // And storage is unchanged by the migration — still plain text.
    const stored = getStorageHtml(editor);
    expect(stored).toContain("\\(x^2\\)");
    expect(stored).toContain("\\(\\frac{1}{2}\\)");
    expect(stored).not.toContain("<span");
  });

  // A closing paren inside the latex body is extremely common in this app's content
  // (\sin(x), f(x), \left(a+b\right)). The delimiter is the 2-char sequence \) — a bare
  // ) must not terminate the match, or these sit in the editor as literal text.
  it.each([
    ["\\(\\sin(x)\\)", "\\sin(x)"],
    ["\\(f(x)\\)", "f(x)"],
    ["\\(\\left(a+b\\right)\\)", "\\left(a+b\\right)"],
  ])("legacy math containing a closing paren still becomes live math: %s (FR-46)", async (stored, _latex) => {
    const editor = createMathTestEditor(`<p>Given ${stored} solve</p>`);

    migrateLegacyInlineMath(editor);

    await waitFor(() => {
      expect(editor.view.dom.querySelectorAll(".katex").length).toBe(1);
    });
    expect(editor.view.dom.textContent).not.toContain(stored);
    // Storage round-trips byte-identically — the paren must survive too.
    expect(getStorageHtml(editor)).toContain(stored);
  });

  it("two adjacent equations each containing parens migrate separately, not as one blob (FR-46)", async () => {
    const editor = createMathTestEditor("<p>\\(f(x)\\) and \\(g(y)\\)</p>");

    migrateLegacyInlineMath(editor);

    await waitFor(() => {
      expect(editor.view.dom.querySelectorAll(".katex").length).toBe(2);
    });
    const stored = getStorageHtml(editor);
    expect(stored).toContain("\\(f(x)\\)");
    expect(stored).toContain("\\(g(y)\\)");
    expect(stored).not.toContain("<span");
  });

  it("the allowlist is not widened to accommodate math (FR-45): span/data-type/data-latex/class stay off QUESTION_BODY_ALLOWED_TAGS", () => {
    expect(QUESTION_BODY_ALLOWED_TAGS).not.toContain("span");
    for (const forbidden of ["class", "data-type", "data-latex"]) {
      expect(QUESTION_BODY_ALLOWED_TAGS).not.toContain(forbidden);
    }
  });

  it("@tiptap/extension-mathematics is pinned to an exact version in package.json (no ^)", () => {
    const pkg = JSON.parse(readFileSync(join(__dirname, "..", "..", "package.json"), "utf8"));
    expect(pkg.dependencies["@tiptap/extension-mathematics"]).toBe("3.29.2");
  });
});

// Component-level proof of FR-46: the actual RichTextEditor, mounted with a
// `value` shaped like a real pre-existing question body, renders its math —
// not just the bare-Editor harness above.
describe("RichTextEditor component — legacy math on load (Task 19, FR-46)", () => {
  it("mounting with a body containing \\(x^2\\) renders KaTeX markup in the editing surface", async () => {
    // Bare JSX attribute string literals don't process backslash escapes the
    // way JS strings do (JSX treats them like HTML text) — "\\(" here would
    // stay two literal backslashes, not one. The {"..."} expression form
    // goes through normal JS string escaping instead.
    render(<RichTextEditor value={"<p>Solve \\(x^2\\) please</p>"} onChange={vi.fn()} />);
    const editable = screen.getByRole("textbox");

    await waitFor(() => {
      expect(editable.querySelector(".katex")).not.toBeNull();
    });
    expect(editable.textContent).not.toContain("\\(x^2\\)");
  });

  it("does not call onChange merely from migrating legacy math on mount", async () => {
    const onChange = vi.fn();
    render(<RichTextEditor value={"<p>Solve \\(x^2\\) please</p>"} onChange={onChange} />);
    const editable = screen.getByRole("textbox");

    await waitFor(() => {
      expect(editable.querySelector(".katex")).not.toBeNull();
    });
    expect(onChange).not.toHaveBeenCalled();
  });
});
