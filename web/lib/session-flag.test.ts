import { describe, it, expect, beforeEach } from "vitest";
import { SESSION_FLAG_SCRIPT } from "./session-flag";

const run = () => new Function(SESSION_FLAG_SCRIPT)();

describe("SESSION_FLAG_SCRIPT", () => {
  beforeEach(() => {
    localStorage.clear();
    delete document.documentElement.dataset.session;
  });

  it("sets the flag for a stored token", () => {
    localStorage.setItem("abak-auth", JSON.stringify({ state: { token: "t" } }));
    run();
    expect(document.documentElement.dataset.session).toBe("1");
  });

  it("does not throw on garbage and sets no flag", () => {
    localStorage.setItem("abak-auth", "{not json");
    expect(run).not.toThrow();
    expect(document.documentElement.dataset.session).toBeUndefined();
  });

  it.each([[{ state: { token: null } }], [{ state: { token: "" } }], [{}]])("sets no flag for %j", (v) => {
    localStorage.setItem("abak-auth", JSON.stringify(v));
    run();
    expect(document.documentElement.dataset.session).toBeUndefined();
  });

  it("sets no flag with nothing stored", () => {
    run();
    expect(document.documentElement.dataset.session).toBeUndefined();
  });
});
