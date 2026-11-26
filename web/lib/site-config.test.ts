import { describe, expect, it } from "vitest";
import { SITE_CONFIG, waHref } from "./site-config";

describe("site config", () => {
  it("builds a wa.me link with the message url-encoded", () => {
    expect(waHref("Halo & selamat siang?")).toBe(
      "https://wa.me/6282318910482?text=Halo%20%26%20selamat%20siang%3F",
    );
  });

  it("keeps the agreed hero stats", () => {
    expect(SITE_CONFIG.stats.map((s) => s.value)).toEqual(["20.000+", "200+", "10"]);
  });

  it("hides every social link until the client supplies a url", () => {
    expect(SITE_CONFIG.social.every((s) => s.url === null)).toBe(true);
  });
});
