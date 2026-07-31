import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// JSDOM stubs for Radix UI components
Element.prototype.scrollIntoView = vi.fn();
Element.prototype.hasPointerCapture = vi.fn();

// JSDOM does not implement Range layout measurement at all (unlike Element,
// which at least stubs zero rects) — https://github.com/jsdom/jsdom/issues/3729.
// ProseMirror's view calls Range.getClientRects()/getBoundingClientRect() to
// scroll the selection into view after every transaction (e.g. on paste),
// so any TipTap-driven test throws without this stub.
if (typeof Range !== "undefined") {
  Range.prototype.getClientRects ??= () => [] as unknown as DOMRectList;
  Range.prototype.getBoundingClientRect ??= () =>
    ({ top: 0, bottom: 0, left: 0, right: 0, width: 0, height: 0, x: 0, y: 0, toJSON: () => ({}) }) as DOMRect;
}

// JSDOM does not implement the deprecated contentEditable execCommand API.
// Provide a no-op default so tests can spy on it.
if (typeof document !== "undefined" && !document.execCommand) {
  document.execCommand = vi.fn(() => true);
}
