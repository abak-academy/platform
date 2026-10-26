import { describe, it, expect, beforeEach, vi } from "vitest";
import { AFF_REF_COOKIE, AFF_REF_MAX_AGE_SECONDS, setAffRefCookie, readAffRefCookie } from "./aff-ref";

describe("aff_ref cookie", () => {
  beforeEach(() => {
    document.cookie = `${AFF_REF_COOKIE}=; path=/; max-age=0`;
  });

  it("writes path=/ and a fixed 30-day max-age", () => {
    const setter = vi.spyOn(document, "cookie", "set");
    setAffRefCookie("BUDI10");

    expect(setter).toHaveBeenCalledWith(expect.stringContaining(`${AFF_REF_COOKIE}=BUDI10`));
    expect(setter).toHaveBeenCalledWith(expect.stringContaining("path=/"));
    expect(setter).toHaveBeenCalledWith(expect.stringContaining(`max-age=${AFF_REF_MAX_AGE_SECONDS}`));
    setter.mockRestore();
    expect(AFF_REF_MAX_AGE_SECONDS).toBe(30 * 24 * 60 * 60);
  });

  it("uses the admin's cookie lifetime when the server reports one", () => {
    const setter = vi.spyOn(document, "cookie", "set");
    setAffRefCookie("BUDI10", 45);

    expect(setter).toHaveBeenCalledWith(expect.stringContaining(`max-age=${45 * 24 * 60 * 60}`));
    setter.mockRestore();
  });

  it("stores the code in upper case so a lowercase link still matches", () => {
    setAffRefCookie(" budi10 ");

    expect(readAffRefCookie()).toBe("BUDI10");
  });

  it("round-trips a code with special characters", () => {
    setAffRefCookie("ABC+1=");

    expect(readAffRefCookie()).toBe("ABC+1=");
  });

  it("returns an empty string when no cookie is stored", () => {
    expect(readAffRefCookie()).toBe("");
  });
});
