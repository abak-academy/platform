import { describe, it, expect } from "vitest";
import { render, waitFor } from "@testing-library/react";
import { RichContent } from "./RichContent";

describe("RichContent", () => {
  it("renders sanitized HTML (bold) in the DOM", async () => {
    const { container } = render(<RichContent html="<b>bold</b>" />);

    const target = container.querySelector("[data-rich-content]");
    expect(target).not.toBeNull();
    expect(target?.querySelector("b")).not.toBeNull();
    expect(target?.textContent).toBe("bold");
  });

  it("renders LaTeX delimiters as KaTeX markup, not literal text", async () => {
    const { container } = render(<RichContent html="Solve \(x^2\) now" />);

    const target = container.querySelector("[data-rich-content]") as HTMLElement;
    // After the effect, KaTeX replaces the \(...\) text with a .katex/.katex-mathml/.katex-html subtree
    await waitFor(() => {
      expect(target.textContent).not.toContain("\\(");
      expect(target.textContent).not.toContain("x^2\\)");
    });
    expect(target.querySelector(".katex")).not.toBeNull();
  });

  it("re-renders formulas when html prop changes (no stale markup)", async () => {
    const { container, rerender } = render(<RichContent html="<span>q1 \(a^2\)</span>" />);

    const target = container.querySelector("[data-rich-content]") as HTMLElement;
    await waitFor(() => {
      expect(target.querySelector(".katex")).not.toBeNull();
    });

    // New content: previous formula's marker should be gone, new one present
    rerender(<RichContent html="<span>q2 \(b^3\)</span>" />);

    await waitFor(() => {
      // Old literal delimiters are not present (we never render them in markup
      // after KaTeX auto-render); the visible text of the new node should
      // contain b^3 (Katex renders the literal exponent in the html version).
      expect(target.textContent).toContain("b");
      // And it should not contain a^2 from the previous question.
      expect(target.textContent).not.toContain("a^2");
    });
    expect(target.querySelector(".katex")).not.toBeNull();
  });

  it("preserves <br> and <p> line breaks (FB-24)", async () => {
    const { container } = render(<RichContent html="<p>line one</p><p>line two</p><br>" />);

    const target = container.querySelector("[data-rich-content]") as HTMLElement;
    expect(target.querySelectorAll("p")).toHaveLength(2);
    expect(target.querySelector("br")).not.toBeNull();
  });

  it("renders a table with colspan intact after DOMPurify (FR-38)", async () => {
    const html = '<table><thead><tr><th>Header</th></tr></thead><tbody><tr><td colspan="2">Cell</td></tr></tbody></table>';
    const { container } = render(<RichContent html={html} />);

    const target = container.querySelector("[data-rich-content]") as HTMLElement;
    expect(target.querySelector("table")).not.toBeNull();
    const td = target.querySelector("td");
    expect(td).not.toBeNull();
    expect(td?.getAttribute("colspan")).toBe("2");
  });

  it("sanitizes a legacy unsanitized body at render time (defense-in-depth for pre-PR rows)", async () => {
    const legacyMaliciousBody = '<img src=x onerror="window.__xss = true">safe text';
    const { container } = render(<RichContent html={legacyMaliciousBody} />);

    const target = container.querySelector("[data-rich-content]") as HTMLElement;
    await waitFor(() => {
      expect(target.querySelector("img")?.getAttribute("onerror")).toBeNull();
    });
    expect((window as unknown as { __xss?: boolean }).__xss).toBeUndefined();
    expect(target.textContent).toBe("safe text");
  });
});
