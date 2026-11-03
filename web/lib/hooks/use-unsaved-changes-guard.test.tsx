import { describe, it, expect, vi, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useUnsavedChangesGuard } from "./use-unsaved-changes-guard";

function addLink(attrs: Record<string, string>) {
  const a = document.createElement("a");
  for (const [k, v] of Object.entries(attrs)) a.setAttribute(k, v);
  a.textContent = "link";
  document.body.appendChild(a);
  return a;
}

function click(el: HTMLElement) {
  const event = new MouseEvent("click", { bubbles: true, cancelable: true, button: 0 });
  el.dispatchEvent(event);
  return event;
}

describe("useUnsavedChangesGuard", () => {
  afterEach(() => {
    document.body.innerHTML = "";
    vi.unstubAllGlobals();
  });

  it("warns on beforeunload only while dirty", () => {
    const { rerender } = renderHook(({ dirty }) => useUnsavedChangesGuard(dirty, "buang?"), {
      initialProps: { dirty: false },
    });

    const clean = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(clean);
    expect(clean.defaultPrevented).toBe(false);

    rerender({ dirty: true });
    const dirty = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(dirty);
    expect(dirty.defaultPrevented).toBe(true);
  });

  it("blocks an in-app link click when the user declines to discard", () => {
    const confirmFn = vi.fn(() => false);
    vi.stubGlobal("confirm", confirmFn);
    renderHook(() => useUnsavedChangesGuard(true, "buang?"));

    const event = click(addLink({ href: "/admin/exam/tests" }));

    expect(confirmFn).toHaveBeenCalledWith("buang?");
    expect(event.defaultPrevented).toBe(true);
  });

  it("lets the link through when the user confirms", () => {
    vi.stubGlobal("confirm", () => true);
    renderHook(() => useUnsavedChangesGuard(true, "buang?"));

    const event = click(addLink({ href: "/admin/exam/tests" }));

    expect(event.defaultPrevented).toBe(false);
  });

  it("does not prompt when clean, or for external and new-tab links", () => {
    const confirmFn = vi.fn(() => false);
    vi.stubGlobal("confirm", confirmFn);
    const { rerender } = renderHook(({ dirty }) => useUnsavedChangesGuard(dirty, "buang?"), {
      initialProps: { dirty: false },
    });

    click(addLink({ href: "/admin/exam/tests" }));
    expect(confirmFn).not.toHaveBeenCalled();

    rerender({ dirty: true });
    click(addLink({ href: "https://example.test/x" }));
    click(addLink({ href: "/admin/exam/tests", target: "_blank" }));
    expect(confirmFn).not.toHaveBeenCalled();
  });
});
