import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { RichTextEditor } from "./RichTextEditor";

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
    expect(editable.innerHTML).toBe("<b>hello</b>");
  });

  it("sets defaultParagraphSeparator to 'p' on mount so Enter produces allowlisted markup (FB-24)", () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation(() => true);
    render(<RichTextEditor value="" onChange={vi.fn()} />);
    expect(execSpy).toHaveBeenCalledWith("defaultParagraphSeparator", false, "p");
    execSpy.mockRestore();
  });

  it("clicking Bold with a selection invokes document.execCommand with 'bold'", () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation(() => true);
    const onChange = vi.fn();
    render(<RichTextEditor value="hello" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    // Simulate a selection over the editor's text content.
    const range = document.createRange();
    range.selectNodeContents(editable);
    const sel = window.getSelection();
    sel?.removeAllRanges();
    sel?.addRange(range);

    fireEvent.click(screen.getByRole("button", { name: /bold/i }));

    expect(execSpy).toHaveBeenCalledWith("bold", false, undefined);
    execSpy.mockRestore();
  });

  it("clicking the formula button with no selection inserts '\\( \\)'", () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation(() => true);
    const onChange = vi.fn();
    render(<RichTextEditor value="" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();
    // Ensure no selection.
    const sel = window.getSelection();
    sel?.removeAllRanges();

    fireEvent.click(screen.getByRole("button", { name: /formula/i }));

    expect(execSpy).toHaveBeenCalledWith("insertText", false, "\\(\\ \\)");
    execSpy.mockRestore();
  });

  it("clicking the formula button with a selection wraps the selection in '\\(...\\)'", () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation(() => true);
    const onChange = vi.fn();
    render(<RichTextEditor value="x" onChange={onChange} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    const range = document.createRange();
    range.selectNodeContents(editable);
    const sel = window.getSelection();
    sel?.removeAllRanges();
    sel?.addRange(range);

    fireEvent.click(screen.getByRole("button", { name: /formula/i }));

    expect(execSpy).toHaveBeenCalledWith("insertText", false, "\\(x\\)");
    execSpy.mockRestore();
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

  it("exec restores a saved Range before running execCommand, surviving an intervening focus change (FR27/29)", () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation(() => true);
    render(<RichTextEditor value="hello world" onChange={vi.fn()} />);
    const editable = screen.getByRole("textbox");
    editable.focus();

    // Select "world".
    const textNode = editable.firstChild as Text;
    const range = document.createRange();
    range.setStart(textNode, 6);
    range.setEnd(textNode, 11);
    const sel = window.getSelection();
    sel?.removeAllRanges();
    sel?.addRange(range);

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
      sel?.removeAllRanges();

      fireEvent.click(screen.getByRole("button", { name: /bold/i }));

      expect(execSpy).toHaveBeenCalledWith("bold", false, undefined);
      // The selection must have been restored onto the editable before exec ran.
      const restored = window.getSelection();
      expect(restored?.toString()).toBe("world");
    } finally {
      document.body.removeChild(outside);
      execSpy.mockRestore();
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

  it("does not parse angle brackets in plain-text paste as markup (prevents XSS)", async () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation((cmd, _ui, arg) => {
      // For insertText, append the literal text to the editor.
      if (cmd === "insertText" && typeof arg === "string") {
        const editable = document.querySelector('[contenteditable="true"]');
        if (editable) {
          // insertText should insert literal text, not parse markup.
          const text = document.createTextNode(arg);
          editable.appendChild(text);
        }
        return true;
      }
      // For insertHTML, set innerHTML (for HTML pastes).
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

    // The inserted text must be literal, not parsed as markup.
    // Verify: should call insertText (not insertHTML) and the text should be literal.
    await waitFor(() => {
      // execCommand should have been called with insertText for plain text
      const insertTextCalls = execSpy.mock.calls.filter(([cmd]) => cmd === "insertText");
      expect(insertTextCalls.length).toBeGreaterThan(0);
      expect(insertTextCalls[insertTextCalls.length - 1][2]).toBe(plainTextWithTags);

      // Verify no actual <div> element was created (text should be escaped/literal)
      const divElements = editable.querySelectorAll("div");
      expect(divElements.length).toBe(0); // The <div> in the plain text should NOT be parsed

      // Verify the literal text is present in the editor
      expect(editable.textContent).toContain(plainTextWithTags);
    });

    execSpy.mockRestore();
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
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation((cmd, _ui, arg) => {
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

    const html = "<p>line one</p><p>line two</p><br>";
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
      expect(lastCall).toContain("<p>line two</p>");
      expect(lastCall).toContain("<br>");
    });

    execSpy.mockRestore();
  });

  it("keeps a pasted table with colspan intact (FR-38)", async () => {
    const execSpy = vi.spyOn(document, "execCommand").mockImplementation((cmd, _ui, arg) => {
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

    const tableHtml = '<table><thead><tr><th>Header</th></tr></thead><tbody><tr><td colspan="2">Cell</td></tr></tbody></table>';
    const pasteEvent = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(pasteEvent, "clipboardData", {
      value: {
        getData: (type: string) => (type === "text/html" ? tableHtml : ""),
      },
    });

    editable.dispatchEvent(pasteEvent);

    await waitFor(() => {
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1]?.[0];
      expect(lastCall).toBeDefined();
      expect(lastCall).toContain("<table>");
      expect(lastCall).toContain('colspan="2"');
    });

    execSpy.mockRestore();
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
