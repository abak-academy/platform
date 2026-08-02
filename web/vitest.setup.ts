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

// Node's own experimental global `localStorage` (unconfigured, no
// --localstorage-file) shadows jsdom's real implementation before vitest's
// jsdom environment ever copies it onto globalThis — see the KEYS allowlist
// in vitest's jsdom environment setup, which doesn't list localStorage/
// sessionStorage, so `"localStorage" in global` short-circuits the copy.
// Net effect: `localStorage` is `undefined` in every jsdom test regardless of
// this project's own code. Polyfill a minimal in-memory Storage so any code
// under test that reads/writes localStorage has something real to hit.
if (typeof globalThis.localStorage === "undefined") {
  class MemoryStorage {
    private store = new Map<string, string>();
    getItem(key: string): string | null {
      return this.store.has(key) ? this.store.get(key)! : null;
    }
    setItem(key: string, value: string): void {
      this.store.set(key, String(value));
    }
    removeItem(key: string): void {
      this.store.delete(key);
    }
    clear(): void {
      this.store.clear();
    }
    key(index: number): string | null {
      return [...this.store.keys()][index] ?? null;
    }
    get length(): number {
      return this.store.size;
    }
  }
  Object.defineProperty(globalThis, "localStorage", {
    value: new MemoryStorage(),
    configurable: true,
    writable: true,
  });
}
